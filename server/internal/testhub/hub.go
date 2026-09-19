package testhub

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync"
	"time"

	"local_config_center/internal/catalog"
	"local_config_center/internal/config"
	"local_config_center/internal/rewrite"
)

type Loader interface {
	LoadSnapshot(ctx context.Context) (config.Snapshot, error)
}

type Hub struct {
	loader     Loader
	artifacts  *ArtifactStore
	rewriter   rewrite.HostSide
	mu         sync.Mutex
	runs       map[string]*liveRun
	byIdem     map[string]string
	verified   map[string]catalog.Verification
	httpLong   *httpClientSet
}

type httpClientSet struct {
	chat     *http.Client
	media    *http.Client
	download *http.Client
}

func NewHub(loader Loader, artifacts *ArtifactStore, rewriter rewrite.HostSide) *Hub {
	return &Hub{
		loader:    loader,
		artifacts: artifacts,
		rewriter:  rewriter,
		runs:      map[string]*liveRun{},
		byIdem:    map[string]string{},
		verified:  map[string]catalog.Verification{},
		httpLong: &httpClientSet{
			chat:     newHTTPClient(0),
			media:    newHTTPClient(3 * time.Minute),
			download: newHTTPClient(2 * time.Minute),
		},
	}
}

func (h *Hub) Verifications() map[string]catalog.Verification {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make(map[string]catalog.Verification, len(h.verified))
	for k, v := range h.verified {
		out[k] = v
	}
	return out
}

func (h *Hub) Invoke(ctx context.Context, req CreateTestRequest) (*liveRun, error) {
	req.ExpectedRevision = ""
	return h.create(ctx, req, false)
}

func (h *Hub) Create(ctx context.Context, req CreateTestRequest) (*liveRun, error) {
	return h.create(ctx, req, true)
}

