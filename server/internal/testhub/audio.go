package testhub

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode"

	"local_config_center/internal/config"
)

func (h *Hub) runAudio(ctx context.Context, live *liveRun, binding Binding, req CreateTestRequest) error {
	provider := binding.Provider
	if provider == nil {
		return fmt.Errorf("缺少语音模型配置")
	}
	if strings.TrimSpace(provider.APIKey) == "" {
		return APIError{Status: 400, Code: "MISSING_CREDENTIAL", Message: "该配置未登记 API Key，无法调用"}
	}
	text := strings.TrimSpace(req.Input.Text)
	if text == "" {
		text = strings.TrimSpace(req.Input.Prompt)
	}
	base := rewriteURL(h.rewriter, provider.BaseURL)
	if err := hostAllowed(base); err != nil {
		return err
	}
	live.setStage("正在合成语音")
	h.emit(live, RunEvent{Type: "status", RunID: live.run.ID, State: StateRunning, StageLabel: "正在合成语音"})

	voice := strings.TrimSpace(req.Input.Voice)
	if voice == "" {
		voice = strings.TrimSpace(provider.Voice)
	}
	if voice == "" {
		voice = strings.TrimSpace(provider.Options["voice"])
	}

	if strings.EqualFold(strings.TrimSpace(provider.Type), "mimo-tts") {
		return h.runMimoTTS(ctx, live, provider, base, text, voice)
	}
	if binding.Public.ID == "ai:grok:audio" || strings.EqualFold(strings.TrimSpace(provider.Type), "sub2api-tts") {
		return h.runSub2APITTS(ctx, live, provider, base, text, voice)
	}

	body := map[string]any{
		"model": provider.Model,
		"input": text,
	}
	if voice != "" {
		body["voice"] = voice
	} else {
		body["voice"] = "alloy"
	}
	if format := strings.TrimSpace(provider.Options["responseFormat"]); format != "" {
		body["response_format"] = format
	}

	endpoints := []string{joinURL(base, "/audio/speech")}

	var lastErr error
	for _, endpoint := range unique(endpoints) {
		resp, raw, err := doJSON(ctx, h.httpLong.media, "POST", endpoint, provider.APIKey, body, nil)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode >= 300 {
			lastErr = httpStatusError(resp.StatusCode, raw)
			if resp.StatusCode == 404 || resp.StatusCode == 405 {
				continue
			}
			return lastErr
		}
		mime := resp.Header.Get("Content-Type")
		if i := strings.Index(mime, ";"); i > 0 {
			mime = mime[:i]
		}
		audio := raw
		if strings.Contains(mime, "json") || json.Valid(raw) {
			extracted, extractedMIME, ok := extractAudioJSON(raw)
			if !ok {
				lastErr = fmt.Errorf("语音接口返回了 JSON，但没有可播放的音频字段")
				continue
			}
			audio = extracted
			if extractedMIME != "" {
				mime = extractedMIME
			}
		}
		if mime == "" || strings.Contains(mime, "json") || mime == "application/octet-stream" {
			mime = guessMIME("speech.wav", audio)
		}
		if !strings.HasPrefix(mime, "audio/") && mime != "application/octet-stream" {
			lastErr = fmt.Errorf("上游返回的不是音频（%s）", mime)
			continue
		}
		if mime == "application/octet-stream" {
			mime = "audio/wav"
		}
		art, err := h.artifacts.Save("speech", mime, audio)
		if err != nil {
			return err
		}
		ref := art.Ref()
		block := OutputBlock{ID: "audio-1", Kind: "audio", Asset: &ref, Transcript: text}
		live.setOutput(block)
		h.emit(live, RunEvent{Type: "output", RunID: live.run.ID, Block: &block})
		return nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("语音接口没有返回可播放音频")
	}
	return lastErr
}

