package testhub

import (
	"net/url"
	"regexp"
	"strings"
)

var (
	bearerRE = regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9._\-+=/]+`)
	keyRE    = regexp.MustCompile(`(?i)(api[_-]?key|secret|password|token)([=:\s]+)([^\s"'\\]+)`)
	skRE     = regexp.MustCompile(`\bsk-[A-Za-z0-9]{8,}\b`)
)

func Sanitize(message string) string {
	out := bearerRE.ReplaceAllString(message, "Bearer [redacted]")
	out = skRE.ReplaceAllString(out, "sk-[redacted]")
	out = keyRE.ReplaceAllString(out, "$1$2[redacted]")
	if parsed, err := url.Parse(out); err == nil && parsed.User != nil {
		parsed.User = url.UserPassword(parsed.User.Username(), "[redacted]")
		out = parsed.String()
	}
	return strings.TrimSpace(out)
}

func Clamp(text string, max int) string {
	if max <= 0 || len(text) <= max {
		return text
	}
	return text[:max]
}
