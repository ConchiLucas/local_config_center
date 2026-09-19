package catalog

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"local_config_center/internal/config"
)

func TestProjectHidesSecretsAndSplitsMenus(t *testing.T) {
	snapshot := config.Snapshot{
		AI: config.AIConfiguration{Providers: []config.AIProvider{
			{
				ID: "antigravity-chat", Label: "Antigravity Gemini 3.1 Pro", Type: "openai-compatible",
				BaseURL: "https://gateway.example/v1", APIKey: "sk-secret-antigravity",
				Model: "gemini-3.1-pro", MaxTokens: 4096, Enabled: true,
				Capabilities: []string{"TEXT_GENERATION"}, Options: map[string]string{},
			},
			{
				ID: "antigravity-chat-2", Label: "Antigravity Gemini 3.6 Flash High", Type: "openai-compatible",
				BaseURL: "https://gateway.example/v1", APIKey: "sk-secret-antigravity",
				Model: "gemini-3.6-flash-high", MaxTokens: 4096, Enabled: true,
				Capabilities: []string{"TEXT_GENERATION"}, Options: map[string]string{},
			},
			{
				ID: "antigravity-image", Label: "Antigravity Image", Type: "openai-compatible",
				BaseURL: "https://gateway.example/v1", APIKey: "sk-secret-image",
				Model: "gemini-image", MaxTokens: 1024, Enabled: true,
				Capabilities: []string{"IMAGE_GENERATION", "IMAGE_REFERENCE"},
				Options:      map[string]string{"size": "1536x864", "quality": "hd"},
			},
			{
				ID: "sub2api-grok-image", Label: "Sub2API Grok Image", Type: "openai-compatible",
				BaseURL: "https://gateway.example/v1", APIKey: "sk-secret-grok",
				Model: "grok-imagine-image", MaxTokens: 1024, Enabled: true,
				Capabilities: []string{"IMAGE_GENERATION", "IMAGE_REFERENCE"},
				Options:      map[string]string{"size": "1536x864"},
			},
			{
				ID: "xiaomi-mimo-tts", Label: "小米 Mimo TTS", Type: "mimo-tts",
				BaseURL: "https://api.xiaomimimo.com/v1", APIKey: "mimo-secret",
				Model: "mimo-2.5-tts", MaxTokens: 1024, Enabled: true,
				Capabilities: []string{"AUDIO_TTS"}, Options: map[string]string{},
			},
		}},
		AIUpdatedAt: time.Date(2026, 9, 19, 1, 2, 3, 0, time.UTC),
		Databases: config.DatabaseConfiguration{Databases: []config.DatabaseConnection{{
			ID: "1", Name: "words", Type: "postgresql", Host: "127.0.0.1", Port: 5432,
			Database: "rob_english_word", Username: "conchi", Password: "db-secret",
			Parameters: map[string]string{},
		}}},
		DBUpdatedAt: time.Date(2026, 9, 19, 1, 2, 4, 0, time.UTC),
		ObjectStorage: config.ObjectStorageConfiguration{
			Configured: true, Enabled: true, Endpoint: "minio:9000",
			AccessKeyID: "minio-key", SecretAccessKey: "minio-secret",
			BucketName: "english-material", BasePath: "image-story",
		},
		OSUpdatedAt: time.Date(2026, 9, 19, 1, 2, 5, 0, time.UTC),
		LocalCLI: config.LocalCLIConfiguration{Configs: []config.LocalCLIConfig{{
			ID: "grok", Label: "Grok CLI", Enabled: true, Command: "/usr/bin/grok",
			TimeoutSeconds: 30, DefaultArgs: []string{"--single"}, Capabilities: []string{},
		}}},
		CLIUpdatedAt: time.Date(2026, 9, 19, 1, 2, 6, 0, time.UTC),
	}

	cat := Project(snapshot, time.Date(2026, 9, 19, 8, 0, 0, 0, time.UTC), nil)
	if len(cat.Groups) < 4 {
		t.Fatalf("expected distinct groups, got %#v", groupIDs(cat))
	}
	if pagesOf(cat, "antigravity")["video"] {
		t.Fatal("antigravity should not register a video page without VIDEO_GENERATION")
	}
	if !pagesOf(cat, "antigravity")["chat"] || !pagesOf(cat, "antigravity")["image"] {
		t.Fatalf("antigravity pages = %#v", pagesOf(cat, "antigravity"))
	}
	if !pagesOf(cat, "grok")["video"] {
		t.Fatal("grok should have video page")
	}
	if !pagesOf(cat, "grok")["audio"] {
		t.Fatal("grok should have audio page")
	}
	if !pagesOf(cat, "grok")["image"] {
		t.Fatal("grok should have image page")
	}
	if !pagesOf(cat, "grok")["chat"] {
		t.Fatal("grok should have chat page")
	}
	if !pagesOf(cat, "mimo")["audio"] {
		t.Fatal("mimo should have audio page")
	}
	if pagesOf(cat, "mimo")["chat"] {
		t.Fatal("mimo should not have chat page")
	}

	raw, err := json.Marshal(cat)
	if err != nil {
		t.Fatal(err)
	}
	blob := string(raw)
	for _, secret := range []string{"sk-secret-antigravity", "sk-secret-image", "sk-secret-grok", "mimo-secret", "db-secret", "minio-secret", "minio-key"} {
		if strings.Contains(blob, secret) {
			t.Fatalf("catalog leaked secret %q", secret)
		}
	}
	if strings.Contains(blob, `"apiKey"`) || strings.Contains(blob, `"password"`) || strings.Contains(blob, `"secretAccessKey"`) {
		t.Fatal("catalog must not include secret field names")
	}

	chat, ok := FindConfig(cat, "ai:antigravity:chat")
	if !ok {
		t.Fatal("missing clustered chat config")
	}
	if len(chat.Models) != 2 {
		t.Fatalf("expected 2 clustered models, got %#v", chat.Models)
	}
	if chat.Models[0].Model != "gemini-3.1-pro" || chat.Models[1].Model != "gemini-3.6-flash-high" {
		t.Fatalf("clustered models = %#v", chat.Models)
	}
	if !chat.AllowCustomModel {
		t.Fatal("clustered chat should allow typing a model name")
	}
	antigravity := pagesOf(cat, "antigravity")
	_ = antigravity
	var chatCount int
	for _, group := range cat.Groups {
		if group.ID != "antigravity" {
			continue
		}
		for _, page := range group.Pages {
			if page.ID == "chat" {
				chatCount = len(page.Configs)
			}
		}
	}
	if chatCount != 1 {
		t.Fatalf("antigravity chat should be a single card, got %d", chatCount)
	}
	if hasLabel(chat, "质量") || hasLabel(chat, "默认音色") {
		t.Fatalf("chat config has unrelated fields: %#v", chat.PublicProperties)
	}
	image, _ := FindConfig(cat, "ai:antigravity:image")
	if !hasLabel(image, "质量") || !hasLabel(image, "默认尺寸") {
		t.Fatalf("image config missing image fields: %#v", image.PublicProperties)
	}
	db, _ := FindConfig(cat, "db:1")
	if hasLabel(db, "模型") || hasLabel(db, "默认音色") {
		t.Fatalf("database config has model fields: %#v", db.PublicProperties)
	}
	if db.Probe != "SELECT 1" {
		t.Fatalf("probe = %s", db.Probe)
	}
}

