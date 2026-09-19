package catalog

import (
	"strings"

	"local_config_center/internal/config"
)

// AuroraKnownChatModels is the ChatGPT web-proxy model list shown in the
// Aurora workbench dropdown. Slugs match the gateway's published catalog.
var AuroraKnownChatModels = []ModelOption{
	{Category: "智能调度 (推荐)", Model: "auto", Label: "auto", Note: "按问题复杂度自动路由"},
	{Category: "主线常规 & 极速", Model: "gpt-5-6", Label: "gpt-5-6", Note: "主力对话"},
	{Category: "主线常规 & 极速", Model: "gpt-5-6-instant", Label: "gpt-5-6-instant", Note: "极速响应"},
	{Category: "主线常规 & 极速", Model: "gpt-5-5", Label: "gpt-5-5", Note: "主力对话"},
	{Category: "主线常规 & 极速", Model: "gpt-5-5-instant", Label: "gpt-5-5-instant", Note: "极速响应"},
	{Category: "轻量级 Mini", Model: "gpt-5-6-mini", Label: "gpt-5-6-mini", Note: "轻量高吞吐"},
	{Category: "轻量级 Mini", Model: "gpt-5-5-mini", Label: "gpt-5-5-mini", Note: "轻量高吞吐"},
	{Category: "轻量级 Mini", Model: "gpt-5-3-mini", Label: "gpt-5-3-mini", Note: "轻量高吞吐"},
	{Category: "深度思考 (Thinking)", Model: "gpt-5-6-thinking", Label: "gpt-5-6-thinking", Note: "思维链推理"},
	{Category: "深度思考 (Thinking)", Model: "gpt-5-5-thinking", Label: "gpt-5-5-thinking", Note: "思维链推理"},
	{Category: "深度思考 (Thinking)", Model: "gpt-5-6-t-mini", Label: "gpt-5-6-t-mini", Note: "轻量思考"},
	{Category: "专项架构 (WM 系列)", Model: "gpt-5.6-sol-wm", Label: "gpt-5.6-sol-wm", Note: "复杂推理与代码"},
	{Category: "专项架构 (WM 系列)", Model: "gpt-5.6-terra-wm", Label: "gpt-5.6-terra-wm", Note: "高速通用生成"},
	{Category: "专项架构 (WM 系列)", Model: "gpt-5.6-luna-wm", Label: "gpt-5.6-luna-wm", Note: "高阶微调实验"},
	{Category: "专项架构 (WM 系列)", Model: "gpt-5.5-wm", Label: "gpt-5.5-wm", Note: "WM 架构"},
	{Category: "专项架构 (WM 系列)", Model: "gpt-6-astra-wm", Label: "gpt-6-astra-wm", Note: "高阶微调实验"},
	{Category: "专业旗舰 (Pro)", Model: "gpt-6-pro", Label: "gpt-6-pro", Note: "旗舰长文本"},
	{Category: "专业旗舰 (Pro)", Model: "gpt-5-6-pro", Label: "gpt-5-6-pro", Note: "旗舰长文本"},
	{Category: "专业旗舰 (Pro)", Model: "gpt-5-5-pro", Label: "gpt-5-5-pro", Note: "旗舰长文本"},
	{Category: "深度研究 (Research)", Model: "research", Label: "research", Note: "深度联网研究"},
}

var AuroraImageModels = []ModelOption{
	{Category: "AI 画图", Model: "gpt-image-2", Label: "gpt-image-2", Note: "/v1/images/generations"},
}

var AuroraTTSModels = []ModelOption{
	{Category: "文本转语音", Model: "tts-1", Label: "tts-1", Note: "/v1/audio/speech"},
	{Category: "文本转语音", Model: "tts-1-hd", Label: "tts-1-hd", Note: "更高音质"},
	{Category: "文本转语音", Model: "gpt-4o-mini-tts", Label: "gpt-4o-mini-tts", Note: "新一代 TTS"},
}

var AuroraSTTModels = []ModelOption{
	{Category: "语音识别转文字", Model: "whisper-1", Label: "whisper-1", Note: "/v1/audio/transcriptions"},
	{Category: "语音识别转文字", Model: "gpt-4o-mini-transcribe", Label: "gpt-4o-mini-transcribe", Note: "更快的听写"},
	{Category: "语音识别转文字", Model: "gpt-4o-transcribe", Label: "gpt-4o-transcribe", Note: "更高准确率"},
}

func isAuroraFamily(family Family, providers []config.AIProvider) bool {
	if family.ID == "aurora" {
		return true
	}
	return usesAuroraModelCatalog(family, "chat", providers)
}

func applyAuroraEndpointDefaults(cfg PublicConfig, kind string) PublicConfig {
	switch kind {
	case "image":
		cfg.Title = "Aurora · AI 画图"
		cfg.Model = "gpt-image-2"
		cfg.PageID = "image"
		cfg.Models = append([]ModelOption{}, AuroraImageModels...)
		cfg.PublicProperties = withoutLabels(cfg.PublicProperties, "默认尺寸", "质量", "返回格式", "默认音色")
		cfg.PublicProperties = append([]Property{{Label: "接口路径", Value: "/v1/images/generations"}}, cfg.PublicProperties...)
	case "audio":
		cfg.Title = "文本转语音"
		cfg.Model = "tts-1"
		cfg.PageID = "voice"
		cfg.Models = append([]ModelOption{}, AuroraTTSModels...)
		cfg.PublicProperties = withoutLabels(cfg.PublicProperties, "默认音色")
		cfg.PublicProperties = append([]Property{
			{Label: "接口路径", Value: "/v1/audio/speech"},
			{Label: "默认音色", Value: "alloy"},
		}, cfg.PublicProperties...)
	}
	return cfg
}

func usesAuroraModelCatalog(family Family, kind string, providers []config.AIProvider) bool {
	if kind != "chat" {
		return false
	}
	if family.ID == "aurora" {
		return true
	}
	for _, provider := range providers {
		if config.HasCapability(provider, "WEB_PROXY") {
			return true
		}
		blob := strings.ToLower(provider.ID + " " + provider.Label)
		if strings.Contains(blob, "aurora") {
			return true
		}
	}
	return false
}

func mergeKnownChatModels(family Family, kind string, providers []config.AIProvider, registered []ModelOption) []ModelOption {
	if !usesAuroraModelCatalog(family, kind, providers) {
		return registered
	}
	bySlug := map[string]ModelOption{}
	for _, item := range registered {
		bySlug[item.Model] = item
	}
	out := make([]ModelOption, 0, len(AuroraKnownChatModels)+len(registered))
	seen := map[string]bool{}
	for _, known := range AuroraKnownChatModels {
		item := known
		if existing, ok := bySlug[known.Model]; ok {
			item.ProviderID = existing.ProviderID
			item.Registered = true
			if existing.Label != "" && existing.Label != existing.Model {
				item.Label = existing.Label
			}
		}
		out = append(out, item)
		seen[item.Model] = true
	}
	for _, item := range registered {
		if seen[item.Model] {
			continue
		}
		item.Category = "配置中心已登记"
		item.Registered = true
		out = append(out, item)
		seen[item.Model] = true
	}
	return out
}
