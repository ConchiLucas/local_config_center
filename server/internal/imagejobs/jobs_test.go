package imagejobs

import (
	"bytes"
	"context"
	"errors"
	"image/png"
	"os"
	"sync"
	"testing"
	"time"
)

func TestMockDeterministicRaster(t *testing.T) {
	a, mime, err := Mock(context.Background(), Request{Model: "mock/image", Prompt: "hello"})
	if err != nil || mime != "image/png" {
		t.Fatalf("mock: %s %v", mime, err)
	}
	b, _, _ := Mock(context.Background(), Request{Model: "mock/image", Prompt: "hello"})
	if !bytes.Equal(a, b) {
		t.Fatal("not deterministic")
	}
	if _, err := png.Decode(bytes.NewReader(a)); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresIdempotencyRecoveryAndFencing(t *testing.T) {
	url := os.Getenv("IMAGE_JOBS_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set IMAGE_JOBS_TEST_DATABASE_URL to a dedicated disposable database")
	}
	ctx := context.Background()
	var s *Store
	var err error
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
		s, err = Open(ctx, url)
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.pool.Exec(ctx, "TRUNCATE image_jobs.jobs"); err != nil {
		t.Fatal(err)
	}
	if second, err := Open(ctx, url); err == nil {
		second.Close()
		t.Fatal("second owner accepted")
	}
	req := Request{Model: "mock/image", Prompt: "test", Parameters: map[string]any{}}
	var wg sync.WaitGroup
	ids := make(chan string, 16)
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			j, e := s.Submit(ctx, "same-key", req)
			if e != nil {
				t.Error(e)
				return
			}
			ids <- j.ID
		}()
	}
	wg.Wait()
	close(ids)
	var id string
	for got := range ids {
		if id != "" && id != got {
			t.Fatal("duplicate jobs")
		}
		id = got
	}
	req.Prompt = "different"
	if _, err := s.Submit(ctx, "same-key", req); !errors.Is(err, ErrConflict) {
		t.Fatalf("conflict: %v", err)
	}
	j, err := s.claim(ctx)
	if err != nil {
		t.Fatal(err)
	}
	data, mime, _ := Mock(ctx, Request{Prompt: "test"})
	if err := s.finish(ctx, j, "succeeded", data, mime, nil); err != nil {
		t.Fatal(err)
	}
	interrupted, _ := s.Submit(ctx, "interrupted", req)
	oldClaim, err := s.claim(ctx)
	if err != nil {
		t.Fatal(err)
	}
	queued, _ := s.Submit(ctx, "queued", req)
	s.Close()
	next, err := Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	recovered, err := next.Get(ctx, id, false)
	if err != nil || recovered.State != "succeeded" {
		t.Fatalf("media state: %#v %v", recovered, err)
	}
	got, _, err := next.Media(ctx, id)
	if err != nil || !bytes.Equal(data, got) {
		t.Fatalf("durable media: %v", err)
	}
	unknown, _ := next.Get(ctx, interrupted.ID, false)
	if unknown.State != "unknown" || unknown.Error.OutcomeKnown {
		t.Fatalf("recovery: %#v", unknown)
	}
	if err := next.finish(ctx, oldClaim, "succeeded", data, mime, nil); !errors.Is(err, ErrOwnership) {
		t.Fatalf("stale completion: %v", err)
	}
	claimed, err := next.claim(ctx)
	if err != nil || claimed.ID != queued.ID {
		t.Fatalf("queued recovery: %#v %v", claimed, err)
	}
}
