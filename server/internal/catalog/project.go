package catalog

import (
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"local_config_center/internal/config"
)

func Project(snapshot config.Snapshot, now time.Time, verifications map[string]Verification) Catalog {
	groups := map[string]*Group{}
	order := make([]string, 0)

	ensure := func(family Family) *Group {
		if existing, ok := groups[family.ID]; ok {
			return existing
		}
		group := &Group{
			ID:       family.ID,
			Label:    family.Label,
			Eyebrow:  family.Eyebrow,
			Mark:     family.Mark,
			Icon:     family.Icon,
			Pages:    nil,
			SortRank: family.Rank,
		}
		groups[family.ID] = group
		order = append(order, family.ID)
		return group
	}

	revisionAI := revisionOf("ai", snapshot.AIUpdatedAt)
	clusterOrder := make([]*aiCluster, 0)
	clusterIndex := map[string]*aiCluster{}
	addAI := func(provider config.AIProvider, kind string) {
		family := FamilyOf(provider)
		key := family.ID + "|" + kind + "|" + strings.ToLower(strings.TrimSpace(provider.Type)) + "|" + publicBaseURL(provider.BaseURL)
		if existing, ok := clusterIndex[key]; ok {
			existing.providers = append(existing.providers, provider)
			return
		}
		item := &aiCluster{family: family, kind: kind, providers: []config.AIProvider{provider}}
		clusterIndex[key] = item
		clusterOrder = append(clusterOrder, item)
	}
	for _, provider := range snapshot.AI.Providers {
		if !provider.Enabled {
			continue
		}
		if config.IsChatProvider(provider) {
			addAI(provider, "chat")
		}
		if config.IsImageProvider(provider) {
			addAI(provider, "image")
		}
		if config.IsVoiceProvider(provider) {
			addAI(provider, "audio")
		}
		if config.IsVideoProvider(provider) {
			addAI(provider, "video")
		}
	}
	clusterCounts := map[string]int{}
	clusterOrder = appendAuroraEndpointClusters(clusterOrder)
	clusterOrder = appendGrokEndpointClusters(clusterOrder)
	for _, item := range clusterOrder {
		clusterCounts[item.family.ID+"|"+item.kind]++
	}
	clusterSeen := map[string]int{}
	for _, item := range clusterOrder {
		group := ensure(item.family)
		key := item.family.ID + "|" + item.kind
		clusterSeen[key]++
		id := "ai:" + item.family.ID + ":" + item.kind
		if clusterCounts[key] > 1 {
			id = fmt.Sprintf("%s-%d", id, clusterSeen[key])
		}
		addConfig(group, clusterEntry(id, item.kind, item.family, item.providers, snapshot.AI.ActiveProviderID, revisionAI, verifications))
	}
	addAuroraTranscriptionPages(ensure, clusterOrder, snapshot.AI.ActiveProviderID, revisionAI, verifications)

	revisionDB := revisionOf("databases", snapshot.DBUpdatedAt)
	for _, database := range snapshot.Databases.Databases {
		family := localFamily()
		group := ensure(family)
		addConfig(group, databaseEntry(database, family.ID, revisionDB, verifications))
	}

	revisionCLI := revisionOf("local-cli", snapshot.CLIUpdatedAt)
	for _, item := range snapshot.LocalCLI.Configs {
		if !item.Enabled {
			continue
		}
		family := localFamily()
		group := ensure(family)
		addConfig(group, cliEntry(item, family.ID, revisionCLI, verifications))
	}

	if snapshot.ObjectStorage.Configured {
		family := storageFamily()
		group := ensure(family)
		revisionOS := revisionOf("objectStorage", snapshot.OSUpdatedAt)
		addConfig(group, storageEntry(snapshot.ObjectStorage, family.ID, "storage", revisionOS, verifications))
		addConfig(group, storageEntry(snapshot.ObjectStorage, family.ID, "media", revisionOS, verifications))
	}

	out := Catalog{
		Source:        "postgres:shared_config",
		SchemaVersion: "1",
		GeneratedAt:   now.UTC(),
		Groups:        make([]Group, 0, len(order)),
	}
	for _, id := range order {
		group := groups[id]
		if len(group.Pages) == 0 {
			continue
		}
		out.Groups = append(out.Groups, *group)
	}
	return out
}

