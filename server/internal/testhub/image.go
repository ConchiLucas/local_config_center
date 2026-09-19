package testhub

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func (h *Hub) runImage(ctx context.Context, live *liveRun, binding Binding, req CreateTestRequest) error {
	provider := binding.Provider
	if provider == nil {
		return fmt.Errorf("缺少图片模型配置")
	}
	if strings.TrimSpace(provider.APIKey) == "" {
		return APIError{Status: 400, Code: "MISSING_CREDENTIAL", Message: "该配置未登记 API Key，无法调用"}
	}
	base := rewriteURL(h.rewriter, provider.BaseURL)
	if err := hostAllowed(base); err != nil {
		return err
	}
	live.setStage("正在生成图片")
	h.emit(live, RunEvent{Type: "status", RunID: live.run.ID, State: StateRunning, StageLabel: "正在生成图片"})
	model := strings.TrimSpace(provider.Model)
	body := map[string]any{
		"model":  model,
		"prompt": strings.TrimSpace(req.Input.Prompt),
		"n":      1,
	}
	if size := strings.TrimSpace(provider.Options["size"]); size != "" {
		body["size"] = size
	}
	if !strings.HasPrefix(model, "gpt-image") {
		if format := strings.TrimSpace(provider.Options["responseFormat"]); format != "" {
			body["response_format"] = format
		} else {
			body["response_format"] = "b64_json"
		}
		if quality := strings.TrimSpace(provider.Options["quality"]); quality != "" {
			body["quality"] = quality
		}
	}
	endpoint := joinURL(base, "/images/generations")
	var parsed map[string]any
	resp, raw, err := doJSON(ctx, h.httpLong.media, "POST", endpoint, provider.APIKey, body, &parsed)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return httpStatusError(resp.StatusCode, raw)
	}
	assets, err := h.collectImages(ctx, provider.APIKey, parsed, raw)
	if err != nil {
		return err
	}
	if len(assets) == 0 {
		return fmt.Errorf("上游未返回可展示的图片数据")
	}
	block := OutputBlock{ID: "image-1", Kind: "image", Assets: assets}
	live.setOutput(block)
	h.emit(live, RunEvent{Type: "output", RunID: live.run.ID, Block: &block})
	return nil
}

func (h *Hub) collectImages(ctx context.Context, apiKey string, parsed map[string]any, raw []byte) ([]ArtifactRef, error) {
	items := imageItems(parsed)
	if len(items) == 0 && json.Valid(raw) {
		var alt struct {
			Data []map[string]any `json:"data"`
		}
		_ = json.Unmarshal(raw, &alt)
		items = alt.Data
	}
	refs := make([]ArtifactRef, 0, len(items))
	for i, item := range items {
		if b64, _ := item["b64_json"].(string); strings.TrimSpace(b64) != "" {
			data, err := base64.StdEncoding.DecodeString(b64)
			if err != nil {
				data, err = base64.RawStdEncoding.DecodeString(b64)
			}
			if err != nil {
				return nil, fmt.Errorf("图片 Base64 无法解码")
			}
			art, err := h.artifacts.Save(fmt.Sprintf("image-%d", i+1), guessMIME(fmt.Sprintf("image-%d.png", i+1), data), data)
			if err != nil {
				return nil, err
			}
			refs = append(refs, art.Ref())
			continue
		}
		url, _ := item["url"].(string)
		if strings.TrimSpace(url) == "" {
			continue
		}
		data, mime, err := h.download(ctx, url, apiKey)
		if err != nil {
			return nil, err
		}
		art, err := h.artifacts.Save(fmt.Sprintf("image-%d", i+1), mime, data)
		if err != nil {
			return nil, err
		}
		refs = append(refs, art.Ref())
	}
	return refs, nil
}

func imageItems(parsed map[string]any) []map[string]any {
	if parsed == nil {
		return nil
	}
	if data, ok := parsed["data"].([]any); ok {
		out := make([]map[string]any, 0, len(data))
		for _, item := range data {
			if m, ok := item.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	}
	return nil
}

func (h *Hub) download(ctx context.Context, rawURL, apiKey string) ([]byte, string, error) {
	if err := hostAllowed(rawURL); err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", err
	}
	if strings.TrimSpace(apiKey) != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := h.httpLong.download.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("下载媒体失败: %s", Sanitize(err.Error()))
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4000))
		return nil, "", httpStatusError(resp.StatusCode, body)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, "", err
	}
	mime := resp.Header.Get("Content-Type")
	if mime == "" {
		mime = guessMIME("download.bin", data)
	}
	if i := strings.Index(mime, ";"); i > 0 {
		mime = mime[:i]
	}
	return data, mime, nil
}
