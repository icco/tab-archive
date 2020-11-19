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
        id serial primary key,
				name text,
				google_id text,
				created_at timestamp with time zone,
        modified_at timestamp with time zone
      );
      `,
		},
		{
			Version:     2,
			Description: "Creating table tabs",
			Script: `
      CREATE TABLE tabs (
        id serial primary key,
				user_id integer REFERENCES users (id),
        url text,
        favicon text,
        title text,
        seen timestamp with time zone,
				created_at timestamp with time zone,
        modified_at timestamp with time zone
      );
      `,
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
