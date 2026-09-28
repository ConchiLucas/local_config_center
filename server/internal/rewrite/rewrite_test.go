package rewrite

import "testing"

func TestHostSideMapsSharedDockerServices(t *testing.T) {
	h := HostSide{Enabled: true}
	tests := map[string]string{
		"http://grok2api:8000/v1": "http://127.0.0.1:37821/v1",
		"http://sub2api:8090/v1":  "http://127.0.0.1:8090/v1",
		"http://minio:9000/bucket": "http://127.0.0.1:19100/bucket",
	}
	for input, want := range tests {
		if got := h.ApplyURL(input); got != want {
			t.Errorf("ApplyURL(%q) = %q, want %q", input, got, want)
		}
	}
}
