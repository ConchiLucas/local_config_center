package testhub

import (
	"fmt"
	"os"
	"strings"

	"local_config_center/internal/catalog"
	"local_config_center/internal/config"
)

const (
	DefaultSub2APIBaseURL = "http://sub2api:8090/v1"
	DefaultSub2APIAPIKey  = "sk-0ef4b56f19a410eb32a2f3f1a536d6f065499e33122c37e4de309de6d56815e2"
)

func getSub2APIBaseURL() string {
	if u := strings.TrimSpace(os.Getenv("SUB2API_BASE_URL")); u != "" {
		return u
	}
	return DefaultSub2APIBaseURL
}

func getSub2APIAPIKey() string {
	if k := strings.TrimSpace(os.Getenv("SUB2API_API_KEY")); k != "" {
		return k
	}
	return DefaultSub2APIAPIKey
}

type Binding struct {
	Public    catalog.PublicConfig
	Kind      string
	Provider  *config.AIProvider
	Providers []config.AIProvider
	Database  *config.DatabaseConnection
	Storage   *config.ObjectStorageConfiguration
	CLI       *config.LocalCLIConfig
}

func Bind(snapshot config.Snapshot, configID string) (Binding, error) {
	cat := catalog.Project(snapshot, snapshot.AIUpdatedAt, nil)
	pub, ok := catalog.FindConfig(cat, configID)
	if !ok {
		return Binding{}, fmt.Errorf("unknown config")
	}
	binding := Binding{Public: pub, Kind: pub.Kind}
	switch {
	case strings.HasPrefix(configID, "ai:"):
		matched := matchProviders(snapshot, pub)
		if len(matched) == 0 {
			return Binding{}, fmt.Errorf("provider no longer present")
		}
		binding.Providers = matched
		primary := matched[0]
		binding.Provider = &primary
		if configID == "ai:grok:audio" {
			sub2apiProvider := config.AIProvider{
				ID:           "sub2api-grok-tts",
				Label:        "Sub2API Grok TTS",
				Type:         "sub2api-tts",
				BaseURL:      getSub2APIBaseURL(),
				APIKey:       getSub2APIAPIKey(),
				Model:        "grok-voice-latest",
				Voice:        "Ara",
				Capabilities: []string{"AUDIO_TTS"},
				Enabled:      true,
			}
			binding.Provider = &sub2apiProvider
			binding.Providers = []config.AIProvider{sub2apiProvider}
		}
		return binding, nil
	case strings.HasPrefix(configID, "db:"):
		id := strings.TrimPrefix(configID, "db:")
		for i := range snapshot.Databases.Databases {
			if snapshot.Databases.Databases[i].ID == id {
				d := snapshot.Databases.Databases[i]
				binding.Database = &d
				return binding, nil
			}
		}
		return Binding{}, fmt.Errorf("database no longer present")
	case strings.HasPrefix(configID, "cli:"):
		id := strings.TrimPrefix(configID, "cli:")
		for i := range snapshot.LocalCLI.Configs {
			if snapshot.LocalCLI.Configs[i].ID == id {
				c := snapshot.LocalCLI.Configs[i]
				binding.CLI = &c
				return binding, nil
			}
		}
		return Binding{}, fmt.Errorf("cli config no longer present")
	case configID == "storage:object" || configID == "storage:media":
		if !snapshot.ObjectStorage.Configured {
			return Binding{}, fmt.Errorf("object storage is not configured")
		}
		s := snapshot.ObjectStorage
		binding.Storage = &s
		return binding, nil
	default:
		return Binding{}, fmt.Errorf("unknown config kind")
	}
}

func matchProviders(snapshot config.Snapshot, pub catalog.PublicConfig) []config.AIProvider {
	wanted := map[string]bool{}
	for _, id := range pub.MemberIDs {
		wanted[id] = true
	}
	matched := make([]config.AIProvider, 0, len(wanted))
	if len(wanted) > 0 {
		for _, provider := range snapshot.AI.Providers {
			if wanted[provider.ID] {
				matched = append(matched, provider)
			}
		}
		if len(matched) > 0 {
			return matched
		}
	}
	rest := strings.TrimPrefix(pub.ID, "ai:")
	parts := strings.Split(rest, ":")
	if len(parts) < 2 {
		return nil
	}
	kind := parts[len(parts)-1]
	familyID := strings.Join(parts[:len(parts)-1], ":")
	for _, provider := range snapshot.AI.Providers {
		if catalog.FamilyOf(provider).ID != familyID {
			continue
		}
		if kindMatches(provider, kind) {
			matched = append(matched, provider)
		}
	}
	return matched
}

func kindMatches(provider config.AIProvider, kind string) bool {
	switch kind {
	case "chat":
		return config.IsChatProvider(provider) || isGrokProvider(provider)
	case "image":
		return config.IsImageProvider(provider) || config.HasCapability(provider, "WEB_PROXY") || isGrokProvider(provider)
	case "audio", "transcribe":
		return config.IsVoiceProvider(provider) || config.HasCapability(provider, "WEB_PROXY") || isGrokProvider(provider)
	case "video":
		return config.IsVideoProvider(provider) || isGrokProvider(provider)
	default:
		return false
	}
}

func isGrokProvider(provider config.AIProvider) bool {
	blob := strings.ToLower(provider.ID + " " + provider.Label + " " + provider.Model)
	return strings.Contains(blob, "grok")
}

func pickProvider(binding Binding, model string) *config.AIProvider {
	model = strings.TrimSpace(model)
	if model == "" {
		model = strings.TrimSpace(binding.Public.Model)
	}
	if binding.Public.ID == "ai:grok:audio" && binding.Provider != nil {
		copied := *binding.Provider
		if model != "" {
			copied.Model = model
		}
		return &copied
	}
	candidates := binding.Providers
	if len(candidates) == 0 && binding.Provider != nil {
		candidates = []config.AIProvider{*binding.Provider}
	}
	if len(candidates) == 0 {
		return binding.Provider
	}
	if model != "" {
		for i := range candidates {
			if candidates[i].Model == model || candidates[i].ID == model {
				copied := candidates[i]
				return &copied
			}
		}
		copied := candidates[0]
		copied.Model = model
		return &copied
	}
	copied := candidates[0]
	if binding.Provider != nil {
		copied = *binding.Provider
	}
	return &copied
}