func TestAuroraDropdownIncludesPublishedCatalog(t *testing.T) {
	snapshot := config.Snapshot{
		AI: config.AIConfiguration{
			ActiveProviderID: "aurora-auto",
			Providers: []config.AIProvider{{
				ID: "aurora-auto", Label: "Aurora Auto", Type: "openai-compatible",
				BaseURL: "http://host.docker.internal:8088/v1", APIKey: "secret",
				Model: "auto", MaxTokens: 4096, Enabled: true,
				Capabilities: []string{"TEXT_GENERATION", "WEB_PROXY"}, Options: map[string]string{},
			}},
		},
		AIUpdatedAt: time.Date(2026, 9, 19, 1, 2, 3, 0, time.UTC),
	}
	cat := Project(snapshot, time.Date(2026, 9, 19, 8, 0, 0, 0, time.UTC), nil)
	chat, ok := FindConfig(cat, "ai:aurora:chat")
	if !ok {
		t.Fatal("missing aurora chat cluster")
	}
	if len(chat.Models) < 20 {
		t.Fatalf("expected the published Aurora catalog, got %d models: %#v", len(chat.Models), chat.Models)
	}
	got := map[string]ModelOption{}
	for _, item := range chat.Models {
		got[item.Model] = item
	}
	for _, slug := range []string{"auto", "gpt-5-6", "gpt-5-6-instant", "gpt-5-6-mini", "gpt-5-6-thinking", "gpt-5.6-sol-wm", "gpt-6-pro", "research"} {
		if _, ok := got[slug]; !ok {
			t.Fatalf("missing published slug %s", slug)
		}
	}
	if !got["auto"].Registered {
		t.Fatal("registered auto model should stay marked registered")
	}
	if got["research"].Category != "深度研究 (Research)" {
		t.Fatalf("research category = %q", got["research"].Category)
	}
	pages := pagesOf(cat, "aurora")
	if !pages["image"] || !pages["voice"] {
		t.Fatalf("aurora should expose image and voice pages, got %#v", pages)
	}
	image, ok := FindConfig(cat, "ai:aurora:image")
	if !ok || image.Model != "gpt-image-2" {
		t.Fatalf("aurora image = %#v", image)
	}
	tts, ok := FindConfig(cat, "ai:aurora:audio")
	if !ok || tts.PageID != "voice" {
		t.Fatalf("aurora tts = %#v", tts)
	}
	stt, ok := FindConfig(cat, "ai:aurora:transcribe")
	if !ok || stt.Kind != "transcribe" {
		t.Fatalf("aurora stt = %#v", stt)
	}
	if strings.Contains(mustJSON(t, cat), "secret") {
		t.Fatal("catalog leaked secret")
	}
}

