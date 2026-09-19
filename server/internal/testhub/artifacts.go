package testhub

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

type Artifact struct {
	ID         string
	MIME       string
	Filename   string
	Size       int64
	Width      int
	Height     int
	DurationMs int
	Path       string
}

type ArtifactStore struct {
	dir   string
	mu    sync.Mutex
	items map[string]Artifact
}

func NewArtifactStore(dir string) (*ArtifactStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &ArtifactStore{dir: dir, items: map[string]Artifact{}}, nil
}

func (s *ArtifactStore) Save(filename, mime string, data []byte) (Artifact, error) {
	if mime == "" {
		mime = http.DetectContentType(data)
	}
	id := newAssetID()
	ext := extensionFor(mime, filename)
	path := filepath.Join(s.dir, id+ext)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return Artifact{}, err
	}
	width, height := imageSize(data, mime)
	art := Artifact{
		ID:       id,
		MIME:     mime,
		Filename: filename,
		Size:     int64(len(data)),
		Width:    width,
		Height:   height,
		Path:     path,
	}
	s.mu.Lock()
	s.items[id] = art
	s.mu.Unlock()
	return art, nil
}

func (s *ArtifactStore) Get(id string) (Artifact, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	art, ok := s.items[id]
	return art, ok
}

func (a Artifact) Ref() ArtifactRef {
	return ArtifactRef{
		AssetID:    a.ID,
		MimeType:   a.MIME,
		Filename:   a.Filename,
		SizeBytes:  a.Size,
		Width:      a.Width,
		Height:     a.Height,
		DurationMs: a.DurationMs,
		Access: map[string]any{
			"type": "same_origin",
			"path": "/api/media/" + a.ID,
		},
	}
}

func newAssetID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", os.Getpid())
	}
	return hex.EncodeToString(b[:])
}

func extensionFor(mime, filename string) string {
	switch mime {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "audio/mpeg":
		return ".mp3"
	case "audio/wav", "audio/x-wav", "audio/wave":
		return ".wav"
	case "audio/ogg":
		return ".ogg"
	case "audio/mp4", "audio/aac":
		return ".m4a"
	case "video/mp4":
		return ".mp4"
	case "video/webm":
		return ".webm"
	}
	if ext := filepath.Ext(filename); ext != "" {
		return ext
	}
	return ".bin"
}

func imageSize(data []byte, mime string) (int, int) {
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
		return cfg.Width, cfg.Height
	}
	if mime == "image/webp" || (len(data) > 30 && bytes.HasPrefix(data, []byte("RIFF")) && bytes.Contains(data[8:12], []byte("WEBP"))) {
		return webpSize(data)
	}
	return 0, 0
}

func webpSize(data []byte) (int, int) {
	if len(data) < 30 {
		return 0, 0
	}
	if bytes.HasPrefix(data[12:], []byte("VP8X")) && len(data) >= 30 {
		w := 1 + int(uint32(data[24])|uint32(data[25])<<8|uint32(data[26])<<16)
		h := 1 + int(uint32(data[27])|uint32(data[28])<<8|uint32(data[29])<<16)
		return w, h
	}
	if bytes.HasPrefix(data[12:], []byte("VP8 ")) && len(data) >= 30 {
		raw := data[26:]
		if len(raw) >= 10 && raw[0] == 0x9d && raw[1] == 0x01 && raw[2] == 0x2a {
			w := int(binary.LittleEndian.Uint16(raw[6:8])) & 0x3fff
			h := int(binary.LittleEndian.Uint16(raw[8:10])) & 0x3fff
			return w, h
		}
	}
	return 0, 0
}

func sniffAudioVideo(mime string) bool {
	return len(mime) >= 5 && (mime[:5] == "audio" || mime[:5] == "video" || (len(mime) >= 6 && mime[:6] == "audio/") || (len(mime) >= 6 && mime[:6] == "video/"))
}

func isPreviewable(mime string) bool {
	if mime == "image/svg+xml" {
		return false
	}
	return mime == "image/png" || mime == "image/jpeg" || mime == "image/webp" || mime == "image/gif" ||
		mime == "audio/mpeg" || mime == "audio/wav" || mime == "audio/x-wav" || mime == "audio/wave" || mime == "audio/ogg" || mime == "audio/mp4" ||
		mime == "video/mp4" || mime == "video/webm" || sniffAudioVideo(mime)
}

func guessMIME(name string, data []byte) string {
	detected := http.DetectContentType(data)
	if detected != "application/octet-stream" {
		return detected
	}
	switch filepath.Ext(name) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	default:
		return detected
	}
}

func formatBytes(n int64) string {
	return fmt.Sprintf("%d", n)
}
