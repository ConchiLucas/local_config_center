package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"local_config_center/internal/imagejobs"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestImageJobsRequireBearerBeforeStorage(t *testing.T) {
	h := New(nil, nil, "", "test-secret")
	for _, path := range []string{"/v1/image-jobs", "/v1/image-jobs/job-x", "/v1/image-jobs/by-key/key", "/v1/image-jobs/job-x/image", "/v1/image-jobs/capabilities"} {
		method := http.MethodGet
		if path == "/v1/image-jobs" {
			method = http.MethodPost
		}
		for _, auth := range []string{"", "Bearer wrong", "Basic test-secret"} {
			r := httptest.NewRequest(method, path, nil)
			r.Header.Set("Authorization", auth)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != 401 {
				t.Errorf("%s auth=%q: got %d", path, auth, w.Code)
			}
		}
	}
}

func TestImageJobHTTPLifecycle(t *testing.T) {
	url := os.Getenv("IMAGE_JOBS_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("requires dedicated PG database")
	}
	ctx := context.Background()
	open := func() *imagejobs.Store {
		t.Helper()
		var s *imagejobs.Store
		var err error
		for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
			s, err = imagejobs.Open(ctx, url)
			if err == nil {
				return s
			}
			time.Sleep(100 * time.Millisecond)
		}
		t.Fatal(err)
		return nil
	}
	store := open()
	defer store.Close()
	handler := NewWithImageJobs(nil, nil, "", "test-secret", store, false)
	call := func(method, path, key, payload string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(payload))
		r.Header.Set("Authorization", "Bearer test-secret")
		r.Header.Set("Idempotency-Key", key)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	cap := call("GET", "/v1/image-jobs/capabilities", "", "")
	if cap.Code != 200 || !strings.Contains(cap.Body.String(), `"real_enabled":false`) {
		t.Fatal(cap.Body.String())
	}
	key := fmt.Sprintf("http-%d", time.Now().UnixNano())
	for _, payload := range []string{`{"model":"mock/image","prompt":"hello","parameters":{"size":"x"}}`, `{"model":"mock/image","prompt":"hello","parameters":null}`, `{"model":"mock/image","prompt":"hello","parameters":{}} {}`} {
		if got := call("POST", "/v1/image-jobs", key, payload); got.Code != 400 {
			t.Fatalf("invalid accepted: %d", got.Code)
		}
	}
	if got := call("POST", "/v1/image-jobs", key, `{"model":"real/image","prompt":"hello","parameters":{}}`); got.Code != 403 {
		t.Fatalf("real gate: %d", got.Code)
	}
	payload := `{"model":"mock/image","prompt":"hello","parameters":{}}`
	accepted := call("POST", "/v1/image-jobs", key, payload)
	if accepted.Code != 202 {
		t.Fatalf("submit %d %s", accepted.Code, accepted.Body.String())
	}
	var envelope struct {
		Job imagejobs.Job `json:"job"`
	}
	if err := json.Unmarshal(accepted.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	id := envelope.Job.ID
	duplicate := call("POST", "/v1/image-jobs", key, payload)
	if duplicate.Code != 202 || !strings.Contains(duplicate.Body.String(), id) {
		t.Fatal("duplicate lost")
	}
	if got := call("POST", "/v1/image-jobs", key, strings.Replace(payload, "hello", "different", 1)); got.Code != 409 {
		t.Fatal("conflict missing")
	}
	for _, tc := range []struct {
		name, prompt string
		want         int
	}{{"max-unicode", strings.Repeat("😀", 20000), 202}, {"over-unicode", strings.Repeat("界", 20001), 400}} {
		body, _ := json.Marshal(imagejobs.Request{Model: "mock/image", Prompt: tc.prompt, Parameters: map[string]any{}})
		if got := call("POST", "/v1/image-jobs", key+tc.name, string(body)); got.Code != tc.want {
			t.Errorf("%s got %d want %d", tc.name, got.Code, tc.want)
		}
	}
	for _, size := range []int{128 << 10, (128 << 10) + 1} {
		body := payload + strings.Repeat(" ", size-len(payload))
		want := 202
		if size > 128<<10 {
			want = 400
		}
		if got := call("POST", "/v1/image-jobs", fmt.Sprintf("%s-body-%d", key, size), body); got.Code != want {
			t.Errorf("body bytes %d got %d want %d", size, got.Code, want)
		}
	}
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- store.Run(runCtx, ImageExecutor(nil, nil, false)) }()
	defer func() { cancel() }()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		result := call("GET", "/v1/image-jobs/"+id, "", "")
		json.Unmarshal(result.Body.Bytes(), &envelope)
		if envelope.Job.State == "succeeded" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if envelope.Job.State != "succeeded" || envelope.Job.BilledAmount == nil || *envelope.Job.BilledAmount != "0" {
		cancel()
		<-done
		t.Fatalf("mock result %#v", envelope.Job)
	}
	media := call("GET", envelope.Job.Result.Path, "", "")
	if media.Code != 200 || media.Header().Get("Content-Type") != "image/png" {
		t.Fatal("media unavailable")
	}
	lookup := call("GET", "/v1/image-jobs/by-key/"+key, "", "")
	if lookup.Code != 200 || !strings.Contains(lookup.Body.String(), id) {
		t.Fatal("lookup failed")
	}
	cancel()
	<-done
	store.Close()
	restarted := open()
	defer restarted.Close()
	handler = NewWithImageJobs(nil, nil, "", "test-secret", restarted, false)
	restored := call("GET", envelope.Job.Result.Path, "", "")
	if restored.Code != 200 || !bytes.Equal(restored.Body.Bytes(), media.Body.Bytes()) {
		t.Fatal("media lost after restart")
	}
	if got := call("GET", "/v1/image-jobs/missing", "", ""); got.Code != 404 {
		t.Fatal("missing job status")
	}
}

func TestDurableImageMediaBoundary(t *testing.T) {
	pngBytes, _, _ := imagejobs.Mock(context.Background(), imagejobs.Request{Prompt: "test"})
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.White)
	var gifBytes, jpegBytes bytes.Buffer
	if err := gif.Encode(&gifBytes, img, nil); err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(&jpegBytes, img, nil); err != nil {
		t.Fatal(err)
	}
	webp := append([]byte("RIFF"), make([]byte, 4)...)
	webp = append(webp, []byte("WEBPVP8 ")...)
	webp = append(webp, make([]byte, 14)...)
	for _, tc := range []struct {
		name string
		data []byte
		want string
	}{{"PNG", pngBytes, "image/png"}, {"JPEG", jpegBytes.Bytes(), "image/jpeg"}, {"WebP", webp, "image/webp"}, {"GIF", gifBytes.Bytes(), ""}, {"max-bytes", append(append([]byte{}, pngBytes...), make([]byte, (20<<20)-len(pngBytes))...), "image/png"}, {"over-bytes", append(append([]byte{}, pngBytes...), make([]byte, (20<<20)+1-len(pngBytes))...), ""}} {
		t.Run(tc.name, func(t *testing.T) {
			data, mime, err := readJobImage(bytes.NewReader(tc.data))
			if tc.want == "" {
				if err == nil {
					t.Fatal("unsupported image accepted")
				}
				return
			}
			if err != nil || mime != tc.want || !bytes.Equal(data, tc.data) {
				t.Fatalf("image %s %v", mime, err)
			}
		})
	}
}