type aiCluster struct {
	family    Family
	kind      string
	providers []config.AIProvider
}

func addConfig(group *Group, cfg PublicConfig) {
	page := pageForConfig(cfg)
	idx := -1
	for i := range group.Pages {
		if group.Pages[i].ID == page.ID {
			idx = i
			break
		}
	}
	if idx < 0 {
		page.Configs = []PublicConfig{cfg}
		group.Pages = append(group.Pages, page)
		return
	}
	group.Pages[idx].Configs = append(group.Pages[idx].Configs, cfg)
}

func pageForConfig(cfg PublicConfig) Page {
	if cfg.Kind == "gateway" || cfg.PageID == "gateway" {
		return Page{ID: "gateway", Label: "网关接入", En: "Local gateway", Kind: "gateway", Icon: "lock", Description: "给本机其它项目调用的 JSON 网关。鉴权密钥写在请求 JSON 的 auth 字段里，不使用各厂商 API Key。"}
	}
	if cfg.PageID == "voice" || cfg.Kind == "transcribe" {
		return Page{ID: "voice", Label: "语音与听写", En: "Speech", Kind: "audio", Icon: "audio", Description: "测试文本转语音和语音识别。合成结果直接试听，转写结果直接阅读。"}
	}
	if strings.Contains(cfg.ID, ":grok:") {
		switch cfg.Kind {
		case "video":
			return Page{ID: "video", Label: "生成视频", En: "Video generation", Kind: "video", Icon: "video", Description: "查看 Grok 视频生成能力、任务状态与播放结果。无需跳转到外部页面。"}
		case "audio":
			return Page{ID: "audio", Label: "TTS", En: "Text to speech", Kind: "audio", Icon: "audio", Description: "测试 Grok 文本转语音。输入要朗读的文本，在结果区直接试听真实合成的语音。"}
		case "image":
			return Page{ID: "image", Label: "生成图片", En: "Image generation", Kind: "image", Icon: "image", Description: "查看 Grok 图片模型与输出规格。结果直接以图片展示，点击可放大。"}
		case "chat":
			return Page{ID: "chat", Label: "Chat 大模型", En: "Language models", Kind: "chat", Icon: "chat", Description: "查看 Grok 模型连接与返回能力。输入内容后，在右侧工作台查看真实返回结果。"}
		}
	}
	if cfg.Kind == "image" && strings.Contains(cfg.ID, ":aurora:") {
		page := pageMeta("image")
		page.Label = "AI 画图"
		page.Description = "调用 Aurora 的 /v1/images/generations。结果直接以图片展示，点击可放大。"
		return page
	}
	return pageMeta(cfg.Kind)
}

func appendAuroraEndpointClusters(in []*aiCluster) []*aiCluster {
	var chat *aiCluster
	hasImage, hasAudio := false, false
	for _, item := range in {
		if !isAuroraFamily(item.family, item.providers) {
			continue
		}
		switch item.kind {
		case "chat":
			chat = item
		case "image":
			hasImage = true
		case "audio":
			hasAudio = true
		}
	}
	if chat == nil {
		return in
	}
	out := in
	if !hasImage {
		out = append(out, &aiCluster{family: chat.family, kind: "image", providers: chat.providers})
	}
	if !hasAudio {
		out = append(out, &aiCluster{family: chat.family, kind: "audio", providers: chat.providers})
	}
	return out
}

