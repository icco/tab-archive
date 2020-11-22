package lib

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

type Tab struct {
	URL     string
	Title   string
	Favicon string
	Seen    time.Time
}

func ParseAndStore(ctx context.Context, db *sql.DB, u *User, buf []byte) error {
	log.WithField("body", string(buf)).Debug("attempting to parse")
	var t *Tab
	if err := json.Unmarshal(buf, &t); err != nil {
		return fmt.Errorf("could not parse: %w", err)
	}

	log.WithFields(logrus.Fields{
		"tab":  t,
		"user": u,
	}).Debug("parsed")
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
