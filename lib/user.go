package lib

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"golang.org/x/oauth2"
	oauth2_api "google.golang.org/api/oauth2/v2"
)

type User struct {
	ID         int64
	Name       string
	GoogleID   string
	CreatedAt  time.Time
	ModifiedAt time.Time
}

func GetUser(ctx context.Context, db *sql.DB, authToken string) (*User, error) {
	f := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: authToken})
	c, err := oauth2_api.New(oauth2.NewClient(ctx, f))
	if err != nil {
		return nil, fmt.Errorf("could not make oauth2 api client: %w", err)
	}

	ti, err := c.Tokeninfo().AccessToken(authToken).Do()
	if err != nil {
		return nil, fmt.Errorf("could not get oauth2 token info: %w", err)
	}

	if ti.ExpiresIn <= 0 {
		return nil, fmt.Errorf("Token is expired.")
	}

	if !ti.VerifiedEmail {
		return nil, fmt.Errorf("Email not verified.")
	}

	var id int64
	if err := db.QueryRowContext(
		ctx,
		`INSERT INTO users (google_id) VALUES($1) ON CONFLICT ON (google_id) DO UPDATE SET modified_at = $2 RETURNING id`,
		ti.UserId,
		time.Now(),
		nil).Scan(&id); err != nil {
		return nil, fmt.Errorf("writing db entry: %w", err)
	}

	return loadUser(ctx, db, id)
}

func loadUser(ctx context.Context, db *sql.DB, id int64) (*User, error) {
	var u *User
	err := db.QueryRow(
		"SELECT name, google_id, created_at, modified_at FROM users WHERE id = ?",
		id).Scan(&u.Name, &u.GoogleID, &u.CreatedAt, &u.ModifiedAt)
	if err != nil {
		return nil, fmt.Errorf("could not get user %d: %w", id, err)
	}

	return u, nil
}
