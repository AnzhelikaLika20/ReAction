package storage

import (
	"context"
	"fmt"
	"os"
	"time"

	"ReAction/internal/config"
	"ReAction/internal/storage/sqlc/gen"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

type PostgresStorage struct {
	pool    *pgxpool.Pool
	Queries *db.Queries
}

func NewPostgresStorage(ctx context.Context, cfg config.Database) (*PostgresStorage, error) {
	connstring := fmt.Sprintf(
		"host=%s port=%s dbname=%s user=%s password=%s sslmode=require target_session_attrs=read-write",
		cfg.Host, cfg.Port, cfg.DBName, cfg.User, cfg.Password)

	connConfig, err := pgxpool.ParseConfig(connstring)
	if err != nil {
		return nil, fmt.Errorf("Unable to parse config: %v\n", err)
	}

	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(connectCtx, connConfig)
	if err != nil {
		return nil, fmt.Errorf("Unable to connect to database: %v\n", err)
	}

	queries := db.New(pool)
	storage := &PostgresStorage{
		pool:    pool,
		Queries: queries,
	}

	if err := storage.applyMigrations(connstring); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to apply migrations: %w", err)
	}

	var version string
	err = pool.QueryRow(context.Background(), "select version()").Scan(&version)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("QueryRow failed: %v\n", err)
	}

	fmt.Println(version)

	return storage, nil
}

func (s *PostgresStorage) applyMigrations(dsn string) error {
	db, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		return fmt.Errorf("failed to connect for migrations: %w", err)
	}
	defer db.Close(context.Background())

	sqlDB := stdlib.OpenDB(*db.Config())
	defer sqlDB.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	migrationsDir := "internal/storage/sqlc/schema"
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		if err := os.MkdirAll(migrationsDir, 0755); err != nil {
			return fmt.Errorf("failed to create migrations directory: %w", err)
		}
	}

	if err := goose.Up(sqlDB, migrationsDir); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	return nil
}

func (s *PostgresStorage) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

func (s *PostgresStorage) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return s.pool.Begin(ctx)
}
