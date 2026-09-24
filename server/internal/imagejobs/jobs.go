// Package imagejobs owns durable gateway jobs independently of config_documents.
package imagejobs

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrConflict = errors.New("idempotency key belongs to a different request")
var ErrOwnership = errors.New("job ownership lost")
var ErrNotFound = errors.New("job not found")

type Request struct {
	Model      string         `json:"model"`
	Prompt     string         `json:"prompt"`
	Parameters map[string]any `json:"parameters"`
}
type JobError struct {
	Code         string `json:"code"`
	Message      string `json:"message"`
	OutcomeKnown bool   `json:"outcome_known"`
	Retryable    bool   `json:"retryable"`
}
type Result struct {
	Path      string `json:"path"`
	MediaType string `json:"media_type"`
}
type Job struct {
	ID           string    `json:"id"`
	Model        string    `json:"model"`
	State        string    `json:"state"`
	Error        *JobError `json:"error,omitempty"`
	Result       *Result   `json:"result,omitempty"`
	BilledAmount *string   `json:"billed_amount"`
	request      Request
	owner        string
}
type Executor func(context.Context, Request) ([]byte, string, *JobError)
type Store struct {
	pool  *pgxpool.Pool
	conn  *pgx.Conn
	owner string
	once  sync.Once
}