func (h *Hub) create(ctx context.Context, req CreateTestRequest, requireRevision bool) (*liveRun, error) {
	if stringsTrim(req.ConfigID) == "" {
		return nil, APIError{Status: 400, Code: "INVALID_INPUT", Message: "configId 不能为空"}
	}
	if requireRevision && stringsTrim(req.ExpectedRevision) == "" {
		return nil, APIError{Status: 400, Code: "INVALID_INPUT", Message: "expectedRevision 不能为空"}
	}
	snapshot, err := h.loader.LoadSnapshot(ctx)
	if err != nil {
		return nil, APIError{Status: 503, Code: "UNAVAILABLE", Message: "无法读取配置数据库"}
	}
	binding, err := Bind(snapshot, req.ConfigID)
	if err != nil {
		return nil, APIError{Status: 404, Code: "NOT_FOUND", Message: "找不到该配置"}
	}
	if picked := pickProvider(binding, req.Input.Model); picked != nil {
		binding.Provider = picked
	}
	if requireRevision && binding.Public.Revision != req.ExpectedRevision {
		return nil, APIError{Status: 409, Code: "CONFIG_CHANGED", Message: "配置版本已变化，请刷新后重新确认"}
	}
	if err := validateInput(binding, req); err != nil {
		return nil, err
	}

	if key := stringsTrim(req.IdempotencyKey); key != "" {
		h.mu.Lock()
		if runID, ok := h.byIdem[key]; ok {
			live := h.runs[runID]
			h.mu.Unlock()
			return live, nil
		}
		h.mu.Unlock()
	}

	id := "run-" + newAssetID()[:16]
	now := time.Now().UTC()
	runCtx, cancel := context.WithCancel(context.Background())
	live := &liveRun{
		run: TestRun{
			ID:             id,
			ConfigID:       binding.Public.ID,
			ConfigRevision: binding.Public.Revision,
			State:          StateRunning,
			StageLabel:     "正在调用已登记服务",
			Outputs:        []OutputBlock{},
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		cancel:    cancel,
		done:      make(chan struct{}),
		startedAt: now,
	}
	h.mu.Lock()
	h.runs[id] = live
	if key := stringsTrim(req.IdempotencyKey); key != "" {
		h.byIdem[key] = id
	}
	h.mu.Unlock()
	go h.execute(runCtx, live, binding, req)
	return live, nil
}

func (h *Hub) Get(id string) (*liveRun, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	live, ok := h.runs[id]
	return live, ok
}

func (h *Hub) Artifact(id string) (Artifact, bool) {
	return h.artifacts.Get(id)
}

func (h *Hub) Cancel(id string) (TestRun, error) {
	live, ok := h.Get(id)
	if !ok {
		return TestRun{}, APIError{Status: 404, Code: "NOT_FOUND", Message: "找不到该测试运行"}
	}
	live.mu.Lock()
	if live.run.State == StateSucceeded || live.run.State == StateFailed || live.run.State == StateCancelled {
		out := cloneRun(live.run)
		live.mu.Unlock()
		return out, nil
	}
	live.run.Cancellation = "requested"
	live.run.StageLabel = "已停止等待"
	live.mu.Unlock()
	if live.cancel != nil {
		live.cancel()
	}
	live.mu.Lock()
	out := cloneRun(live.run)
	live.mu.Unlock()
	return out, nil
}

func (h *Hub) execute(ctx context.Context, live *liveRun, binding Binding, req CreateTestRequest) {
	defer close(live.done)
	h.emit(live, RunEvent{Type: "status", RunID: live.run.ID, State: StateRunning, StageLabel: live.run.StageLabel})
	err := h.dispatch(ctx, live, binding, req)
	elapsed := time.Since(live.startedAt).Milliseconds()
	live.mu.Lock()
	if live.run.Metrics == nil {
		live.run.Metrics = &Metrics{}
	}
	live.run.Metrics.ElapsedMs = elapsed
	if ctx.Err() != nil && live.run.State == StateRunning {
		live.run.State = StateFailed
		live.run.Cancellation = "unsupported"
		live.run.Error = &RunError{Code: "STOPPED_WAITING", SafeMessage: "已停止等待，上游可能仍在运行并计费", RequestID: live.run.ID}
		live.run.StageLabel = "已停止等待"
	} else if err != nil && live.run.State == StateRunning {
		live.run.State = StateFailed
		live.run.Error = asRunError(live.run.ID, err)
		live.run.StageLabel = "调用失败"
	} else if live.run.State == StateRunning {
		live.run.State = StateSucceeded
		live.run.StageLabel = "已完成"
		h.mu.Lock()
		h.verified[binding.Public.ID] = catalog.Verification{
			At:       time.Now().UTC().Format(time.RFC3339),
			Revision: binding.Public.Revision,
			Status:   "passed",
		}
		h.mu.Unlock()
	}
	live.run.UpdatedAt = time.Now().UTC()
	final := cloneRun(live.run)
	live.mu.Unlock()
	if final.State == StateFailed {
		h.emit(live, RunEvent{Type: "error", RunID: final.ID, Code: final.Error.Code, SafeMessage: final.Error.SafeMessage, RequestID: final.Error.RequestID})
	}
	h.emit(live, RunEvent{Type: "done", Run: &final})
}

func (h *Hub) dispatch(ctx context.Context, live *liveRun, binding Binding, req CreateTestRequest) error {
	switch binding.Kind {
	case "chat":
		return h.runChat(ctx, live, binding, req)
	case "image":
		return h.runImage(ctx, live, binding, req)
	case "audio":
		return h.runAudio(ctx, live, binding, req)
	case "transcribe":
		return h.runTranscribe(ctx, live, binding, req)
	case "video":
		return h.runVideo(ctx, live, binding, req)
	case "database":
		return h.runDatabase(ctx, live, binding, req)
	case "storage", "media":
		return h.runStorage(ctx, live, binding, req)
	case "cli":
		return h.runCLI(ctx, live, binding)
	default:
		return fmt.Errorf("没有为 %s 注册测试适配器", binding.Kind)
	}
}

func (h *Hub) emit(live *liveRun, event RunEvent) {
	if event.ID == "" {
		event.ID = "evt-" + newAssetID()[:12]
	}
	live.mu.Lock()
	live.events = append(live.events, event)
	waiters := append([]chan RunEvent(nil), live.waiters...)
	live.mu.Unlock()
	for _, ch := range waiters {
		select {
		case ch <- event:
		default:
		}
	}
}

func (live *liveRun) Wait(ctx context.Context) TestRun {
	select {
	case <-ctx.Done():
		return live.Snapshot()
	case <-live.done:
		return live.Snapshot()
	}
}

func (live *liveRun) Snapshot() TestRun {
	live.mu.Lock()
	defer live.mu.Unlock()
	return cloneRun(live.run)
}

func (live *liveRun) Subscribe() (<-chan RunEvent, []RunEvent, func()) {
	ch := make(chan RunEvent, 32)
	live.mu.Lock()
	past := append([]RunEvent(nil), live.events...)
	live.waiters = append(live.waiters, ch)
	done := live.run.State == StateSucceeded || live.run.State == StateFailed || live.run.State == StateCancelled
	live.mu.Unlock()
	cancel := func() {
		live.mu.Lock()
		filtered := live.waiters[:0]
		for _, w := range live.waiters {
			if w != ch {
				filtered = append(filtered, w)
			}
		}
		live.waiters = filtered
		live.mu.Unlock()
		close(ch)
	}
	if done {
		return ch, past, cancel
	}
	return ch, past, cancel
}

func (live *liveRun) setStage(label string) {
	live.mu.Lock()
	live.run.StageLabel = label
	live.run.UpdatedAt = time.Now().UTC()
	live.mu.Unlock()
}

func (live *liveRun) setOutput(block OutputBlock) {
	live.mu.Lock()
	replaced := false
	for i, existing := range live.run.Outputs {
		if existing.ID == block.ID {
			live.run.Outputs[i] = block
			replaced = true
			break
		}
	}
	if !replaced {
		live.run.Outputs = append(live.run.Outputs, block)
	}
	live.run.UpdatedAt = time.Now().UTC()
	live.mu.Unlock()
}

func (live *liveRun) appendText(blockID, delta string) OutputBlock {
	live.mu.Lock()
	defer live.mu.Unlock()
	for i, existing := range live.run.Outputs {
		if existing.ID == blockID {
			live.run.Outputs[i].Text += delta
			live.run.UpdatedAt = time.Now().UTC()
			return live.run.Outputs[i]
		}
	}
	block := OutputBlock{ID: blockID, Kind: "text", Text: delta, Complete: false}
	live.run.Outputs = append(live.run.Outputs, block)
	live.run.UpdatedAt = time.Now().UTC()
	return block
}

func cloneRun(run TestRun) TestRun {
	out := run
	out.Outputs = append([]OutputBlock{}, run.Outputs...)
	if run.Metrics != nil {
		copied := *run.Metrics
		out.Metrics = &copied
	}
	if run.Error != nil {
		copied := *run.Error
		out.Error = &copied
	}
	return out
}

type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e APIError) Error() string { return e.Message }

