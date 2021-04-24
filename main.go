package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/icco/gutil/logging"
	"github.com/icco/tab-archive/lib"
	"go.uber.org/zap"

	_ "github.com/lib/pq"
)

var (
	log        = logging.Must(logging.NewLogger("tab-archive"))
	GCPProject = "icco-cloud"
)

type pageData struct {
	TabCount  int64
	UserCount int64
}

func main() {
	port := "8080"
	if fromEnv := os.Getenv("PORT"); fromEnv != "" {
		port = fromEnv
	}
	log.Infow("Starting up", "host", fmt.Sprintf("http://localhost:%s", port))

	db, err := lib.InitDB(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalw("cannot connect to database", zap.Error(err))
	}
	defer db.Close()

	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(logging.Middleware(log.Desugar(), GCPProject))

	crs := cors.New(cors.Options{
		AllowCredentials:   true,
		OptionsPassthrough: false,
		AllowedOrigins:     []string{"*"},
		AllowedMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:     []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:     []string{"Link"},
		MaxAge:             300, // Maximum value not ignored by any of major browsers
	})
	r.Use(crs.Handler)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		tc, err := lib.TabCount(ctx, db)
		if err != nil {
			log.Errorw("tab count", zap.Error(err))
			jsError(w, err, http.StatusInternalServerError)
			return
		}

		uc, err := lib.UserCount(ctx, db)
		if err != nil {
			log.Errorw("user count", zap.Error(err))
			jsError(w, err, http.StatusInternalServerError)
			return
		}

		tmpl := template.Must(template.ParseFiles("index.tmpl"))
		if err := tmpl.Execute(w, &pageData{TabCount: tc, UserCount: uc}); err != nil {
			log.Errorw("template execution", zap.Error(err))
		}
	})

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hi."))
	})

	r.Post("/hook", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		tok := r.Header.Get("Authorization")
		if tok == "" {
			err := fmt.Errorf("no Authorization header")
			jsError(w, err, http.StatusUnauthorized)
			return
		}
		tok = strings.TrimPrefix(tok, "Bearer ")
		u, err := lib.GetUser(ctx, db, tok)
		if err != nil {
			log.Errorw("could not get user", zap.Error(err))
			jsError(w, err, http.StatusInternalServerError)
			return
		}

		buf, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Errorw("could not read buffer", zap.Error(err))
			jsError(w, err, http.StatusInternalServerError)
			return
		}

		ct := r.Header.Get("content-type")
		if ct != "application/json" {
			err := fmt.Errorf("expected 'application/json' content type, got %q", ct)
			log.Errorw("bad content type", zap.Error(err))
			jsError(w, err, http.StatusBadRequest)
			return
		}

		if err := lib.ParseAndStore(ctx, db, u, buf); err != nil {
			log.Errorw("could not parse request", zap.Error(err))
			jsError(w, err, http.StatusInternalServerError)
			return
		}

		w.Write([]byte(`{"status": "success"}`))
	})

	r.Get("/archive", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		j := json.NewEncoder(w)

		tok := r.Header.Get("Authorization")
		if tok == "" {
			err := fmt.Errorf("no Authorization header")
			log.Errorw("could not get user", zap.Error(err))
			jsError(w, err, http.StatusInternalServerError)
			return
		}
		tok = strings.TrimPrefix(tok, "Bearer ")
		u, err := lib.GetUser(ctx, db, tok)
		if err != nil {
			log.Errorw("could not get user", zap.Error(err))
			jsError(w, err, http.StatusInternalServerError)
			return
		}

		tabs, err := u.GetArchive(ctx, db)
		if err != nil {
			log.Errorw("could not get user tabs", zap.Error(err))
			jsError(w, err, http.StatusInternalServerError)
			return
		}

		if err := j.Encode(map[string]interface{}{
			"status": "success",
			"tabs":   tabs,
		}); err != nil {
			log.Errorw("could not marshal data", zap.Error(err))
			jsError(w, err, http.StatusInternalServerError)
			return
		}
	})

	log.Fatal(http.ListenAndServe(":"+port, r))
}

func jsError(w http.ResponseWriter, err error, statusCode int) {
	w.WriteHeader(statusCode)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.Encode(map[string]string{"error": err.Error()})
}