func addAuroraTranscriptionPages(ensure func(Family) *Group, clusters []*aiCluster, activeID, revision string, verifications map[string]Verification) {
	var chat *aiCluster
	for _, item := range clusters {
		if item.kind == "chat" && isAuroraFamily(item.family, item.providers) {
			chat = item
			break
		}
	}
	if chat == nil {
		return
	}
	primary := choosePrimary(chat.providers, activeID)
	cfg := transcribeEntry(primary, chat.family.ID, revision, verifications)
	cfg.ID = "ai:" + chat.family.ID + ":transcribe"
	cfg.Title = "语音识别转文字"
	cfg.PageID = "voice"
	cfg.AllowCustomModel = true
	cfg.Model = "whisper-1"
	cfg.Models = append([]ModelOption{}, AuroraSTTModels...)
	cfg.MemberIDs = make([]string, 0, len(chat.providers))
	for _, provider := range chat.providers {
		cfg.MemberIDs = append(cfg.MemberIDs, provider.ID)
	}
	cfg.PublicProperties = withoutLabels(cfg.PublicProperties, "模型", "配置 ID", "默认音色")
	cfg.PublicProperties = append([]Property{{Label: "接口路径", Value: "/v1/audio/transcriptions"}}, cfg.PublicProperties...)
	cfg.PublicProperties = append(cfg.PublicProperties, Property{Label: "可选模型", Value: fmt.Sprintf("%d 个可选，可下拉或手输", len(cfg.Models))})
	addConfig(ensure(chat.family), finish(cfg, verifications))
}

func pageMeta(kind string) Page {
	switch kind {
	case "chat":
		return Page{ID: "chat", Label: "大模型配置", En: "Language models", Kind: "chat", Icon: "chat", Description: "查看模型连接与返回能力。输入内容后，在右侧工作台查看真实返回结果。"}
	case "image":
		return Page{ID: "image", Label: "图片模型", En: "Image generation", Kind: "image", Icon: "image", Description: "查看图片模型与输出规格。结果直接以图片展示，点击可放大。"}
	case "audio":
		return Page{ID: "audio", Label: "TTS 语音", En: "Text to speech", Kind: "audio", Icon: "audio", Description: "输入要朗读的文本，在结果区直接试听真实合成的语音。"}
	case "video":
		return Page{ID: "video", Label: "视频模型", En: "Video generation", Kind: "video", Icon: "video", Description: "查看视频生成能力、任务状态与播放结果。无需跳转到外部页面。"}
	case "database":
		return Page{ID: "database", Label: "数据库", En: "Database connections", Kind: "database", Icon: "database", Description: "只读展示连接信息。测试仅检查连接或执行受控的只读探针。"}
	case "cli":
		return Page{ID: "cli", Label: "本地 CLI", En: "Local CLI", Kind: "cli", Icon: "layers", Description: "展示本地命令配置。测试只检查可执行文件是否存在，不会替你发起一次完整任务。"}
	case "storage":
		return Page{ID: "storage", Label: "对象存储", En: "Storage connection", Kind: "storage", Icon: "box", Description: "展示存储端点、桶与权限范围。测试结果使用对象列表，而不是模型对话框。"}
	default:
		return Page{ID: "media", Label: "媒体访问", En: "Media delivery", Kind: "media", Icon: "play", Description: "测试媒体读取与预览。图片、音频、视频使用各自的查看组件。"}
	}
}

func clusterEntry(id, kind string, family Family, providers []config.AIProvider, activeID, revision string, verifications map[string]Verification) PublicConfig {
	primary := choosePrimary(providers, activeID)
	var cfg PublicConfig
	switch kind {
	case "image":
		cfg = imageEntry(primary, family.ID, revision, verifications)
	case "audio":
		cfg = audioEntry(primary, family.ID, revision, verifications)
	case "video":
		cfg = videoEntry(primary, family.ID, revision, verifications)
	default:
		cfg = chatEntry(primary, family.ID, revision, verifications)
	}
	cfg.ID = id
	cfg.Title = family.Label + " · " + kindTitle(kind)
	cfg.AllowCustomModel = true
	cfg.Models = make([]ModelOption, 0, len(providers))
	cfg.MemberIDs = make([]string, 0, len(providers))
	seenModel := map[string]bool{}
	for _, provider := range providers {
		cfg.MemberIDs = append(cfg.MemberIDs, provider.ID)
		if seenModel[provider.Model] {
			continue
		}
		seenModel[provider.Model] = true
		cfg.Models = append(cfg.Models, ModelOption{ProviderID: provider.ID, Label: provider.Label, Model: provider.Model, Registered: true})
	}
	cfg.Models = mergeKnownChatModels(family, kind, providers, cfg.Models)
	if isAuroraFamily(family, providers) {
		cfg = applyAuroraEndpointDefaults(cfg, kind)
	}
	if isGrokFamily(family, providers) {
		cfg = applyGrokEndpointDefaults(cfg, kind)
	}
	cfg = applyAntigravityImageDefaults(cfg, family, kind)
	registeredCount := 0
	for _, item := range cfg.Models {
		if item.Registered {
			registeredCount++
		}
	}
	cfg.PublicProperties = withoutLabels(cfg.PublicProperties, "模型", "配置 ID")
	cfg.PublicProperties = append(cfg.PublicProperties, Property{
		Label: "可选模型",
		Value: fmt.Sprintf("%d 个可选，其中 %d 个已在配置中心登记；可下拉或手输", len(cfg.Models), registeredCount),
	})
	if kind == "chat" {
		cfg.PublicProperties = withoutLabels(cfg.PublicProperties, "输出上限")
	}
	return finish(cfg, verifications)
}

