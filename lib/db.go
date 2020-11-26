package lib

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/GuiaBolso/darwin"
	"github.com/opencensus-integrations/ocsql"

	// Needed to talk to postgres
	_ "github.com/lib/pq"
)

var (
	dbDriver   = "postgres"
	migrations = []darwin.Migration{
		{
			Version:     1,
			Description: "Creating table users",
			Script: `
      CREATE TABLE users (
        id          SERIAL PRIMARY KEY,
				name        TEXT,
				google_id   TEXT,
				created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
        modified_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
      );
      `,
		},
		{
			Version:     2,
			Description: "Creating table tabs",
			Script: `
      CREATE TABLE tabs (
        id          SERIAL PRIMARY KEY,
				user_id     INTEGER REFERENCES users (id),
        url         TEXT,
        favicon     TEXT,
        title       TEXT,
        seen        TIMESTAMP WITH TIME ZONE,
				created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
        modified_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
      );
      `,
		},
		{
			Version:     3,
			Description: "add not null restraint",
			Script:      `ALTER TABLE users ALTER COLUMN google_id SET NOT NULL`,
		},
		{
			Version:     4,
			Description: "add unique restraint",
			Script:      `ALTER TABLE users ADD UNIQUE (google_id)`,
		},
	}
)

// InitDB creates a package global db connection from a database string.
func InitDB(ctx context.Context, dataSourceName string) (*sql.DB, error) {
	// Connect to Database
	wrappedDriver, err := ocsql.Register(dbDriver, ocsql.WithAllTraceOptions())
	if err != nil {
		return nil, fmt.Errorf("failed to register the ocsql driver: %w", err)
	}

	db, err := sql.Open(wrappedDriver, dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("failed open db: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping: %w", err)
	}

	// Migrate
	driver := darwin.NewGenericDriver(db, darwin.PostgresDialect{})
	d := darwin.New(driver, migrations, nil)

	if err := d.Migrate(); err != nil {
		return nil, fmt.Errorf("migrate error: %w", err)
	}

	return db, err
}