// The dedicated connection holds a session advisory lock. All worker state writes
// use this connection, so loss of the lock also prevents further claims/writes.
func Open(ctx context.Context, url string) (*Store, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		pool.Close()
		return nil, err
	}
	s := &Store{pool: pool, conn: conn, owner: rand.Text()}
	fail := func(err error) (*Store, error) { s.Close(); return nil, err }
	var locked bool
	if err = conn.QueryRow(ctx, "SELECT pg_try_advisory_lock(781493620019)").Scan(&locked); err != nil {
		return fail(err)
	}
	if !locked {
		return fail(errors.New("image jobs executor already owned by another config-center instance"))
	}
	_, err = conn.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS image_jobs;
 CREATE TABLE IF NOT EXISTS image_jobs.jobs (
 id text PRIMARY KEY, idem_key text UNIQUE NOT NULL, payload_hash text NOT NULL,
 request jsonb NOT NULL, state text NOT NULL, owner text NOT NULL DEFAULT '',
 job jsonb NOT NULL, image bytea, media_type text, created_at timestamptz NOT NULL DEFAULT now());
 CREATE INDEX IF NOT EXISTS jobs_queued ON image_jobs.jobs(created_at) WHERE state='queued';`)
	if err != nil {
		return fail(err)
	}
	recovery := &JobError{Code: "EXECUTION_INTERRUPTED", Message: "Execution was interrupted; upstream outcome and billing are unknown.", OutcomeKnown: false, Retryable: false}
	raw, _ := json.Marshal(recovery)
	_, err = conn.Exec(ctx, `UPDATE image_jobs.jobs SET state='unknown', owner='', job=job || jsonb_build_object('state','unknown','error',$1::jsonb) WHERE state='running'`, raw)
	if err != nil {
		return fail(err)
	}
	return s, nil
}
func (s *Store) Close() {
	s.once.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.conn.Close(ctx)
		s.pool.Close()
	})
}
func (s *Store) Submit(ctx context.Context, key string, req Request) (Job, error) {
	raw, err := json.Marshal(req)
	if err != nil {
		return Job{}, err
	}
	sum := sha256.Sum256(raw)
	hash := hex.EncodeToString(sum[:])
	job := Job{ID: "job-" + rand.Text(), Model: req.Model, State: "queued"}
	if req.Model == "mock/image" {
		zero := "0"
		job.BilledAmount = &zero
	}
	meta, _ := json.Marshal(job)
	// The UNIQUE constraint serializes concurrent submissions across all clients.
	_, err = s.pool.Exec(ctx, `INSERT INTO image_jobs.jobs(id,idem_key,payload_hash,request,state,job) VALUES($1,$2,$3,$4,'queued',$5) ON CONFLICT(idem_key) DO NOTHING`, job.ID, key, hash, raw, meta)
	if err != nil {
		return Job{}, err
	}
	var existingHash string
	err = s.pool.QueryRow(ctx, `SELECT payload_hash,job FROM image_jobs.jobs WHERE idem_key=$1`, key).Scan(&existingHash, &meta)
	if err != nil {
		return Job{}, err
	}
	if hash != existingHash {
		return Job{}, ErrConflict
	}
	err = json.Unmarshal(meta, &job)
	return job, err
}
func (s *Store) Get(ctx context.Context, value string, byKey bool) (Job, error) {
	column := "id"
	if byKey {
		column = "idem_key"
	}
	var raw []byte
	err := s.pool.QueryRow(ctx, "SELECT job FROM image_jobs.jobs WHERE "+column+"=$1", value).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrNotFound
	}
	if err != nil {
		return Job{}, err
	}
	var job Job
	err = json.Unmarshal(raw, &job)
	return job, err
}
func (s *Store) Media(ctx context.Context, id string) ([]byte, string, error) {
	var data []byte
	var mime string
	err := s.pool.QueryRow(ctx, `SELECT image,media_type FROM image_jobs.jobs WHERE id=$1 AND state='succeeded'`, id).Scan(&data, &mime)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrNotFound
	}
	return data, mime, err
}
func (s *Store) claim(ctx context.Context) (Job, error) {
	var raw, payload []byte
	err := s.conn.QueryRow(ctx, `UPDATE image_jobs.jobs SET state='running',owner=$1,job=job || '{"state":"running"}'::jsonb
 WHERE id=(SELECT id FROM image_jobs.jobs WHERE state='queued' ORDER BY created_at,id LIMIT 1) RETURNING job,request`, s.owner).Scan(&raw, &payload)
	if err != nil {
		return Job{}, err
	}
	var j Job
	if err = json.Unmarshal(raw, &j); err != nil {
		return Job{}, err
	}
	err = json.Unmarshal(payload, &j.request)
	j.owner = s.owner
	return j, err
}
func (s *Store) finish(ctx context.Context, j Job, state string, data []byte, mime string, jobErr *JobError) error {
	j.State = state
	j.Error = jobErr
	if state == "succeeded" {
		j.Result = &Result{Path: "/v1/image-jobs/" + j.ID + "/image", MediaType: mime}
	}
	raw, _ := json.Marshal(j)
	tag, err := s.conn.Exec(ctx, `UPDATE image_jobs.jobs SET state=$3,job=$4,image=$5,media_type=$6 WHERE id=$1 AND owner=$2 AND state='running'`, j.ID, j.owner, state, raw, data, mime)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrOwnership
	}
	return nil
}

// Run returns on any database error: the application must stop rather than use a
// replacement session that no longer owns the advisory lock.
func (s *Store) Run(ctx context.Context, execute Executor) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		j, err := s.claim(ctx)
		if errors.Is(err, pgx.ErrNoRows) {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(200 * time.Millisecond):
				continue
			}
		}
		if err != nil {
			return err
		}
		data, mime, jobErr := execute(ctx, j.request)
		state := "succeeded"
		if jobErr != nil {
			state = "failed"
			if !jobErr.OutcomeKnown {
				state = "unknown"
			}
		}
		if err = s.finish(ctx, j, state, data, mime, jobErr); err != nil {
			return err
		}
	}
}
func Mock(_ context.Context, req Request) ([]byte, string, *JobError) {
	sum := sha256.Sum256([]byte(req.Prompt))
	img := image.NewRGBA(image.Rect(0, 0, 256, 256))
	for y := 0; y < 256; y++ {
		for x := 0; x < 256; x++ {
			img.SetRGBA(x, y, color.RGBA{R: sum[0] ^ uint8(x), G: sum[1] ^ uint8(y), B: sum[2], A: 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		return nil, "", &JobError{Code: "MOCK_ERROR", Message: fmt.Sprint(err), OutcomeKnown: true}
	}
	return b.Bytes(), "image/png", nil
}
