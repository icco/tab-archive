package lib

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type Tab struct {
	URL     string
	Title   string
	Favicon string
	Seen    time.Time
}

func ParseAndStore(ctx context.Context, db *sql.DB, buf []byte) error {
	log.WithField("body", string(buf)).Debug("attempting to parse")
	var t *Tab
	if err := json.Unmarshal(buf, &t); err != nil {
		return fmt.Errorf("could not parse: %w", err)
	}

	log.WithField("tab", t).Debug("parsed")

	// TODO: Get user, insert if new

	if _, err := db.ExecContext(
		ctx,
		`INSERT INTO tabs (title, url, seen, favicon, user_id) VALUES ($1, $2, $3, $4, $5)`,
		t.Title,
		t.URL,
		t.Seen,
		t.Favicon,
		nil); err != nil {
		return fmt.Errorf("writing db entry: %w", err)
	}

	return nil
}
