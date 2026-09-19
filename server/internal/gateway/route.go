package gateway

import (
	"strings"

	"local_config_center/internal/catalog"
)

func NormalizeVendor(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "ag", "antigravity", "antigravity-tool":
		return "antigravity"
	case "aurora":
		return "aurora"
	case "grok":
		return "grok"
	case "mimo", "xiaomi", "xiaomi-mimo":
		return "mimo"
	default:
		return strings.ToLower(strings.TrimSpace(raw))
	}
}

func SplitModel(raw string) (vendor, slug string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}
	vendor, slug, ok := strings.Cut(raw, "/")
	if !ok {
		return "", raw
	}
	return NormalizeVendor(vendor), slug
}

func ConfigID(vendor, kind string) string {
	vendor = NormalizeVendor(vendor)
	switch kind {
	case "transcribe":
		return "ai:" + vendor + ":transcribe"
	case "audio":
		if vendor == "aurora" {
			return "ai:" + vendor + ":audio"
		}
		return "ai:" + vendor + ":audio"
	default:
		return "ai:" + vendor + ":" + kind
	}
}

func InferVendor(cat catalog.Catalog, kind, slug string) string {
	for _, group := range cat.Groups {
		for _, page := range group.Pages {
			if page.Kind != kind && page.ID != kind {
				continue
			}
			for _, cfg := range page.Configs {
				if cfg.Kind != kind {
					continue
				}
				if cfg.Model == slug {
					return group.ID
				}
				for _, item := range cfg.Models {
					if item.Model == slug {
						return group.ID
					}
				}
			}
		}
	}
	return ""
}

func ComposeImageSlug(vendor, slug, resolution, aspect string) string {
	if NormalizeVendor(vendor) != "antigravity" {
		return slug
	}
	if strings.TrimSpace(resolution) == "" && strings.TrimSpace(aspect) == "" {
		return slug
	}
	family := "flash-3.1"
	if strings.HasPrefix(slug, "gemini-3-pro-image") {
		family = "pro-3"
	}
	return catalog.ComposeAntigravityImageModel(family, resolution, aspect)
}