func choosePrimary(providers []config.AIProvider, activeID string) config.AIProvider {
	for _, provider := range providers {
		if provider.ID == activeID {
			return provider
		}
	}
	return providers[0]
}

func kindTitle(kind string) string {
	switch kind {
	case "chat":
		return "对话"
	case "image":
		return "图片"
	case "audio":
		return "语音"
	case "video":
		return "视频"
	default:
		return kind
	}
}

func withoutLabels(props []Property, labels ...string) []Property {
	skip := map[string]bool{}
	for _, label := range labels {
		skip[label] = true
	}
	out := make([]Property, 0, len(props))
	for _, prop := range props {
		if skip[prop.Label] {
			continue
		}
		out = append(out, prop)
	}
	return out
}

func sharedTokenLimit(providers []config.AIProvider) string {
	if len(providers) == 0 {
		return ""
	}
	first := providers[0].MaxTokens
	for _, provider := range providers[1:] {
		if provider.MaxTokens != first {
			return "按所选模型"
		}
	}
	return formatTokens(first)
}

func chatEntry(provider config.AIProvider, groupID, revision string, verifications map[string]Verification) PublicConfig {
	props := []Property{
		{Label: "接口协议", Value: displayType(provider.Type)},
		{Label: "接口地址", Value: publicBaseURL(provider.BaseURL)},
		{Label: "模型", Value: provider.Model},
	}
	if provider.ID != "" {
		props = append(props, Property{Label: "配置 ID", Value: provider.ID})
	}
	formats := []string{"text"}
	if config.HasCapability(provider, "JSON_OBJECT") || config.IsOpenAICompatible(provider) {
		formats = append(formats, "json")
	}
	delivery := []string{"complete"}
	if config.HasCapability(provider, "STREAM") || config.IsOpenAICompatible(provider) {
		delivery = []string{"stream", "complete"}
	}
	return finish(PublicConfig{
		ID:               "ai:" + provider.ID + ":chat",
		GroupID:          groupID,
		PageID:           "chat",
		Kind:             "chat",
		Title:            provider.Label,
		Model:            provider.Model,
		Revision:         revision,
		RendererKey:      "chat",
		PublicProperties: props,
		CredentialState:  credentialState(provider.APIKey),
		Capabilities: CapabilitySet{
			Delivery:     delivery,
			Formats:      formats,
			Cancellation: "unsupported",
		},
	}, verifications)
}

func imageEntry(provider config.AIProvider, groupID, revision string, verifications map[string]Verification) PublicConfig {
	size := option(provider, "size")
	format := option(provider, "responseFormat")
	quality := option(provider, "quality")
	props := []Property{
		{Label: "接口协议", Value: displayType(provider.Type)},
		{Label: "接口地址", Value: publicBaseURL(provider.BaseURL)},
		{Label: "模型", Value: provider.Model},
	}
	if size != "" {
		props = append(props, Property{Label: "默认尺寸", Value: size})
	}
	if format != "" {
		props = append(props, Property{Label: "返回格式", Value: format})
	}
	if quality != "" {
		props = append(props, Property{Label: "质量", Value: quality})
	}
	return finish(PublicConfig{
		ID:               "ai:" + provider.ID + ":image",
		GroupID:          groupID,
		PageID:           "image",
		Kind:             "image",
		Title:            provider.Label,
		Model:            provider.Model,
		Revision:         revision,
		RendererKey:      "image",
		PublicProperties: props,
		CredentialState:  credentialState(provider.APIKey),
		Capabilities: CapabilitySet{
			Delivery:       []string{"complete"},
			Formats:        []string{"image"},
			ReferenceImage: config.HasCapability(provider, "IMAGE_REFERENCE"),
			Cancellation:   "unsupported",
		},
	}, verifications)
}

