package storage

import (
	"context"
	"encoding/json"
	"time"
)

type Document struct {
	Key       string
	Content   json.RawMessage
	UpdatedAt time.Time
}

type Repository interface {
	Get(ctx context.Context, key string) (Document, bool, error)
	Ping(ctx context.Context) error
}