func TestGrokDropdownAndSubmenus(t *testing.T) {
	snapshot := config.Snapshot{
		AI: config.AIConfiguration{
			ActiveProviderID: "sub2api-grok-image",
			Providers: []config.AIProvider{{
				ID: "grok2api", Label: "Grok2API", Type: "openai-compatible",
				BaseURL: "http://127.0.0.1:37821/v1", APIKey: "sk-grok-secret-key",
				Model: "grok-imagine-image", MaxTokens: 4096, Enabled: true,
				Capabilities: []string{"IMAGE_GENERATION", "IMAGE_REFERENCE"},
				Options: map[string]string{"size": "1536x864", "quality": "hd", "responseFormat": "b64_json"},
			}},
		},
		AIUpdatedAt: time.Date(2026, 9, 19, 1, 2, 3, 0, time.UTC),
	}
	cat := Project(snapshot, time.Date(2026, 9, 19, 8, 0, 0, 0, time.UTC), nil)

	var grokGroup *Group
	for i := range cat.Groups {
		if cat.Groups[i].ID == "grok" {
			grokGroup = &cat.Groups[i]
			break
		}
	}
	if grokGroup == nil {
		t.Fatal("missing grok group")
	}

	// 检查 4 个子菜单以及指定顺序: 生成视频(video) -> tts(audio) -> 生成图片(image) -> chat大模型(chat)
	if len(grokGroup.Pages) != 4 {
		t.Fatalf("expected 4 pages under Grok, got %d: %#v", len(grokGroup.Pages), grokGroup.Pages)
	}
	expectedPages := []struct {
		id    string
		label string
		kind  string
	}{
		{id: "video", label: "生成视频", kind: "video"},
		{id: "audio", label: "TTS", kind: "audio"},
		{id: "image", label: "生成图片", kind: "image"},
		{id: "chat", label: "Chat 大模型", kind: "chat"},
	}
	for i, exp := range expectedPages {
		p := grokGroup.Pages[i]
		if p.ID != exp.id || p.Label != exp.label || p.Kind != exp.kind {
			t.Fatalf("page[%d] mismatch: got id=%s, label=%s, kind=%s; want id=%s, label=%s, kind=%s",
				i, p.ID, p.Label, p.Kind, exp.id, exp.label, exp.kind)
		}
	}

	// 验证视频配置与模型
	videoCfg, ok := FindConfig(cat, "ai:grok:video")
	if !ok || videoCfg.Kind != "video" || videoCfg.Model != "grok-imagine-video-1.5" {
		t.Fatalf("grok video config = %#v", videoCfg)
	}
	if len(videoCfg.Models) < 2 {
		t.Fatalf("expected video models, got %#v", videoCfg.Models)
	}

	// 验证 TTS 配置与模型
	ttsCfg, ok := FindConfig(cat, "ai:grok:audio")
	if !ok || ttsCfg.Kind != "audio" || ttsCfg.Model != "grok-voice-latest" {
		t.Fatalf("grok tts config = %#v", ttsCfg)
	}
	if len(ttsCfg.Models) < 3 {
		t.Fatalf("expected tts models, got %#v", ttsCfg.Models)
	}

	// 验证图片配置与模型
	imageCfg, ok := FindConfig(cat, "ai:grok:image")
	if !ok || imageCfg.Kind != "image" || imageCfg.Model != "grok-imagine-image" {
		t.Fatalf("grok image config = %#v", imageCfg)
	}
	if len(imageCfg.Models) < 5 {
		t.Fatalf("expected image models, got %#v", imageCfg.Models)
	}

	// 验证 Chat 大模型配置与全部模型
	chatCfg, ok := FindConfig(cat, "ai:grok:chat")
	if !ok || chatCfg.Kind != "chat" || chatCfg.Model != "grok-4.20-0309-reasoning" {
		t.Fatalf("grok chat config = %#v", chatCfg)
	}
	if len(chatCfg.Models) < 15 {
		t.Fatalf("expected full grok chat models, got %d: %#v", len(chatCfg.Models), chatCfg.Models)
	}

	// 验证无秘钥泄露
	if strings.Contains(mustJSON(t, cat), "sk-grok-secret-key") {
		t.Fatal("catalog leaked grok secret")
	}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func groupIDs(cat Catalog) []string {
	ids := make([]string, 0, len(cat.Groups))
	for _, g := range cat.Groups {
		ids = append(ids, g.ID)
	}
	return ids
}

func pagesOf(cat Catalog, groupID string) map[string]bool {
	out := map[string]bool{}
	for _, g := range cat.Groups {
		if g.ID != groupID {
			continue
		}
		for _, p := range g.Pages {
			out[p.ID] = true
		}
	}
	return out
}

func hasLabel(cfg PublicConfig, label string) bool {
	for _, item := range cfg.PublicProperties {
		if item.Label == label {
			return true
		}
	}
	return false
}
