package catalog

import (
	"strings"

	"local_config_center/internal/config"
)

// GrokKnownChatModels 包含了 Grok 后台支持的全部对话与思考大模型
var GrokKnownChatModels = []ModelOption{
	{Category: "智能路由 (推荐)", Model: "grok-chat-auto", Label: "grok-chat-auto", Note: "根据任务复杂度自动分流"},
	{Category: "主力思考与推理 (Reasoning)", Model: "grok-4.20-0309-reasoning", Label: "grok-4.20-0309-reasoning", Note: "主力思维链深度推理"},
	{Category: "主力思考与推理 (Reasoning)", Model: "grok-4.5", Label: "grok-4.5", Note: "主力长文本大模型"},
	{Category: "主力思考与推理 (Reasoning)", Model: "grok-4.3", Label: "grok-4.3", Note: "高稳定性综合推理"},
	{Category: "主力思考与推理 (Reasoning)", Model: "grok-4.6", Label: "grok-4.6", Note: "新一代预览推理模型"},
	{Category: "极速响应与常规对话", Model: "grok-4.20-0309-non-reasoning", Label: "grok-4.20-0309-non-reasoning", Note: "无思维链极速响应"},
	{Category: "极速响应与常规对话", Model: "grok-chat-fast", Label: "grok-chat-fast", Note: "轻量高吞吐快速对话"},
	{Category: "多智能体与专家 (Multi-Agent)", Model: "grok-4.20-multi-agent-0309", Label: "grok-4.20-multi-agent-0309", Note: "多智能体协同协作"},
	{Category: "多智能体与专家 (Multi-Agent)", Model: "grok-chat-expert", Label: "grok-chat-expert", Note: "专家级深度领域求解"},
	{Category: "多智能体与专家 (Multi-Agent)", Model: "grok-chat-heavy", Label: "grok-chat-heavy", Note: "重任务高算力模型"},
	{Category: "代码与编程 Agent (Build 系列)", Model: "grok-build-0.1", Label: "grok-build-0.1", Note: "代码编写与 Agent 工具调用"},
	{Category: "代码与编程 Agent (Build 系列)", Model: "grok-composer-2.5-fast", Label: "grok-composer-2.5-fast", Note: "极速代码补全与生成"},
	{Category: "经典系列 (Grok 3 / 2)", Model: "grok-3", Label: "grok-3", Note: "经典 Grok 3 旗舰"},
	{Category: "经典系列 (Grok 3 / 2)", Model: "grok-3-mini", Label: "grok-3-mini", Note: "经典轻量级 Grok 3"},
	{Category: "经典系列 (Grok 3 / 2)", Model: "grok-2", Label: "grok-2", Note: "经典 Grok 2"},
	{Category: "经典系列 (Grok 3 / 2)", Model: "grok-2-mini", Label: "grok-2-mini", Note: "经典轻量级 Grok 2"},
}

// GrokVideoModels 包含了 Grok 后台支持的全部视频生成模型
var GrokVideoModels = []ModelOption{
	{Category: "视频生成 (Imagine Video)", Model: "grok-imagine-video-1.5", Label: "grok-imagine-video-1.5", Note: "新一代 Imagine 视频生成 (推荐)"},
	{Category: "视频生成 (Imagine Video)", Model: "grok-imagine-video", Label: "grok-imagine-video", Note: "标准 Imagine 视频生成"},
}

// GrokTTSModels 包含了 Grok 后台支持的全部文本转语音模型
var GrokTTSModels = []ModelOption{
	{Category: "Grok 原生语音 (Sub2API)", Model: "grok-voice-latest", Label: "grok-voice-latest", Note: "最新高拟真语音合成 (默认)"},
	{Category: "Grok 原生语音 (Sub2API)", Model: "grok-voice-think-fast-2.0", Label: "grok-voice-think-fast-2.0", Note: "第二代超低延迟思考语音"},
	{Category: "Grok 原生语音 (Sub2API)", Model: "grok-voice-think-fast-1.0", Label: "grok-voice-think-fast-1.0", Note: "第一代思考语音"},
	{Category: "OpenAI 兼容语音", Model: "tts-1", Label: "tts-1", Note: "OpenAI 兼容基础语音"},
	{Category: "OpenAI 兼容语音", Model: "tts-1-hd", Label: "tts-1-hd", Note: "OpenAI 兼容高清语音"},
}