func asRunError(requestID string, err error) *RunError {
	if api, ok := err.(APIError); ok {
		return &RunError{Code: api.Code, SafeMessage: Sanitize(api.Message), RequestID: requestID}
	}
	return &RunError{Code: "UPSTREAM_ERROR", SafeMessage: Sanitize(err.Error()), RequestID: requestID}
}

func stringsTrim(v string) string { return trim(v) }

func trim(v string) string {
	for len(v) > 0 && (v[0] == ' ' || v[0] == '\n' || v[0] == '\t' || v[0] == '\r') {
		v = v[1:]
	}
	for len(v) > 0 && (v[len(v)-1] == ' ' || v[len(v)-1] == '\n' || v[len(v)-1] == '\t' || v[len(v)-1] == '\r') {
		v = v[:len(v)-1]
	}
	return v
}

func newRunID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func validateInput(binding Binding, req CreateTestRequest) error {
	kind := binding.Kind
	in := req.Input
	if in.Kind != "" && in.Kind != kind && !(kind == "media" && in.Kind == "storage") && !(kind == "storage" && in.Kind == "media") && !(kind == "cli" && in.Kind == "database") {
		if in.Kind != kind {
			// allow omitting strict match for cli using diagnostic
		}
	}
	switch kind {
	case "chat":
		text := chatText(in)
		if text == "" {
			return APIError{Status: 400, Code: "INVALID_INPUT", Message: "请输入本次对话内容"}
		}
		if len(text) > 8000 {
			return APIError{Status: 400, Code: "INVALID_INPUT", Message: "输入过长"}
		}
	case "image", "video":
		if trim(in.Prompt) == "" {
			return APIError{Status: 400, Code: "INVALID_INPUT", Message: "请输入提示文本"}
		}
		if len(in.Prompt) > 4000 {
			return APIError{Status: 400, Code: "INVALID_INPUT", Message: "提示文本过长"}
		}
	case "audio":
		if trim(in.Text) == "" && trim(in.Prompt) == "" {
			return APIError{Status: 400, Code: "INVALID_INPUT", Message: "请输入要朗读的文本"}
		}
	case "transcribe":
		if trim(in.AudioBase64) == "" {
			return APIError{Status: 400, Code: "INVALID_INPUT", Message: "请上传要识别的音频文件"}
		}
		if len(in.AudioBase64) > 12<<20 {
			return APIError{Status: 400, Code: "INVALID_INPUT", Message: "音频文件过大"}
		}
	case "database", "cli":
		return nil
	case "storage", "media":
		if len(in.ObjectKey) > 512 || len(in.Prefix) > 512 {
			return APIError{Status: 400, Code: "INVALID_INPUT", Message: "对象键过长"}
		}
	}
	return nil
}

func chatText(in TestInput) string {
	if len(in.Messages) > 0 {
		return trim(in.Messages[len(in.Messages)-1].Content)
	}
	if trim(in.Prompt) != "" {
		return trim(in.Prompt)
	}
	return trim(in.Text)
}

func presetOf(req CreateTestRequest) OutputPreset {
	if req.OutputPreset != nil {
		return *req.OutputPreset
	}
	return OutputPreset{Delivery: DeliveryComplete, Format: FormatText}
}
