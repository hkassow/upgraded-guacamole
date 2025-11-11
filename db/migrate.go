package db

import (
	"embed"
	"log"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func RunMigrations(pool *pgxpool.Pool) {
	log.Println("Running database migrations...")

	db := stdlib.OpenDBFromPool(pool)

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		log.Fatalf("Migration driver error: %v", err)
	}

	// Create iofs source driver from embedded FS
	d, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		log.Fatalf("Failed to create iofs driver: %v", err)
	}

	m, err := migrate.NewWithInstance("iofs", d, "postgres", driver)
	if err != nil {
		log.Fatalf("Migration init error: %v", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("Migration failed: %v", err)
	}

	log.Println("Database migrations applied successfully")

	version, dirty, err := m.Version()
	if err != nil {
		if err == migrate.ErrNilVersion {
			log.Println("No migrations applied yet")
		} else {
			log.Fatalf("Error getting migration version: %v", err)
		}
		return
	}

	if dirty {
		log.Printf("Migration version %d is dirty", version)
	} else {
		log.Printf("Current migration version: %d", version)
	}
}
