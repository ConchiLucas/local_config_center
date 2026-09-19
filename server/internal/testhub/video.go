package testhub

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

func (h *Hub) runVideo(ctx context.Context, live *liveRun, binding Binding, req CreateTestRequest) error {
	provider := binding.Provider
	if provider == nil {
		return fmt.Errorf("缺少视频模型配置")
	}
	if strings.TrimSpace(provider.APIKey) == "" {
		return APIError{Status: 400, Code: "MISSING_CREDENTIAL", Message: "该配置未登记 API Key，无法调用"}
	}
	base := rewriteURL(h.rewriter, provider.BaseURL)
	if err := hostAllowed(base); err != nil {
		return err
	}
	live.setStage("正在创建视频任务")
	h.emit(live, RunEvent{Type: "status", RunID: live.run.ID, State: StateRunning, StageLabel: "正在创建视频任务"})
	body := map[string]any{
		"model":  provider.Model,
		"prompt": strings.TrimSpace(req.Input.Prompt),
	}
	if duration := strings.TrimSpace(provider.Options["duration"]); duration != "" {
		body["duration"] = duration
	}
	endpoints := []string{
		joinURL(base, "/videos/generations"),
		joinURL(base, "/video/generations"),
		joinURL(base, "/videos"),
	}
	var created map[string]any
	var raw []byte
	var lastErr error
	for _, endpoint := range endpoints {
		resp, data, err := doJSON(ctx, h.httpLong.media, "POST", endpoint, provider.APIKey, body, &created)
		if err != nil {
			lastErr = err
			continue
		}
		raw = data
		if resp.StatusCode >= 300 {
			lastErr = httpStatusError(resp.StatusCode, data)
			if resp.StatusCode == 404 || resp.StatusCode == 405 {
				continue
			}
			return lastErr
		}
		lastErr = nil
		break
	}
	if lastErr != nil {
		return lastErr
	}
	if videoURL, mime := immediateVideo(created, raw); videoURL != "" {
		return h.finishVideo(ctx, live, provider.APIKey, base, videoURL, mime)
	}
	requestID := videoRequestID(created)
	if requestID == "" {
		return fmt.Errorf("视频任务已受理，但响应里没有可轮询的任务编号，也没有可播放地址")
	}
	pollURL := joinURL(base, "/videos/"+requestID)
	deadline := time.Now().Add(8 * time.Minute)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		live.setStage("视频生成中")
		h.emit(live, RunEvent{Type: "status", RunID: live.run.ID, State: StateRunning, StageLabel: "视频生成中"})
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
		var status map[string]any
		resp, data, err := doJSON(ctx, h.httpLong.media, "GET", pollURL, provider.APIKey, nil, &status)
		if err != nil {
			return err
		}
		if resp.StatusCode >= 300 {
			return httpStatusError(resp.StatusCode, data)
		}
		state := strings.ToLower(fmt.Sprint(status["status"], status["state"]))
		if strings.Contains(state, "fail") || strings.Contains(state, "error") || strings.Contains(state, "expired") {
			return fmt.Errorf("视频任务失败：%s", Sanitize(truncateBody(data)))
		}
		if url, mime := immediateVideo(status, data); url != "" && (strings.Contains(state, "done") || strings.Contains(state, "succeed") || strings.Contains(state, "complete") || mime != "") {
			live.setStage("正在拉取视频")
			return h.finishVideo(ctx, live, provider.APIKey, base, url, mime)
		}
	}
	return fmt.Errorf("视频任务超时，未在限定时间内返回可播放文件")
}

func (h *Hub) finishVideo(ctx context.Context, live *liveRun, apiKey, base, videoURL, mime string) error {
	resolvedURL := resolveMediaURL(videoURL, base)
	data, detected, err := h.download(ctx, resolvedURL, apiKey)
	if err != nil {
		return fmt.Errorf("视频已生成，但拉取播放文件失败：%s", Sanitize(err.Error()))
	}
	if mime == "" {
		mime = detected
	}
	if !strings.HasPrefix(mime, "video/") {
		if strings.HasPrefix(detected, "video/") {
			mime = detected
		} else {
			mime = "video/mp4"
		}
	}
	art, err := h.artifacts.Save("video", mime, data)
	if err != nil {
		return err
	}
	ref := art.Ref()
	block := OutputBlock{ID: "video-1", Kind: "video", Asset: &ref}
	live.setOutput(block)
	h.emit(live, RunEvent{Type: "output", RunID: live.run.ID, Block: &block})
	return nil
}

func videoRequestID(parsed map[string]any) string {
	if parsed == nil {
		return ""
	}
	for _, key := range []string{"request_id", "id", "task_id", "job_id"} {
		if v, ok := parsed[key].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func immediateVideo(parsed map[string]any, raw []byte) (string, string) {
	if parsed == nil && json.Valid(raw) {
		_ = json.Unmarshal(raw, &parsed)
	}
	if parsed == nil {
		return "", ""
	}
	if url, _ := parsed["url"].(string); strings.HasPrefix(url, "http") {
		return url, ""
	}
	if video, ok := parsed["video"].(map[string]any); ok {
		if url, _ := video["url"].(string); strings.HasPrefix(url, "http") {
			return url, ""
		}
	}
	if data, ok := parsed["data"].([]any); ok && len(data) > 0 {
		if item, ok := data[0].(map[string]any); ok {
			if url, _ := item["url"].(string); strings.HasPrefix(url, "http") {
				return url, ""
			}
		}
	}
	return "", ""
}

func resolveMediaURL(rawURL, baseURL string) string {
	raw := strings.TrimSpace(rawURL)
	if raw == "" {
		return raw
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return joinURL(baseURL, raw)
	}
	baseParsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err == nil && baseParsed.Host != "" {
		// 如果返回的是本地地址但端口不一致（例如反代容器内部 8000 端口，而宿主机映射的是 37821），自动对齐到配置的 base 端口
		if (parsed.Hostname() == "127.0.0.1" || parsed.Hostname() == "localhost") &&
			(baseParsed.Hostname() == "127.0.0.1" || baseParsed.Hostname() == "localhost") &&
			parsed.Port() != baseParsed.Port() {
			parsed.Host = baseParsed.Host
			parsed.Scheme = baseParsed.Scheme
			return parsed.String()
		}
	}
	return raw
}
