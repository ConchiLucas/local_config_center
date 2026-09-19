package httpapi

import (
	"net/http"
	"strings"
	"time"

	"local_config_center/internal/catalog"
	"local_config_center/internal/gateway"
	"local_config_center/internal/testhub"
)

type invokeRequest struct {
	Auth   string          `json:"auth"`
	Kind   string          `json:"kind"`
	Model  string          `json:"model"`
	Stream bool            `json:"stream"`
	Format string          `json:"format"`
	Input  testhub.TestInput `json:"input"`
}

type modelsAuthRequest struct {
	Auth string `json:"auth"`
}

func (s *Server) invoke(w http.ResponseWriter, r *http.Request) {
	var req invokeRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeProblem(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	if !gateway.Match(req.Auth, s.gatewayAuth) {
		writeProblem(w, http.StatusUnauthorized, "UNAUTHORIZED", "鉴权不通过")
		return
	}
	kind := strings.TrimSpace(req.Kind)
	if kind == "" {
		kind = strings.TrimSpace(req.Input.Kind)
	}
	if kind == "" {
		writeProblem(w, http.StatusBadRequest, "INVALID_INPUT", "kind 不能为空")
		return
	}
	vendor, slug := gateway.SplitModel(req.Model)
	if slug == "" {
		slug = strings.TrimSpace(req.Input.Model)
	}
	if slug == "" {
		writeProblem(w, http.StatusBadRequest, "INVALID_INPUT", "model 不能为空")
		return
	}
	snapshot, err := s.loader.LoadSnapshot(r.Context())
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "UNAVAILABLE", "无法读取配置数据库")
		return
	}
	cat := catalog.Project(snapshot, time.Now(), s.hub.Verifications())
	if vendor == "" {
		vendor = gateway.InferVendor(cat, kind, slug)
	}
	if vendor == "" {
		writeProblem(w, http.StatusBadRequest, "INVALID_INPUT", "无法识别模型所属供应商，请使用 vendor/model，例如 antigravity/gemini-3.8-flash-high")
		return
	}
	slug = gateway.ComposeImageSlug(vendor, slug, req.Input.Resolution, req.Input.Aspect)
	req.Input.Kind = kind
	req.Input.Model = slug
	preset := &testhub.OutputPreset{Delivery: testhub.DeliveryComplete, Format: testhub.FormatText}
	if req.Stream {
		preset.Delivery = testhub.DeliveryStream
	}
	if strings.EqualFold(req.Format, "json") {
		preset.Format = testhub.FormatJSON
		preset.Delivery = testhub.DeliveryComplete
	}
	live, err := s.hub.Invoke(r.Context(), testhub.CreateTestRequest{
		ConfigID:     gateway.ConfigID(vendor, kind),
		Input:        req.Input,
		OutputPreset: preset,
	})
	if err != nil {
		writeAPIError(w, err)
		return
	}
	if req.Stream && kind == "chat" {
		s.streamLive(w, r, live)
		return
	}
	run := live.Wait(r.Context())
	status := http.StatusOK
	if run.State == testhub.StateFailed {
		status = http.StatusBadGateway
	}
	writeJSON(w, status, map[string]any{"run": run})
}

func (s *Server) streamLive(w http.ResponseWriter, r *http.Request, live *testhub.LiveRun) {
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

func (s *Server) gatewayModels(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.URL.Query().Get("auth"))
	if r.Method == http.MethodPost {
		var req modelsAuthRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeProblem(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
			return
		}
		token = req.Auth
	}
	if !gateway.Match(token, s.gatewayAuth) {
		writeProblem(w, http.StatusUnauthorized, "UNAUTHORIZED", "鉴权不通过")
		return
	}
	snapshot, err := s.loader.LoadSnapshot(r.Context())
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "UNAVAILABLE", "无法读取配置数据库")
		return
	}
	cat := catalog.Project(snapshot, time.Now(), s.hub.Verifications())
	models := make([]map[string]any, 0)
	for _, group := range cat.Groups {
		for _, page := range group.Pages {
			for _, cfg := range page.Configs {
				if len(cfg.Models) == 0 {
					models = append(models, map[string]any{
						"id":     group.ID + "/" + cfg.Model,
						"kind":   cfg.Kind,
						"vendor": group.ID,
						"label":  cfg.Title,
						"model":  cfg.Model,
					})
					continue
				}
				for _, item := range cfg.Models {
					models = append(models, map[string]any{
						"id":     group.ID + "/" + item.Model,
						"kind":   cfg.Kind,
						"vendor": group.ID,
						"label":  item.Label,
						"model":  item.Model,
						"note":   item.Note,
					})
				}
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"models": models})
}
