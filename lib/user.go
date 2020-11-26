package lib

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	oAPI "google.golang.org/api/oauth2/v2"
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
	c, err := oAPI.New(oauth2.NewClient(ctx, f))
	if err != nil {
		return nil, fmt.Errorf("could not make oauth2 api client: %w", err)
	}

	ti, err := c.Tokeninfo().AccessToken(authToken).Do()
	if err != nil {
		return nil, fmt.Errorf("could not get oauth2 token info: %w", err)
	}

	if ti.ExpiresIn <= 0 {
		return nil, fmt.Errorf("token is expired")
	}

	ui, err := oAPI.NewUserinfoService(c).Get().Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("could not get user info: %w", err)
	}

	var id int64
	if err := db.QueryRowContext(
		ctx,
		`INSERT INTO users (google_id, name)
    VALUES($1, $3)
    ON CONFLICT (google_id) DO UPDATE
    SET (name, modified_at) = ($3, $2)
    WHERE users.google_id = $1
    RETURNING id`,
		ti.UserId,
		time.Now(),
		ui.Name).Scan(&id); err != nil {
		return nil, fmt.Errorf("writing db entry: %w", err)
	}
	u, err := loadUser(ctx, db, id)
	if err != nil {
		return nil, err
	}
	log.WithFields(logrus.Fields{
		"token_info": ti,
		"user":       u,
		"user_info":  ui,
	}).Debug("user found")

	return u, nil
}

func loadUser(ctx context.Context, db *sql.DB, id int64) (*User, error) {
	u := &User{}
	u.ID = id
	err := db.QueryRowContext(
		ctx,
		`SELECT google_id, created_at, modified_at FROM users WHERE id = $1`,
		id).Scan(&u.GoogleID, &u.CreatedAt, &u.ModifiedAt)
	switch {
	case err == sql.ErrNoRows:
		return nil, fmt.Errorf("no user with id %d", id)
	case err != nil:
		return nil, fmt.Errorf("could not get user %d: %w", id, err)
	}

	return u, nil
}

func (u *User) GetArchive(ctx context.Context, db *sql.DB) ([]*Tab, error) {
	limit := 1000
	offset := 0

	rows, err := db.QueryContext(ctx, `
  SELECT title, url, favicon, seen
  FROM tabs
	WHERE user_id = $3
  ORDER BY seen DESC
  LIMIT $1 OFFSET $2`,
		limit,
		offset,
		u.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tabs []*Tab
	for rows.Next() {
		t := new(Tab)
		if err := rows.Scan(&t.Title, &t.URL, &t.Favicon, &t.Seen); err != nil {
			return nil, err
		}

		tabs = append(tabs, t)
	}

	return tabs, nil
}
