package testhub

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

func (h *Hub) runChat(ctx context.Context, live *liveRun, binding Binding, req CreateTestRequest) error {
	provider := binding.Provider
	if provider == nil {
		return fmt.Errorf("缺少对话模型配置")
	}
	if strings.TrimSpace(provider.APIKey) == "" {
		return APIError{Status: 400, Code: "MISSING_CREDENTIAL", Message: "该配置未登记 API Key，无法调用"}
	}
	base := rewriteURL(h.rewriter, provider.BaseURL)
	if err := hostAllowed(base); err != nil {
		return err
	}
	preset := presetOf(req)
	messages := req.Input.Messages
	if len(messages) == 0 {
		messages = []ChatMessage{{Role: "user", Content: chatText(req.Input)}}
	}
	if preset.Format == FormatJSON {
		messages = append([]ChatMessage{{
			Role:    "system",
			Content: "Return a single JSON object that answers the user. Do not wrap it in markdown.",
		}}, messages...)
	}
	body := map[string]any{
		"model":    provider.Model,
		"messages": messages,
		"stream":   preset.Delivery == DeliveryStream && preset.Format != FormatJSON,
	}
	if preset.Format == FormatJSON {
		body["response_format"] = map[string]string{"type": "json_object"}
		body["stream"] = false
	}
	endpoint := joinURL(base, "/chat/completions")
	if stream, _ := body["stream"].(bool); stream {
		return h.streamChat(ctx, live, endpoint, provider.APIKey, body)
	}
	return h.completeChat(ctx, live, endpoint, provider.APIKey, body, preset.Format == FormatJSON)
}

func (h *Hub) completeChat(ctx context.Context, live *liveRun, endpoint, apiKey string, body map[string]any, asJSON bool) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	live.setStage("等待完整返回")
	h.emit(live, RunEvent{Type: "status", RunID: live.run.ID, State: StateRunning, StageLabel: "等待完整返回"})
	var parsed map[string]any
	resp, raw, err := doJSON(ctx, h.httpLong.chat, "POST", endpoint, apiKey, body, &parsed)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return httpStatusError(resp.StatusCode, raw)
	}
	text := extractChatText(parsed, raw)
	if asJSON {
		block := jsonBlock("json-1", text)
		live.setOutput(block)
		h.emit(live, RunEvent{Type: "output", RunID: live.run.ID, Block: &block})
		return nil
	}
	block := OutputBlock{ID: "text-1", Kind: "text", Text: text, Complete: true}
	live.setOutput(block)
	h.emit(live, RunEvent{Type: "output", RunID: live.run.ID, Block: &block})
	return nil
}

func (h *Hub) streamChat(ctx context.Context, live *liveRun, endpoint, apiKey string, body map[string]any) error {
	live.setStage("流式输出中")
	h.emit(live, RunEvent{Type: "status", RunID: live.run.ID, State: StateRunning, StageLabel: "流式输出中"})
	resp, err := doStream(ctx, h.httpLong.chat, endpoint, apiKey, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return httpStatusError(resp.StatusCode, raw)
	}
	blockID := "text-1"
	started := time.Now()
	first := true
	reader := bufio.NewReader(resp.Body)
	var carry string
	for {
		if ctx.Err() != nil {
			block := live.appendText(blockID, "")
			block.Complete = false
			live.setOutput(block)
			return ctx.Err()
		}
		line, err := reader.ReadString('\n')
		carry += line
		if err != nil && err != io.EOF && !strings.Contains(err.Error(), "EOF") {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("读取流式响应失败: %s", Sanitize(err.Error()))
		}
		for {
			idx := strings.Index(carry, "\n")
			if idx < 0 {
				break
			}
			piece := strings.TrimSpace(carry[:idx])
			carry = carry[idx+1:]
			if piece == "" || strings.HasPrefix(piece, ":") {
				continue
			}
			if !strings.HasPrefix(piece, "data:") {
				continue
			}
			payload := strings.TrimSpace(strings.TrimPrefix(piece, "data:"))
			if payload == "[DONE]" {
				block := live.appendText(blockID, "")
				block.Complete = true
				live.setOutput(block)
				h.emit(live, RunEvent{Type: "output", RunID: live.run.ID, Block: &block})
				return nil
			}
			delta := extractDelta(payload)
			if delta == "" {
				continue
			}
			if first {
				ms := time.Since(started).Milliseconds()
				live.mu.Lock()
				if live.run.Metrics == nil {
					live.run.Metrics = &Metrics{}
				}
				live.run.Metrics.FirstTokenMs = ms
				live.mu.Unlock()
				first = false
			}
			live.appendText(blockID, delta)
			h.emit(live, RunEvent{Type: "text_delta", RunID: live.run.ID, BlockID: blockID, Delta: delta})
		}
		if err == io.EOF {
			block := live.appendText(blockID, "")
			block.Complete = true
			live.setOutput(block)
			h.emit(live, RunEvent{Type: "output", RunID: live.run.ID, Block: &block})
			if block.Text == "" {
				return fmt.Errorf("流式响应结束，但没有收到任何文本")
			}
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func extractChatText(parsed map[string]any, raw []byte) string {
	if parsed != nil {
		if choices, ok := parsed["choices"].([]any); ok && len(choices) > 0 {
			if choice, ok := choices[0].(map[string]any); ok {
				if msg, ok := choice["message"].(map[string]any); ok {
					if content, ok := msg["content"].(string); ok {
						return content
					}
				}
				if text, ok := choice["text"].(string); ok {
					return text
				}
			}
		}
		if content, ok := parsed["content"].(string); ok {
			return content
		}
	}
	return strings.TrimSpace(string(raw))
}

func extractDelta(payload string) string {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		return ""
	}
	choices, _ := parsed["choices"].([]any)
	if len(choices) == 0 {
		return ""
	}
	choice, _ := choices[0].(map[string]any)
	delta, _ := choice["delta"].(map[string]any)
	if content, ok := delta["content"].(string); ok {
		return content
	}
	if msg, ok := choice["message"].(map[string]any); ok {
		if content, ok := msg["content"].(string); ok {
			return content
		}
	}
	return ""
}

func jsonBlock(id, text string) OutputBlock {
	text = strings.TrimSpace(text)
	text = unwrapMarkdownJSON(text)
	block := OutputBlock{ID: id, Kind: "json", RawText: text}
	valid := json.Valid([]byte(text))
	block.ValidJSON = &valid
	if valid {
		block.Parsed = json.RawMessage(text)
	}
	return block
}

func unwrapMarkdownJSON(text string) string {
	trimmed := strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, "```") {
		trimmed = strings.TrimPrefix(trimmed, "```json")
		trimmed = strings.TrimPrefix(trimmed, "```JSON")
		trimmed = strings.TrimPrefix(trimmed, "```")
		if i := strings.LastIndex(trimmed, "```"); i >= 0 {
			trimmed = trimmed[:i]
		}
		return strings.TrimSpace(trimmed)
	}
	return trimmed
}
