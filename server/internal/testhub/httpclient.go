package testhub

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"local_config_center/internal/rewrite"
)

func newHTTPClient(timeout time.Duration) *http.Client {
	headerTimeout := timeout
	if headerTimeout <= 0 || headerTimeout > 60*time.Second {
		headerTimeout = 60 * time.Second
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:                 nil,
			DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          20,
			IdleConnTimeout:       30 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: headerTimeout,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
}

func joinURL(base, path string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	path = "/" + strings.TrimLeft(path, "/")
	if strings.HasSuffix(base, "/v1") && strings.HasPrefix(path, "/v1/") {
		path = strings.TrimPrefix(path, "/v1")
	}
	return base + path
}

func doJSON(ctx context.Context, client *http.Client, method, rawURL, apiKey string, body any, dest any) (*http.Response, []byte, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, nil, err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, reader)
	if err != nil {
		return nil, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(apiKey) != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("request failed: %s", Sanitize(err.Error()))
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	resp.Body.Close()
	if err != nil {
		return resp, nil, err
	}
	if dest != nil && len(data) > 0 && json.Valid(data) {
		_ = json.Unmarshal(data, dest)
	}
	return resp, data, nil
}

func doStream(ctx context.Context, client *http.Client, rawURL, apiKey string, body any) (*http.Response, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if strings.TrimSpace(apiKey) != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %s", Sanitize(err.Error()))
	}
	return resp, nil
}

func rewriteURL(rewriter rewrite.HostSide, raw string) string {
	return rewriter.ApplyURL(raw)
}

func hostAllowed(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return fmt.Errorf("invalid destination")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("unsupported protocol")
	}
	host := parsed.Hostname()
	if host == "169.254.169.254" || host == "metadata.google.internal" || host == "metadata.google.internal." {
		return fmt.Errorf("destination is not allowed")
	}
	return nil
}

func truncateBody(data []byte) string {
	text := strings.TrimSpace(string(data))
	text = Sanitize(text)
	if len(text) > 400 {
		return text[:400] + "…"
	}
	return text
}

func httpStatusError(status int, body []byte) error {
	return fmt.Errorf("upstream HTTP %d: %s", status, truncateBody(body))
}