func audioEntry(provider config.AIProvider, groupID, revision string, verifications map[string]Verification) PublicConfig {
	voice := strings.TrimSpace(provider.Voice)
	if voice == "" {
		voice = option(provider, "voice")
	}
	if voice == "" {
		voice = "跟随供应商默认音色"
	}
	props := []Property{
		{Label: "接口协议", Value: displayType(provider.Type)},
		{Label: "接口地址", Value: publicBaseURL(provider.BaseURL)},
		{Label: "模型", Value: provider.Model},
		{Label: "默认音色", Value: voice},
	}
	return finish(PublicConfig{
		ID:               "ai:" + provider.ID + ":audio",
		GroupID:          groupID,
		PageID:           "audio",
		Kind:             "audio",
		Title:            provider.Label,
		Model:            provider.Model,
		Revision:         revision,
		RendererKey:      "audio",
		PublicProperties: props,
		CredentialState:  credentialState(provider.APIKey),
		Capabilities: CapabilitySet{
			Delivery:     []string{"complete"},
			Formats:      []string{"audio"},
			Cancellation: "unsupported",
		},
	}, verifications)
}

func transcribeEntry(provider config.AIProvider, groupID, revision string, verifications map[string]Verification) PublicConfig {
	return finish(PublicConfig{
		ID:      "ai:" + provider.ID + ":transcribe",
		GroupID: groupID,
		PageID:  "voice",
		Kind:    "transcribe",
		Title:   "语音识别转文字",
		Model:   "whisper-1",
		Revision: revision,
		RendererKey: "transcribe",
		PublicProperties: []Property{
			{Label: "接口协议", Value: displayType(provider.Type)},
			{Label: "接口地址", Value: publicBaseURL(provider.BaseURL)},
			{Label: "接口路径", Value: "/v1/audio/transcriptions"},
		},
		CredentialState: credentialState(provider.APIKey),
		Capabilities: CapabilitySet{
			Delivery:      []string{"complete"},
			Formats:       []string{"text"},
			Cancellation:  "unsupported",
			AcceptedInput: []string{"audio/mpeg", "audio/wav", "audio/mp4", "audio/webm", "audio/ogg"},
		},
	}, verifications)
}

func videoEntry(provider config.AIProvider, groupID, revision string, verifications map[string]Verification) PublicConfig {
	props := []Property{
		{Label: "接口协议", Value: displayType(provider.Type)},
		{Label: "接口地址", Value: publicBaseURL(provider.BaseURL)},
		{Label: "模型", Value: provider.Model},
		{Label: "任务方式", Value: "异步生成后在本页播放"},
	}
	if duration := option(provider, "duration"); duration != "" {
		props = append(props, Property{Label: "默认时长", Value: duration})
	}
	if ratio := option(provider, "aspectRatio"); ratio != "" {
		props = append(props, Property{Label: "默认画幅", Value: ratio})
	}
	return finish(PublicConfig{
		ID:               "ai:" + provider.ID + ":video",
		GroupID:          groupID,
		PageID:           "video",
		Kind:             "video",
		Title:            provider.Label,
		Model:            provider.Model,
		Revision:         revision,
		RendererKey:      "video",
		PublicProperties: props,
		CredentialState:  credentialState(provider.APIKey),
		Capabilities: CapabilitySet{
			Delivery:       []string{"complete"},
			Formats:        []string{"video"},
			ReferenceImage: config.HasCapability(provider, "IMAGE_REFERENCE") || option(provider, "requiresImage") == "true",
			Cancellation:   "unsupported",
		},
	}, verifications)
}

