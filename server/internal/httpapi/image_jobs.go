package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"local_config_center/internal/catalog"
	"local_config_center/internal/gateway"
	"local_config_center/internal/imagejobs"
	"local_config_center/internal/testhub"
)

func (s *Server) imageJobAuth(w http.ResponseWriter, r *http.Request) bool {
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || !gateway.Match(parts[1], s.gatewayAuth) {
		writeProblem(w, 401, "UNAUTHORIZED", "Bearer authentication required")
		return false
	}
	w.Header().Set("Cache-Control", "no-store")
	return true
}
func (s *Server) imageCapabilities(w http.ResponseWriter, r *http.Request) {
	if !s.imageJobAuth(w, r) {
		return
	}
	writeJSON(w, 200, map[string]any{"version": 1, "real_enabled": s.imageRealEnabled, "parameters": "configured_defaults"})
}
func (s *Server) imageJob(w http.ResponseWriter, r *http.Request) {
	if !s.imageJobAuth(w, r) {
		return
	}
	if s.imageJobs == nil {
		writeProblem(w, 503, "UNAVAILABLE", "Image job storage is unavailable")
		return
	}
	if r.Method == http.MethodPost {
		s.submitImageJob(w, r)
		return
	}
	id := r.PathValue("id")
	key := r.PathValue("key")
	byKey := key != ""
	if byKey {
		id = key
	}
	if strings.HasSuffix(r.URL.Path, "/image") && !byKey {
		data, mime, err := s.imageJobs.Media(r.Context(), id)
		if err != nil {
			s.imageJobProblem(w, err)
			return
		}
		w.Header().Set("Content-Type", mime)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = w.Write(data)
		return
	}
	job, err := s.imageJobs.Get(r.Context(), id, byKey)
	if err != nil {
		s.imageJobProblem(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"job": job})
}
func (s *Server) submitImageJob(w http.ResponseWriter, r *http.Request) {
	var req imagejobs.Request
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeProblem(w, 400, "INVALID_INPUT", "Expected model, prompt and empty parameters object")
		return
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		writeProblem(w, 400, "INVALID_INPUT", "Expected one JSON object")
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if strings.TrimSpace(key) == "" || len(key) > 200 || strings.TrimSpace(req.Model) == "" || len(req.Model) > 250 || strings.TrimSpace(req.Prompt) == "" || utf8.RuneCountInString(req.Prompt) > 20000 || req.Parameters == nil || len(req.Parameters) != 0 {
		writeProblem(w, 400, "INVALID_INPUT", "Idempotency-Key, model, prompt and parameters:{} are required; custom parameters are not supported")
		return
	}
	// Existing keys remain reconcilable even if the real execution gate changed.
	if _, err := s.imageJobs.Get(r.Context(), key, true); errors.Is(err, imagejobs.ErrNotFound) {
		if req.Model != "mock/image" {
			if !s.imageRealEnabled {
				writeProblem(w, 403, "REAL_IMAGE_DISABLED", "Real image jobs are disabled")
				return
			}
			if !s.imageModelAllowed(r.Context(), req.Model) {
				writeProblem(w, 400, "INVALID_MODEL", "Choose an image model from the gateway catalog")
				return
			}
		}
	} else if err != nil {
		s.imageJobProblem(w, err)
		return
	}
	job, err := s.imageJobs.Submit(r.Context(), key, req)
	if err != nil {
		s.imageJobProblem(w, err)
		return
	}
	writeJSON(w, 202, map[string]any{"job": job})
}
func (s *Server) imageJobProblem(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, imagejobs.ErrNotFound):
		writeProblem(w, 404, "NOT_FOUND", "Image job not found")
	case errors.Is(err, imagejobs.ErrConflict):
		writeProblem(w, 409, "IDEMPOTENCY_CONFLICT", "Idempotency key already belongs to a different payload")
	default:
		writeProblem(w, 503, "UNAVAILABLE", "Image job storage is unavailable")
	}
}
func (s *Server) imageModelAllowed(ctx context.Context, model string) bool {
	snapshot, err := s.loader.LoadSnapshot(ctx)
	if err != nil {
		return false
	}
	cat := catalog.Project(snapshot, time.Now(), s.hub.Verifications())
	for _, group := range cat.Groups {
		for _, page := range group.Pages {
			for _, cfg := range page.Configs {
				if cfg.Kind != "image" {
					continue
				}
				if len(cfg.Models) == 0 && model == group.ID+"/"+cfg.Model {
					return true
				}
				for _, item := range cfg.Models {
					if model == group.ID+"/"+item.Model {
						return true
					}
				}
			}
		}
	}
	return false
}