// GrokImageModels 包含了 Grok 后台支持的全部图片生成与编辑模型
var GrokImageModels = []ModelOption{
	{Category: "AI 生图 (Imagine Image)", Model: "grok-imagine-image", Label: "grok-imagine-image", Note: "标准生图 (默认)"},
	{Category: "AI 生图 (Imagine Image)", Model: "grok-imagine-image-quality", Label: "grok-imagine-image-quality", Note: "高品质画质生图"},
	{Category: "AI 生图 (Imagine Image)", Model: "grok-imagine-image-2.0", Label: "grok-imagine-image-2.0", Note: "新一代 Imagine 2.0 生图"},
	{Category: "AI 生图 (Imagine Image)", Model: "grok-imagine-image-lite", Label: "grok-imagine-image-lite", Note: "轻量快速生图"},
	{Category: "AI 生图 (Imagine Image)", Model: "grok-imagine-image-quality-lite", Label: "grok-imagine-image-quality-lite", Note: "轻量高质生图"},
	{Category: "AI 生图 (Imagine Image)", Model: "grok-imagine-image-edit", Label: "grok-imagine-image-edit", Note: "图片局部修改与重绘"},
}

func isGrokFamily(family Family, providers []config.AIProvider) bool {
	if family.ID == "grok" {
		return true
	}
	for _, provider := range providers {
		blob := strings.ToLower(provider.ID + " " + provider.Label + " " + provider.Model)
		if strings.Contains(blob, "grok") {
			return true
		}
	}
	return false
}

func appendGrokEndpointClusters(in []*aiCluster) []*aiCluster {
	var grokProviders []config.AIProvider
	var grokFamily Family
	foundGrok := false

	for _, item := range in {
		if isGrokFamily(item.family, item.providers) {
			foundGrok = true
			grokFamily = item.family
			for _, p := range item.providers {
				already := false
				for _, ep := range grokProviders {
					if ep.ID == p.ID {
						already = true
						break
					}
				}
				if !already {
					grokProviders = append(grokProviders, p)
				}
			}
		}
	}

	if !foundGrok || len(grokProviders) == 0 {
		return in
	}

	out := make([]*aiCluster, 0, len(in)+4)
	inserted := false
	for _, item := range in {
		if isGrokFamily(item.family, item.providers) {
			if !inserted {
				// 按用户要求顺序排列四个子菜单：生成视频、tts、生成图片、chat大模型
				out = append(out,
					&aiCluster{family: grokFamily, kind: "video", providers: grokProviders},
					&aiCluster{family: grokFamily, kind: "audio", providers: grokProviders},
					&aiCluster{family: grokFamily, kind: "image", providers: grokProviders},
					&aiCluster{family: grokFamily, kind: "chat", providers: grokProviders},
				)
				inserted = true
			}
			continue
		}
		out = append(out, item)
	}

	if !inserted {
		out = append(out,
			&aiCluster{family: grokFamily, kind: "video", providers: grokProviders},
			&aiCluster{family: grokFamily, kind: "audio", providers: grokProviders},
			&aiCluster{family: grokFamily, kind: "image", providers: grokProviders},
			&aiCluster{family: grokFamily, kind: "chat", providers: grokProviders},
		)
	}

	return out
}