func databaseEntry(database config.DatabaseConnection, groupID, revision string, verifications map[string]Verification) PublicConfig {
	engine := strings.TrimSpace(database.Type)
	probe := "SELECT 1"
	if strings.EqualFold(engine, "redis") {
		probe = "PING"
	}
	tls := "以连接参数为准"
	if v, ok := database.Parameters["sslmode"]; ok && strings.TrimSpace(v) != "" {
		tls = v
	}
	props := []Property{
		{Label: "引擎", Value: engine},
		{Label: "主机", Value: database.Host},
		{Label: "端口", Value: fmt.Sprintf("%d", database.Port)},
		{Label: "数据库名称", Value: database.Database},
		{Label: "连接账户", Value: database.Username},
		{Label: "环境", Value: emptyAs(database.Environment, "未标注")},
		{Label: "传输加密", Value: tls},
		{Label: "允许的测试", Value: probe},
	}
	return finish(PublicConfig{
		ID:               "db:" + database.ID,
		GroupID:          groupID,
		PageID:           "database",
		Kind:             "database",
		Title:            database.Name,
		Model:            engine + " · " + database.Host,
		Revision:         revision,
		RendererKey:      "database",
		PublicProperties: props,
		CredentialState:  credentialState(database.Password),
		Capabilities: CapabilitySet{
			Delivery:     []string{"complete"},
			Formats:      []string{"database"},
			Cancellation: "unsupported",
		},
		Probe: probe,
	}, verifications)
}

func cliEntry(item config.LocalCLIConfig, groupID, revision string, verifications map[string]Verification) PublicConfig {
	props := []Property{
		{Label: "命令路径", Value: item.Command},
		{Label: "默认参数", Value: strings.Join(item.DefaultArgs, " ")},
		{Label: "工作目录", Value: emptyAs(item.WorkingDirectory, "未指定")},
		{Label: "超时", Value: fmt.Sprintf("%d 秒", item.TimeoutSeconds)},
	}
	if strings.TrimSpace(item.Model) != "" {
		props = append(props, Property{Label: "模型", Value: item.Model})
	}
	return finish(PublicConfig{
		ID:               "cli:" + item.ID,
		GroupID:          groupID,
		PageID:           "cli",
		Kind:             "cli",
		Title:            item.Label,
		Model:            emptyAs(item.Model, item.Command),
		Revision:         revision,
		RendererKey:      "cli",
		PublicProperties: props,
		CredentialState:  "not_required",
		Capabilities: CapabilitySet{
			Delivery:     []string{"complete"},
			Formats:      []string{"diagnostic"},
			Cancellation: "unsupported",
		},
		Probe: "可执行文件检查",
	}, verifications)
}

func storageEntry(storage config.ObjectStorageConfiguration, groupID, kind, revision string, verifications map[string]Verification) PublicConfig {
	id := "storage:object"
	title := "素材存储"
	model := "S3-compatible"
	renderer := "storage"
	pageID := "storage"
	if kind == "media" {
		id = "storage:media"
		title = "媒体预览入口"
		renderer = "media"
		pageID = "media"
	}
	scheme := "http"
	if storage.UseSSL {
		scheme = "https"
	}
	props := []Property{
		{Label: "服务端点", Value: storage.Endpoint},
		{Label: "访问方式", Value: scheme},
		{Label: "Bucket", Value: storage.BucketName},
		{Label: "对象前缀", Value: emptyAs(storage.BasePath, "（桶根目录）")},
		{Label: "访问范围", Value: "只读对象预览"},
	}
	secret := storage.AccessKeyID
	if strings.TrimSpace(storage.SecretAccessKey) == "" {
		secret = ""
	}
	state := "configured"
	if strings.TrimSpace(storage.AccessKeyID) == "" || strings.TrimSpace(storage.SecretAccessKey) == "" {
		state = "missing"
	}
	_ = secret
	return finish(PublicConfig{
		ID:               id,
		GroupID:          groupID,
		PageID:           pageID,
		Kind:             kind,
		Title:            title,
		Model:            model,
		Revision:         revision,
		RendererKey:      renderer,
		PublicProperties: props,
		CredentialState:  state,
		Capabilities: CapabilitySet{
			Delivery:     []string{"complete"},
			Formats:      []string{kind},
			Cancellation: "unsupported",
		},
	}, verifications)
}