// ImageExecutor invokes only catalog-backed models using the existing hub.
func ImageExecutor(loader configLoader, hub *testhub.Hub, realEnabled bool) imagejobs.Executor {
	s := &Server{loader: loader, hub: hub}
	return func(ctx context.Context, req imagejobs.Request) ([]byte, string, *imagejobs.JobError) {
		if req.Model == "mock/image" {
			return imagejobs.Mock(ctx, req)
		}
		known := func(code, message string) ([]byte, string, *imagejobs.JobError) {
			return nil, "", &imagejobs.JobError{Code: code, Message: message, OutcomeKnown: true}
		}
		if !realEnabled {
			return known("REAL_IMAGE_DISABLED", "Real image jobs are disabled")
		}
		if !s.imageModelAllowed(ctx, req.Model) {
			return known("INVALID_MODEL", "The configured image model is unavailable")
		}
		vendor, slug := gateway.SplitModel(req.Model)
		live, err := hub.Invoke(ctx, testhub.CreateTestRequest{ConfigID: gateway.ConfigID(vendor, "image"), Input: testhub.TestInput{Kind: "image", Model: slug, Prompt: req.Prompt}})
		if err != nil {
			return known("PRE_DISPATCH_REJECTED", "Gateway could not dispatch this image request")
		}
		waitCtx, cancel := context.WithTimeout(ctx, 6*time.Minute)
		defer cancel()
		run := live.Wait(waitCtx)
		unknown := func() ([]byte, string, *imagejobs.JobError) {
			return nil, "", &imagejobs.JobError{Code: "UPSTREAM_OUTCOME_UNKNOWN", Message: "Upstream outcome or image persistence could not be confirmed; do not automatically retry.", OutcomeKnown: false, Retryable: false}
		}
		if run.State != testhub.StateSucceeded {
			return unknown()
		}
		for _, block := range run.Outputs {
			refs := block.Assets
			if block.Asset != nil {
				refs = append(refs, *block.Asset)
			}
			for _, ref := range refs {
				art, ok := hub.Artifact(ref.AssetID)
				if !ok {
					continue
				}
				f, err := os.Open(art.Path)
				if err != nil {
					return unknown()
				}
				data, mime, err := readJobImage(f)
				_ = f.Close()
				if err != nil {
					return unknown()
				}
				return data, mime, nil
			}
		}
		return unknown()
	}
}

// Durable job images share the batch worker's raster formats and byte limit.
func readJobImage(r io.Reader) ([]byte, string, error) {
	data, err := io.ReadAll(io.LimitReader(r, (20<<20)+1))
	if err != nil {
		return nil, "", err
	}
	if len(data) > 20<<20 {
		return nil, "", errors.New("image exceeds 20 MiB")
	}
	mime := http.DetectContentType(data)
	switch mime {
	case "image/png", "image/jpeg":
		if _, _, err := image.DecodeConfig(bytes.NewReader(data)); err != nil {
			return nil, "", err
		}
	case "image/webp":
		if len(data) < 30 {
			return nil, "", errors.New("invalid WebP image")
		}
	default:
		return nil, "", errors.New("unsupported image format")
	}
	return data, mime, nil
}
