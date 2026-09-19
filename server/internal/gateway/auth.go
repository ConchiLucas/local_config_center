package gateway

import (
	"crypto/subtle"
	"os"
	"strings"
)

func TokenFromEnv() string {
	token := strings.TrimSpace(os.Getenv("GATEWAY_AUTH"))
	if token == "" {
		return "local-gateway"
	}
	return token
}

func Match(got, want string) bool {
	got = strings.TrimSpace(got)
	want = strings.TrimSpace(want)
	if want == "" || got == "" {
		return false
	}
	if len(got) != len(want) {
		_ = subtle.ConstantTimeCompare([]byte(want), []byte(want))
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}
