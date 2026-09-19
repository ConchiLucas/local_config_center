package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func OpenPostgres(ctx context.Context, databaseURL string) (*PostgresRepository, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}
	repository := &PostgresRepository{pool: pool}
	if err := repository.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return repository, nil
}

func (r *PostgresRepository) Close() {
	r.pool.Close()
}

func (r *PostgresRepository) Get(ctx context.Context, key string) (Document, bool, error) {
	if !validDocumentKey(key) {
		return Document{}, false, fmt.Errorf("invalid configuration document key")
	}
	var raw []byte
	var updatedAt time.Time
	err := r.pool.QueryRow(ctx, `SELECT content, updated_at FROM config_documents WHERE key = $1`, key).Scan(&raw, &updatedAt)
	if err == pgx.ErrNoRows {
		return Document{}, false, nil
	}
	if err != nil {
		return Document{}, false, fmt.Errorf("get configuration document: %w", err)
	}
	return Document{Key: key, Content: raw, UpdatedAt: updatedAt}, true, nil
}

func (r *PostgresRepository) Ping(ctx context.Context) error {
	return r.pool.Ping(ctx)
}

func validDocumentKey(key string) bool {
	return key == "ai" || key == "databases" || key == "objectStorage" || key == "local-cli"
}
