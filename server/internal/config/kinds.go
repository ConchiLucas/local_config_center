package config

import "strings"

func IsChatProvider(provider AIProvider) bool {
	if !provider.Enabled {
		return false
	}
	if strings.TrimSpace(provider.ID) == "" || strings.TrimSpace(provider.BaseURL) == "" || strings.TrimSpace(provider.Model) == "" {
		return false
	}
	return HasCapability(provider, "TEXT_GENERATION") || HasCapability(provider, "CHAT")
}

func IsImageProvider(provider AIProvider) bool {
	if !provider.Enabled {
		return false
	}
	if strings.TrimSpace(provider.ID) == "" || strings.TrimSpace(provider.BaseURL) == "" || strings.TrimSpace(provider.Model) == "" {
		return false
	}
	return HasCapability(provider, "IMAGE_GENERATION")
}

func IsVoiceProvider(provider AIProvider) bool {
	if !provider.Enabled {
		return false
	}
	if strings.TrimSpace(provider.ID) == "" || strings.TrimSpace(provider.BaseURL) == "" || strings.TrimSpace(provider.Model) == "" {
		return false
	}
	return HasCapability(provider, "AUDIO_TTS")
}

func IsVideoProvider(provider AIProvider) bool {
	if !provider.Enabled {
		return false
	}
	if strings.TrimSpace(provider.ID) == "" || strings.TrimSpace(provider.BaseURL) == "" || strings.TrimSpace(provider.Model) == "" {
		return false
	}
	return HasCapability(provider, "VIDEO_GENERATION")
}