func finish(cfg PublicConfig, verifications map[string]Verification) PublicConfig {
	cfg.Properties = make([][]string, 0, len(cfg.PublicProperties))
	for _, item := range cfg.PublicProperties {
		cfg.Properties = append(cfg.Properties, []string{item.Label, item.Value})
	}
	if v, ok := verifications[cfg.ID]; ok {
		copied := v
		cfg.LastVerification = &copied
		if v.Revision == cfg.Revision && v.Status == "passed" {
			cfg.Status = "verified"
			caps := cfg.Capabilities
			caps.Verified = true
			cfg.Capabilities = caps
		} else if v.Status == "passed" {
			cfg.Status = "stale"
		} else {
			cfg.Status = "failed"
		}
	} else {
		cfg.Status = "unverified"
	}
	return cfg
}

func credentialState(secret string) string {
	if strings.TrimSpace(secret) == "" {
		return "missing"
	}
	return "configured"
}

func option(provider config.AIProvider, key string) string {
	if provider.Options == nil {
		return ""
	}
	return strings.TrimSpace(provider.Options[key])
}

func displayType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "openai-compatible":
		return "OpenAI 兼容协议"
	case "mimo-tts":
		return "小米 Mimo TTS"
	case "grok-tts":
		return "Grok TTS"
	default:
		return value
	}
}

func publicBaseURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" {
		return raw
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func formatTokens(n int) string {
	if n <= 0 {
		return "未配置"
	}
	if n >= 1000 {
		return fmt.Sprintf("%s tokens", comma(n))
	}
	return fmt.Sprintf("%d tokens", n)
}

func comma(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	return s + "," + strings.Join(parts, ",")
}

func emptyAs(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func revisionOf(key string, at time.Time) string {
	if at.IsZero() {
		return key + "-missing"
	}
	return key + "-" + at.UTC().Format("20060102T150405Z")
}

func FamilyOf(provider config.AIProvider) Family {
	blob := strings.ToLower(provider.ID + " " + provider.Label + " " + provider.Type)
	switch {
	case strings.Contains(blob, "antigravity"):
		return Family{ID: "antigravity", Label: "Antigravity Tool", Eyebrow: "MODEL GATEWAY", Mark: "A", Icon: "spark", Rank: 10}
	case strings.Contains(blob, "grok"):
		return Family{ID: "grok", Label: "Grok", Eyebrow: "CUSTOM PROVIDER", Mark: "G", Icon: "orbit", Rank: 20}
	case strings.Contains(blob, "aurora"):
		return Family{ID: "aurora", Label: "Aurora", Eyebrow: "WEB PROXY", Mark: "U", Icon: "orbit", Rank: 30}
	case strings.Contains(blob, "mimo") || strings.Contains(blob, "xiaomi"):
		return Family{ID: "mimo", Label: "小米 Mimo", Eyebrow: "VOICE SERVICE", Mark: "米", Icon: "audio", Rank: 40}
	default:
		mark, _ := utf8.DecodeRuneInString(strings.ToUpper(provider.Label + "P"))
		id := "provider-" + sanitizeID(provider.ID)
		return Family{ID: id, Label: provider.Label, Eyebrow: displayType(provider.Type), Mark: string(mark), Icon: "spark", Rank: 80}
	}
}

func localFamily() Family {
	return Family{ID: "local", Label: "本地服务", Eyebrow: "LOCAL INFRASTRUCTURE", Mark: "L", Icon: "database", Rank: 90}
}

func storageFamily() Family {
	return Family{ID: "minio", Label: "MinIO", Eyebrow: "OBJECT STORAGE", Mark: "M", Icon: "box", Rank: 100}
}

func sanitizeID(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(value) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "item"
	}
	return out
}

func FindConfig(cat Catalog, id string) (PublicConfig, bool) {
	for _, group := range cat.Groups {
		for _, page := range group.Pages {
			for _, cfg := range page.Configs {
				if cfg.ID == id {
					return cfg, true
				}
			}
		}
	}
	return PublicConfig{}, false
}
