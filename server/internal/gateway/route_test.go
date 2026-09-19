package gateway

import "testing"

func TestMatchAuth(t *testing.T) {
	if !Match("local-gateway", "local-gateway") {
		t.Fatal("expected match")
	}
	if Match("nope", "local-gateway") {
		t.Fatal("expected reject")
	}
	if Match("", "local-gateway") {
		t.Fatal("empty auth must fail")
	}
}

func TestSplitAndCompose(t *testing.T) {
	v, s := SplitModel("antigravity/gemini-3.8-flash-high")
	if v != "antigravity" || s != "gemini-3.8-flash-high" {
		t.Fatalf("%s %s", v, s)
	}
	v, s = SplitModel("xiaomi/mimo-v2.5-tts")
	if v != "mimo" || s != "mimo-v2.5-tts" {
		t.Fatalf("%s %s", v, s)
	}
	got := ComposeImageSlug("antigravity", "gemini-3-pro-image", "2k", "16x9")
	if got != "gemini-3-pro-image-2k-16x9" {
		t.Fatalf("compose = %s", got)
	}
	if ConfigID("ag", "chat") != "ai:antigravity:chat" {
		t.Fatalf("id = %s", ConfigID("ag", "chat"))
	}
}