func (h *Hub) runMimoTTS(ctx context.Context, live *liveRun, provider *config.AIProvider, base, text, voice string) error {
	if voice == "" || strings.EqualFold(voice, "alloy") {
		voice = "冰糖"
	}
	models := []string{provider.Model}
	if provider.Model == "mimo-2.5-tts" || provider.Model == "mimo-v2-tts" {
		models = append([]string{"mimo-v2.5-tts"}, models...)
	}
	endpoint := joinURL(base, "/chat/completions")
	var lastErr error
	for _, model := range unique(models) {
		body := map[string]any{
			"model": model,
			"messages": []map[string]string{
				{"role": "assistant", "content": text},
			},
			"audio": map[string]string{
				"format": "wav",
				"voice":  voice,
			},
		}
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+provider.APIKey)
		req.Header.Set("api-key", provider.APIKey)
		resp, err := h.httpLong.media.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %s", Sanitize(err.Error()))
			continue
		}
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
		resp.Body.Close()
		if resp.StatusCode >= 300 {
			lastErr = httpStatusError(resp.StatusCode, data)
			continue
		}
		audio, mime, ok := extractAudioJSON(data)
		if !ok {
			lastErr = fmt.Errorf("MiMo TTS 响应里没有可播放的音频字段")
			continue
		}
		if mime == "" {
			mime = "audio/wav"
		}
		art, err := h.artifacts.Save("speech", mime, audio)
		if err != nil {
			return err
		}
		ref := art.Ref()
		block := OutputBlock{ID: "audio-1", Kind: "audio", Asset: &ref, Transcript: text}
		live.setOutput(block)
		h.emit(live, RunEvent{Type: "output", RunID: live.run.ID, Block: &block})
		return nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("MiMo TTS 没有返回可播放音频")
	}
	return lastErr
}

func (h *Hub) runSub2APITTS(ctx context.Context, live *liveRun, provider *config.AIProvider, base, text, voice string) error {
	if voice == "" || strings.EqualFold(voice, "alloy") {
		voice = "Ara"
	}
	lang := "zh"
	hasHan := false
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			hasHan = true
			break
		}
	}
	if !hasHan {
		lang = "en"
	}

	endpoint := joinURL(base, "/tts")
	body := map[string]any{
		"text":     text,
		"language": lang,
		"voice_id": voice,
	}

	resp, raw, err := doJSON(ctx, h.httpLong.media, "POST", endpoint, provider.APIKey, body, nil)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return httpStatusError(resp.StatusCode, raw)
	}
	mime := resp.Header.Get("Content-Type")
	if i := strings.Index(mime, ";"); i > 0 {
		mime = mime[:i]
	}
	audio := raw
	if strings.Contains(mime, "json") || json.Valid(raw) {
		extracted, extractedMIME, ok := extractAudioJSON(raw)
		if !ok {
			return fmt.Errorf("Sub2API 语音接口返回了 JSON，但没有可播放的音频字段")
		}
		audio = extracted
		if extractedMIME != "" {
			mime = extractedMIME
		}
	}
	if mime == "" || strings.Contains(mime, "json") || mime == "application/octet-stream" {
		mime = guessMIME("speech.mp3", audio)
	}
	if !strings.HasPrefix(mime, "audio/") && mime != "application/octet-stream" {
		return fmt.Errorf("上游返回的不是音频（%s）", mime)
	}
	if mime == "application/octet-stream" {
		mime = "audio/mpeg"
	}
	art, err := h.artifacts.Save("speech", mime, audio)
	if err != nil {
		return err
	}
	ref := art.Ref()
	block := OutputBlock{ID: "audio-1", Kind: "audio", Asset: &ref, Transcript: text}
	live.setOutput(block)
	h.emit(live, RunEvent{Type: "output", RunID: live.run.ID, Block: &block})
	return nil
}

func extractAudioJSON(raw []byte) ([]byte, string, bool) {
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, "", false
	}
	for _, key := range []string{"audio", "data", "b64_json", "audio_base64", "audioContent"} {
		if value, ok := parsed[key].(string); ok && value != "" {
			if decoded, err := decodeMaybeBase64(value); err == nil && len(decoded) > 16 {
				return decoded, "", true
			}
		}
	}
	if data, ok := parsed["data"].(map[string]any); ok {
		if value, ok := data["audio"].(string); ok {
			if decoded, err := decodeMaybeBase64(value); err == nil {
				return decoded, "", true
			}
		}
	}
	if choices, ok := parsed["choices"].([]any); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]any); ok {
			if msg, ok := choice["message"].(map[string]any); ok {
				if audio, ok := msg["audio"].(map[string]any); ok {
					if value, ok := audio["data"].(string); ok {
						if decoded, err := decodeMaybeBase64(value); err == nil {
							return decoded, "audio/wav", true
						}
					}
				}
			}
		}
	}
	if url, ok := parsed["url"].(string); ok && strings.HasPrefix(url, "http") {
		return nil, "", false
	}
	return nil, "", false
}

func decodeMaybeBase64(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if i := strings.Index(value, "base64,"); i >= 0 {
		value = value[i+7:]
	}
	return decodeB64(value)
}

func decodeB64(value string) ([]byte, error) {
	if data, err := decodeStd(value); err == nil {
		return data, nil
	}
	return decodeRaw(value)
}

func decodeStd(value string) ([]byte, error) {
	return stdEncoding().DecodeString(value)
}

func decodeRaw(value string) ([]byte, error) {
	return rawEncoding().DecodeString(value)
}

func unique(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, item := range in {
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}