func applyGrokEndpointDefaults(cfg PublicConfig, kind string) PublicConfig {
	switch kind {
	case "video":
		cfg.Title = "Grok · 生成视频"
		cfg.Model = "grok-imagine-video-1.5"
		cfg.PageID = "video"
		cfg.Models = mergeKnownGrokModels("video", GrokVideoModels, cfg.Models)
		cfg.Capabilities.Delivery = []string{"complete"}
		cfg.Capabilities.Formats = []string{"video"}
		cfg.Capabilities.ReferenceImage = true
		cfg.PublicProperties = withoutLabels(cfg.PublicProperties, "默认尺寸", "质量", "返回格式", "默认音色")
		cfg.PublicProperties = append([]Property{
			{Label: "接口路径", Value: "/v1/videos/generations"},
			{Label: "任务方式", Value: "异步生成后在本页播放"},
			{Label: "默认时长", Value: "5 秒"},
		}, cfg.PublicProperties...)
	case "audio":
		cfg.Title = "Grok · TTS"
		cfg.Model = "grok-voice-latest"
		cfg.PageID = "audio"
		cfg.Models = mergeKnownGrokModels("audio", GrokTTSModels, cfg.Models)
		cfg.Capabilities.Delivery = []string{"complete"}
		cfg.Capabilities.Formats = []string{"audio"}
		cfg.PublicProperties = withoutLabels(cfg.PublicProperties, "默认尺寸", "质量", "返回格式", "默认音色", "接口地址", "接口路径", "接口协议")
		cfg.PublicProperties = append([]Property{
			{Label: "接口路径", Value: "/v1/tts"},
			{Label: "默认音色", Value: "Ara"},
			{Label: "接口协议", Value: "OpenAI 兼容协议 (Sub2API)"},
			{Label: "接口地址", Value: "http://127.0.0.1:8090/v1"},
		}, cfg.PublicProperties...)
	case "image":
		cfg.Title = "Grok · 生成图片"
		cfg.Model = "grok-imagine-image"
		cfg.PageID = "image"
		cfg.Models = mergeKnownGrokModels("image", GrokImageModels, cfg.Models)
		cfg.Capabilities.Delivery = []string{"complete"}
		cfg.Capabilities.Formats = []string{"image"}
		cfg.Capabilities.ReferenceImage = true
		cfg.PublicProperties = withoutLabels(cfg.PublicProperties, "默认音色")
		cfg.PublicProperties = append([]Property{
			{Label: "接口路径", Value: "/v1/images/generations"},
		}, cfg.PublicProperties...)
	case "chat":
		cfg.Title = "Grok · Chat 大模型"
		cfg.Model = "grok-4.20-0309-reasoning"
		cfg.PageID = "chat"
		cfg.Models = mergeKnownGrokModels("chat", GrokKnownChatModels, cfg.Models)
		cfg.Capabilities.Delivery = []string{"stream", "complete"}
		cfg.Capabilities.Formats = []string{"text", "json"}
		cfg.PublicProperties = withoutLabels(cfg.PublicProperties, "默认尺寸", "质量", "返回格式", "默认音色")
		cfg.PublicProperties = append([]Property{
			{Label: "接口路径", Value: "/v1/chat/completions"},
		}, cfg.PublicProperties...)
	}
	return cfg
}

func isModelMatchingKind(model, kind string) bool {
	m := strings.ToLower(model)
	switch kind {
	case "video":
		return strings.Contains(m, "video")
	case "audio":
		return strings.Contains(m, "voice") || strings.Contains(m, "tts") || strings.Contains(m, "audio")
	case "image":
		return strings.Contains(m, "image") || strings.Contains(m, "img") || strings.Contains(m, "picture")
	case "chat":
		return !strings.Contains(m, "image") && !strings.Contains(m, "video") && !strings.Contains(m, "voice") && !strings.Contains(m, "tts")
	default:
		return true
	}
}

func mergeKnownGrokModels(kind string, known []ModelOption, registered []ModelOption) []ModelOption {
	bySlug := map[string]ModelOption{}
	for _, item := range registered {
		if isModelMatchingKind(item.Model, kind) {
			bySlug[item.Model] = item
		}
	}
	out := make([]ModelOption, 0, len(known)+len(registered))
	seen := map[string]bool{}
	for _, item := range known {
		candidate := item
		if existing, ok := bySlug[item.Model]; ok {
			candidate.ProviderID = existing.ProviderID
			candidate.Registered = true
			if existing.Label != "" && existing.Label != existing.Model {
				candidate.Label = existing.Label
			}
		}
		out = append(out, candidate)
		seen[item.Model] = true
	}
	for _, item := range registered {
		if !isModelMatchingKind(item.Model, kind) || seen[item.Model] {
			continue
		}
		item.Category = "配置中心已登记"
		item.Registered = true
		out = append(out, item)
		seen[item.Model] = true
	}
	return out
}
