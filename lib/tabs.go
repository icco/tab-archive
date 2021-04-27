package lib

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Tab is a single tab to archive.
type Tab struct {
	URL     string    `json:"url"`
	Title   string    `json:"title"`
	Favicon string    `json:"favicon"`
	Seen    time.Time `json:"seen"`
}

// ParseAndStore parses a tabe and stores to the db.
func ParseAndStore(ctx context.Context, db *sql.DB, u *User, buf []byte) error {
	log.Debugw("attempting to parse", "body", string(buf))
	var t *Tab
	if err := json.Unmarshal(buf, &t); err != nil {
		return fmt.Errorf("could not parse: %w", err)
	}

	log.Debugw("parsed", "tab", t, "user", u)
	if _, err := db.ExecContext(
		ctx,
		`INSERT INTO tabs (title, url, seen, favicon, user_id) VALUES ($1, $2, $3, $4, $5)`,
		t.Title,
		t.URL,
		t.Seen,
		t.Favicon,
		u.ID); err != nil {
		return fmt.Errorf("writing db entry: %w", err)
	}

	return nil
}

// TabCount counts the number of tabs in the db.
func TabCount(ctx context.Context, db *sql.DB) (int64, error) {
	var i int64
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) from tabs`).Scan(&i)
	switch {
	case err == sql.ErrNoRows:
		return 0, fmt.Errorf("no rows found")
	case err != nil:
		return 0, fmt.Errorf("count query failed: %w", err)
	}

	return i, nil
}
