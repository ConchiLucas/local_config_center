package testhub

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
)

func (h *Hub) runTranscribe(ctx context.Context, live *liveRun, binding Binding, req CreateTestRequest) error {
	provider := binding.Provider
	if provider == nil {
		return fmt.Errorf("缺少语音识别配置")
	}
	if strings.TrimSpace(provider.APIKey) == "" {
		return APIError{Status: 400, Code: "MISSING_CREDENTIAL", Message: "该配置未登记 API Key，无法调用"}
	}
	raw := strings.TrimSpace(req.Input.AudioBase64)
	if i := strings.Index(raw, "base64,"); i >= 0 {
		raw = raw[i+7:]
	}
	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		data, err = base64.RawStdEncoding.DecodeString(raw)
	}
	if err != nil || len(data) == 0 {
		return APIError{Status: 400, Code: "INVALID_INPUT", Message: "无法解码上传的音频"}
	}
	if len(data) > 10<<20 {
		return APIError{Status: 400, Code: "INVALID_INPUT", Message: "音频文件过大"}
	}
	filename := strings.TrimSpace(req.Input.Filename)
	if filename == "" {
		filename = "audio.wav"
	}
	filename = filepath.Base(filename)
	base := rewriteURL(h.rewriter, provider.BaseURL)
	if err := hostAllowed(base); err != nil {
		return err
	}
	live.setStage("正在识别语音")
	h.emit(live, RunEvent{Type: "status", RunID: live.run.ID, State: StateRunning, StageLabel: "正在识别语音"})

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return err
	}
	if _, err := part.Write(data); err != nil {
		return err
	}
	if err := writer.WriteField("model", provider.Model); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, joinURL(base, "/audio/transcriptions"), &buf)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	httpReq.Header.Set("Authorization", "Bearer "+provider.APIKey)
	resp, err := h.httpLong.media.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %s", Sanitize(err.Error()))
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return httpStatusError(resp.StatusCode, body)
	}
	text := extractTranscription(body)
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("上游没有返回转写文本")
	}
	block := OutputBlock{ID: "text-1", Kind: "text", Text: text, Complete: true, Transcript: filename}
	live.setOutput(block)
	h.emit(live, RunEvent{Type: "output", RunID: live.run.ID, Block: &block})
	return nil
}

func extractTranscription(raw []byte) string {
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return strings.TrimSpace(string(raw))
	}
	if text, ok := parsed["text"].(string); ok {
		return text
	}
	return ""
}
