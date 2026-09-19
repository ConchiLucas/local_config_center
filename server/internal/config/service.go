package config

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"local_config_center/internal/storage"
)

const (
	AIKey            = "ai"
	DatabasesKey     = "databases"
	ObjectStorageKey = "objectStorage"
	LocalCLIKey      = "local-cli"
)

type Snapshot struct {
	AI            AIConfiguration
	AIUpdatedAt   time.Time
	Databases     DatabaseConfiguration
	DBUpdatedAt   time.Time
	ObjectStorage ObjectStorageConfiguration
	OSUpdatedAt   time.Time
	LocalCLI      LocalCLIConfiguration
	CLIUpdatedAt  time.Time
}

type Service struct {
	repository storage.Repository
}

func NewService(repository storage.Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) Ping(ctx context.Context) error {
	return s.repository.Ping(ctx)
}

func (s *Service) LoadSnapshot(ctx context.Context) (Snapshot, error) {
	var snap Snapshot
	ai, at, err := s.getAI(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	snap.AI, snap.AIUpdatedAt = ai, at

	dbs, at, err := s.getDatabases(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	snap.Databases, snap.DBUpdatedAt = dbs, at

	storageCfg, at, err := s.getObjectStorage(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	snap.ObjectStorage, snap.OSUpdatedAt = storageCfg, at

	cli, at, err := s.getLocalCLI(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	snap.LocalCLI, snap.CLIUpdatedAt = cli, at
	return snap, nil
}

func (s *Service) getAI(ctx context.Context) (AIConfiguration, time.Time, error) {
	value := DefaultAI()
	doc, found, err := s.get(ctx, AIKey)
	if err != nil {
		return AIConfiguration{}, time.Time{}, err
	}
	if !found {
		return value, time.Time{}, nil
	}
	if err := json.Unmarshal(doc.Content, &value); err != nil {
		return AIConfiguration{}, time.Time{}, fmt.Errorf("%w: decode ai: %v", ErrStoredInvalid, err)
	}
	return NormalizeAI(value), doc.UpdatedAt, nil
}

func (s *Service) getDatabases(ctx context.Context) (DatabaseConfiguration, time.Time, error) {
	value := DefaultDatabases()
	doc, found, err := s.get(ctx, DatabasesKey)
	if err != nil {
		return DatabaseConfiguration{}, time.Time{}, err
	}
	if !found {
		return value, time.Time{}, nil
	}
	if err := json.Unmarshal(doc.Content, &value); err != nil {
		return DatabaseConfiguration{}, time.Time{}, fmt.Errorf("%w: decode databases: %v", ErrStoredInvalid, err)
	}
	return NormalizeDatabases(value), doc.UpdatedAt, nil
}

func (s *Service) getObjectStorage(ctx context.Context) (ObjectStorageConfiguration, time.Time, error) {
	value := DefaultObjectStorage()
	doc, found, err := s.get(ctx, ObjectStorageKey)
	if err != nil {
		return ObjectStorageConfiguration{}, time.Time{}, err
	}
	if !found {
		return value, time.Time{}, nil
	}
	if err := json.Unmarshal(doc.Content, &value); err != nil {
		return ObjectStorageConfiguration{}, time.Time{}, fmt.Errorf("%w: decode object storage: %v", ErrStoredInvalid, err)
	}
	return value, doc.UpdatedAt, nil
}

func (s *Service) getLocalCLI(ctx context.Context) (LocalCLIConfiguration, time.Time, error) {
	value := DefaultLocalCLI()
	doc, found, err := s.get(ctx, LocalCLIKey)
	if err != nil {
		return LocalCLIConfiguration{}, time.Time{}, err
	}
	if !found {
		return value, time.Time{}, nil
	}
	if err := json.Unmarshal(doc.Content, &value); err != nil {
		return LocalCLIConfiguration{}, time.Time{}, fmt.Errorf("%w: decode local CLI: %v", ErrStoredInvalid, err)
	}
	return NormalizeLocalCLI(value), doc.UpdatedAt, nil
}

func (s *Service) get(ctx context.Context, key string) (storage.Document, bool, error) {
	doc, found, err := s.repository.Get(ctx, key)
	if err != nil {
		return storage.Document{}, false, fmt.Errorf("%w: get %s: %v", ErrUnavailable, key, err)
	}
	return doc, found, nil
}
