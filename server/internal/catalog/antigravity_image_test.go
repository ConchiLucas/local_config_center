package catalog

import (
	"testing"
	"time"

	"local_config_center/internal/config"
)

func TestComposeAntigravityImageModel(t *testing.T) {
	cases := map[string][3]string{
		"gemini-3.1-flash-image":      {"flash-3.1", "", ""},
		"gemini-3-pro-image":          {"pro-3", "", ""},
		"gemini-3-pro-image-16x9":     {"pro-3", "", "16x9"},
		"gemini-3-pro-image-2k":       {"pro-3", "2k", ""},
		"gemini-3-pro-image-2k-9x16":  {"pro-3", "2k", "9x16"},
		"gemini-3-pro-image-4k-21x9":  {"pro-3", "4k", "21x9"},
	}
	for want, in := range cases {
		got := ComposeAntigravityImageModel(in[0], in[1], in[2])
		if got != want {
			t.Fatalf("compose(%q,%q,%q)=%q want %q", in[0], in[1], in[2], got, want)
		}
	}
	if n := len(AntigravityImageModels()); n != 22 {
		t.Fatalf("expected 22 image slugs, got %d", n)
	}
}

func TestAntigravityImagePickerIsAttached(t *testing.T) {
	snapshot := config.Snapshot{
		AI: config.AIConfiguration{Providers: []config.AIProvider{{
			ID: "antigravity-gemini-image", Label: "Antigravity Gemini Image", Type: "openai-compatible",
			BaseURL: "http://127.0.0.1:8045/v1", APIKey: "secret",
			Model: "gemini-3.1-flash-image", MaxTokens: 4096, Enabled: true,
			Capabilities: []string{"IMAGE_GENERATION"}, Options: map[string]string{"size": "1536x864"},
		}}},
		AIUpdatedAt: time.Date(2026, 9, 19, 1, 2, 3, 0, time.UTC),
	}
	cat := Project(snapshot, time.Date(2026, 9, 19, 8, 0, 0, 0, time.UTC), nil)
	image, ok := FindConfig(cat, "ai:antigravity:image")
	if !ok {
		t.Fatal("missing antigravity image config")
	}
	if image.ModelPicker.Style != "antigravity-image" {
		t.Fatalf("picker = %#v", image.ModelPicker)
	}
	if len(image.Models) < 22 {
		t.Fatalf("models = %d", len(image.Models))
	}
	found := map[string]bool{}
	for _, item := range image.Models {
		found[item.Model] = true
	}
	for _, slug := range []string{"gemini-3.1-flash-image", "gemini-3-pro-image", "gemini-3-pro-image-2k-16x9", "gemini-3-pro-image-4k-9x16"} {
		if !found[slug] {
			t.Fatalf("missing %s", slug)
		}
	}
	if !found["gemini-3.1-flash-image"] {
		t.Fatal("registered flash image missing")
	}
}
