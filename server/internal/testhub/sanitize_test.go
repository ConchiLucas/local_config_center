package testhub

import "testing"

func TestSanitizeRedactsSecrets(t *testing.T) {
	in := "Bearer super-secret-token password=hunter2 api_key=abcd sk-abcdefghijklmnop"
	out := Sanitize(in)
	if containsAny(out, "super-secret-token", "hunter2", "abcd", "sk-abcdefghijklmnop") {
		t.Fatalf("leaked secret: %s", out)
	}
}

func containsAny(s string, parts ...string) bool {
	for _, p := range parts {
		if len(p) > 0 && len(s) > 0 {
			for i := 0; i+len(p) <= len(s); i++ {
				if s[i:i+len(p)] == p {
					return true
				}
			}
		}
	}
	return false
}
