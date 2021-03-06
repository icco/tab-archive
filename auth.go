package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	jwtmiddleware "github.com/auth0/go-jwt-middleware"
	"github.com/form3tech-oss/jwt-go"
	"github.com/icco/tab-archive/lib"
	"go.uber.org/zap"
)

// Jwks is from https://auth0.com/docs/quickstart/backend/golang
type Jwks struct {
	Keys []JSONWebKeys `json:"keys"`
}

// JSONWebKeys is from https://auth0.com/docs/quickstart/backend/golang
type JSONWebKeys struct {
	Kty string   `json:"kty"`
	Kid string   `json:"kid"`
	Use string   `json:"use"`
	N   string   `json:"n"`
	E   string   `json:"e"`
	X5c []string `json:"x5c"`
}

// CustomClaims is from https://auth0.com/docs/quickstart/backend/golang
type CustomClaims struct {
	Scope     string   `json:"scope"`
	Audience  []string `json:"aud,omitempty"`
	ExpiresAt int64    `json:"exp,omitempty"`
	ID        string   `json:"jti,omitempty"`
	IssuedAt  int64    `json:"iat,omitempty"`
	Issuer    string   `json:"iss,omitempty"`
	NotBefore int64    `json:"nbf,omitempty"`
	Subject   string   `json:"sub,omitempty"`
}

func (c CustomClaims) toStandard() jwt.StandardClaims {
	return jwt.StandardClaims{
		Audience:  c.Audience,
		ExpiresAt: c.ExpiresAt,
		Id:        c.ID,
		IssuedAt:  c.IssuedAt,
		Issuer:    c.Issuer,
		NotBefore: c.NotBefore,
		Subject:   c.Subject,
	}
}

// Valid is required for conformance to jwt.Claims.
func (c CustomClaims) Valid() error {
	return c.toStandard().Valid()
}

func jsonError(msg string) error {
	data, err := json.Marshal(map[string]string{"error": msg})
	if err != nil {
		log.Errorw("could not marshal json", zap.Error(err))
	}
	return fmt.Errorf("%s", data)
}

// AuthMiddleware parses the incomming authentication header and turns it into
// an attached user.
func AuthMiddleware(db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		jwtMiddleware := jwtmiddleware.New(jwtmiddleware.Options{
			CredentialsOptional: true,
			ValidationKeyGetter: func(token *jwt.Token) (interface{}, error) {
				// Verify 'aud' claim
				aud := "https://natwelch.com"
				checkAud := token.Claims.(jwt.MapClaims).VerifyAudience(aud, false)
				if !checkAud {
					log.Errorw("invalid audence", "aud", token.Raw)
					return token, jsonError("Invalid audience.")
				}
				// Verify 'iss' claim
				iss := "https://icco.auth0.com/"
				checkIss := token.Claims.(jwt.MapClaims).VerifyIssuer(iss, false)
				if !checkIss {
					log.Errorw("invalid issuer", "iss", token.Raw)
					return token, jsonError("Invalid issuer.")
				}

				cert, err := getPemCert(token)
				if err != nil {
					msg := "cloudn't parse pem cert"
					log.Errorw(msg, zap.Error(err))
					return token, jsonError(msg)
				}

				data, err := jwt.ParseRSAPublicKeyFromPEM([]byte(cert))
				if err != nil {
					log.Errorw("error parsing cert", zap.Error(err), "cert", cert)
					return token, jsonError(err.Error())
				}
				return data, nil
			},
			SigningMethod: jwt.SigningMethodRS256,
			ErrorHandler: func(w http.ResponseWriter, r *http.Request, err string) {
				log.Errorw("error with auth", zap.Error(fmt.Errorf(err)))
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.Header().Set("X-Content-Type-Options", "nosniff")
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprintf(w, `{"error": "%s"}`, err)
				return
			},
		})

		return jwtMiddleware.Handler(getUserFromToken(db, next))
	}
}

func getUserFromToken(db *sql.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString, err := jwtmiddleware.FromAuthHeader(r)
		if err != nil {
			log.Errorw("could not get auth header", zap.Error(err))
			next.ServeHTTP(w, r)
			return
		}

		claims := &CustomClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			cert, err := getPemCert(token)
			if err != nil {
				return nil, err
			}
			return jwt.ParseRSAPublicKeyFromPEM([]byte(cert))
		})

		// No claims, that's cool.
		if err != nil {
			// log.Debugw("could not get claims", zap.Error(err))
			next.ServeHTTP(w, r)
			return
		}

		log.Debugw("the token", "token", token, "claims", claims)

		if claims.Subject != "" {
			user, err := lib.GetUser(r.Context(), db, claims.Subject)
			if err != nil {
				log.Errorw("could not get user", "claims", claims, zap.Error(err))
			} else {
				// put it in context
				ctx := lib.WithUser(r.Context(), user)
				r = r.WithContext(ctx)
			}
		}

		next.ServeHTTP(w, r)
	})
}

func getPemCert(token *jwt.Token) (string, error) {
	cert := ""
	resp, err := http.Get("https://icco.auth0.com/.well-known/jwks.json")

	if err != nil {
		return cert, err
	}
	defer resp.Body.Close()

	var jwks = Jwks{}
	err = json.NewDecoder(resp.Body).Decode(&jwks)

	if err != nil {
		return cert, err
	}

	for _, k := range jwks.Keys {
		if token.Header["kid"] == k.Kid {
			cert = fmt.Sprintf("-----BEGIN CERTIFICATE-----\n%s\n-----END CERTIFICATE-----", k.X5c[0])
		}
	}

	if cert == "" {
		err := fmt.Errorf("unable to find appropriate key")
		return cert, err
	}

	return cert, nil
}
