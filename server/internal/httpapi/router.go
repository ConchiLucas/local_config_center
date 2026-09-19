package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"local_config_center/internal/catalog"
	"local_config_center/internal/config"
	"local_config_center/internal/testhub"
)

type Server struct {
	loader      configLoader
	hub         *testhub.Hub
	webDir      string
	gatewayAuth string
}

type configLoader interface {
	LoadSnapshot(ctx context.Context) (config.Snapshot, error)
	Ping(ctx context.Context) error
}

func New(loader configLoader, hub *testhub.Hub, webDir, gatewayAuth string) http.Handler {
	s := &Server{loader: loader, hub: hub, webDir: webDir, gatewayAuth: gatewayAuth}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /readyz", s.ready)
	mux.HandleFunc("GET /api/catalog", s.catalog)
	mux.HandleFunc("GET /api/configs/{id}", s.config)
	mux.HandleFunc("POST /api/test-runs", s.createRun)
	mux.HandleFunc("GET /api/test-runs/{id}", s.getRun)
	mux.HandleFunc("GET /api/test-runs/{id}/events", s.events)
	mux.HandleFunc("POST /api/test-runs/{id}/cancel", s.cancel)
	mux.HandleFunc("GET /api/media/{id}", s.media)
	mux.HandleFunc("POST /v1/invoke", s.invoke)
	mux.HandleFunc("GET /v1/models", s.gatewayModels)
	mux.HandleFunc("POST /v1/models", s.gatewayModels)
	mux.HandleFunc("GET /v1/media/{id}", s.media)
	mux.HandleFunc("/", s.static)
	return noStore(rejectConfigWrites(mux))
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if err := s.loader.Ping(r.Context()); err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "UNAVAILABLE", "configuration database is not reachable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) catalog(w http.ResponseWriter, r *http.Request) {
	snapshot, err := s.loader.LoadSnapshot(r.Context())
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "UNAVAILABLE", "无法读取配置数据库")
		return
	}
	cat := catalog.Project(snapshot, time.Now(), s.hub.Verifications())
	writeJSON(w, http.StatusOK, cat)
}

func (s *Server) config(w http.ResponseWriter, r *http.Request) {
	snapshot, err := s.loader.LoadSnapshot(r.Context())
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "UNAVAILABLE", "无法读取配置数据库")
		return
	}
	cat := catalog.Project(snapshot, time.Now(), s.hub.Verifications())
	id := r.PathValue("id")
	cfg, ok := catalog.FindConfig(cat, id)
	if !ok {
		writeProblem(w, http.StatusNotFound, "NOT_FOUND", "找不到该配置")
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (s *Server) createRun(w http.ResponseWriter, r *http.Request) {
	var req testhub.CreateTestRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeProblem(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	live, err := s.hub.Create(r.Context(), req)
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"run": live.Snapshot()})
}

func (s *Server) getRun(w http.ResponseWriter, r *http.Request) {
	live, ok := s.hub.Get(r.PathValue("id"))
	if !ok {
		writeProblem(w, http.StatusNotFound, "NOT_FOUND", "找不到该测试运行")
		return
	}
	writeJSON(w, http.StatusOK, live.Snapshot())
}

func (s *Server) cancel(w http.ResponseWriter, r *http.Request) {
	run, err := s.hub.Cancel(r.PathValue("id"))
	if err != nil {
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	live, ok := s.hub.Get(r.PathValue("id"))
	if !ok {
		writeProblem(w, http.StatusNotFound, "NOT_FOUND", "找不到该测试运行")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeProblem(w, http.StatusInternalServerError, "INTERNAL", "streaming not supported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	ch, past, unsub := live.Subscribe()
	defer unsub()
	for _, event := range past {
		if err := writeSSE(w, event); err != nil {
			return
		}
		flusher.Flush()
		if event.Type == "done" {
			return
		}
	}
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			if err := writeSSE(w, event); err != nil {
				return
			}
			flusher.Flush()
			if event.Type == "done" {
				return
			}
		}
	}
}

func writeSSE(w io.Writer, event testhub.RunEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "id: %s\nevent: %s\ndata: %s\n\n", event.ID, event.Type, payload)
	return err
}

func (s *Server) media(w http.ResponseWriter, r *http.Request) {
	art, ok := s.hub.Artifact(r.PathValue("id"))
	if !ok {
		writeProblem(w, http.StatusNotFound, "NOT_FOUND", "找不到该媒体")
		return
	}
	file, err := os.Open(art.Path)
	if err != nil {
		writeProblem(w, http.StatusNotFound, "NOT_FOUND", "媒体文件不存在")
		return
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		writeProblem(w, http.StatusNotFound, "NOT_FOUND", "媒体文件不存在")
		return
	}
	w.Header().Set("Content-Type", art.MIME)
	w.Header().Set("Cache-Control", "private, max-age=60")
	http.ServeContent(w, r, art.Filename, stat.ModTime(), file)
}

func (s *Server) static(w http.ResponseWriter, r *http.Request) {
	if s.webDir == "" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "" || name == "." {
		name = "index.html"
	}
	if strings.Contains(name, "..") {
		http.NotFound(w, r)
		return
	}
	full := path.Join(s.webDir, name)
	info, err := os.Stat(full)
	if err != nil || info.IsDir() {
		index := path.Join(s.webDir, "index.html")
		if _, idxErr := os.Stat(index); idxErr == nil && !strings.HasPrefix(name, "api/") {
			http.ServeFile(w, r, index)
			return
		}
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, full)
}

func rejectConfigWrites(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") && r.URL.Path != "/api/test-runs" && !strings.HasSuffix(r.URL.Path, "/cancel") {
			switch r.Method {
			case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
				if strings.HasPrefix(r.URL.Path, "/api/test-runs") {
					break
				}
				writeProblem(w, http.StatusMethodNotAllowed, "READ_ONLY", "配置接口只读，不提供写入")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}
