package testhub

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

type Delivery string
type ContentFormat string
type RunState string

const (
	DeliveryStream   Delivery = "stream"
	DeliveryComplete Delivery = "complete"
	FormatText       ContentFormat = "text"
	FormatJSON       ContentFormat = "json"

	StateSubmitting RunState = "submitting"
	StateQueued     RunState = "queued"
	StateRunning    RunState = "running"
	StateSucceeded  RunState = "succeeded"
	StateFailed     RunState = "failed"
	StateCancelled  RunState = "cancelled"
)

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type TestInput struct {
	Kind              string        `json:"kind"`
	Model             string        `json:"model,omitempty"`
	Messages          []ChatMessage `json:"messages,omitempty"`
	Prompt            string        `json:"prompt,omitempty"`
	Text              string        `json:"text,omitempty"`
	ProbeID           string        `json:"probeId,omitempty"`
	ObjectKey         string        `json:"objectKey,omitempty"`
	Prefix            string        `json:"prefix,omitempty"`
	ReferenceAssetIDs []string      `json:"referenceAssetIds,omitempty"`
	AudioBase64       string        `json:"audioBase64,omitempty"`
	Filename          string        `json:"filename,omitempty"`
	Voice             string        `json:"voice,omitempty"`
	Resolution        string        `json:"resolution,omitempty"`
	Aspect            string        `json:"aspect,omitempty"`
}

type OutputPreset struct {
	Delivery Delivery      `json:"delivery"`
	Format   ContentFormat `json:"format"`
}

type CreateTestRequest struct {
	ConfigID         string        `json:"configId"`
	ExpectedRevision string        `json:"expectedRevision"`
	IdempotencyKey   string        `json:"idempotencyKey"`
	Input            TestInput     `json:"input"`
	OutputPreset     *OutputPreset `json:"outputPreset,omitempty"`
}

type ArtifactRef struct {
	AssetID    string         `json:"assetId"`
	MimeType   string         `json:"mimeType"`
	Filename   string         `json:"filename,omitempty"`
	SizeBytes  int64          `json:"sizeBytes,omitempty"`
	Width      int            `json:"width,omitempty"`
	Height     int            `json:"height,omitempty"`
	DurationMs int            `json:"durationMs,omitempty"`
	Access     map[string]any `json:"access"`
}

type OutputBlock struct {
	ID         string           `json:"id"`
	Kind       string           `json:"kind"`
	Text       string           `json:"text,omitempty"`
	Complete   bool             `json:"complete,omitempty"`
	RawText    string           `json:"rawText,omitempty"`
	Parsed     json.RawMessage  `json:"parsed,omitempty"`
	ValidJSON  *bool            `json:"validJson,omitempty"`
	Assets     []ArtifactRef    `json:"assets,omitempty"`
	Asset      *ArtifactRef     `json:"asset,omitempty"`
	Poster     *ArtifactRef     `json:"poster,omitempty"`
	Transcript string           `json:"transcript,omitempty"`
	Columns    []string         `json:"columns,omitempty"`
	Rows       [][]any          `json:"rows,omitempty"`
	Checks     []DiagnosticItem `json:"checks,omitempty"`
}

type DiagnosticItem struct {
	Label  string `json:"label"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

type RunError struct {
	Code        string `json:"code"`
	SafeMessage string `json:"safeMessage"`
	RequestID   string `json:"requestId"`
}

type Metrics struct {
	ElapsedMs    int64 `json:"elapsedMs,omitempty"`
	FirstTokenMs int64 `json:"firstTokenMs,omitempty"`
}

type TestRun struct {
	ID             string        `json:"id"`
	ConfigID       string        `json:"configId"`
	ConfigRevision string        `json:"configRevision"`
	State          RunState      `json:"state"`
	StageLabel     string        `json:"stageLabel,omitempty"`
	Outputs        []OutputBlock `json:"outputs"`
	Metrics        *Metrics      `json:"metrics,omitempty"`
	Cancellation   string        `json:"cancellation,omitempty"`
	Error          *RunError     `json:"error,omitempty"`
	CreatedAt      time.Time     `json:"createdAt"`
	UpdatedAt      time.Time     `json:"updatedAt"`
}

type RunEvent struct {
	ID          string       `json:"id"`
	Type        string       `json:"type"`
	RunID       string       `json:"runId,omitempty"`
	State       RunState     `json:"state,omitempty"`
	StageLabel  string       `json:"stageLabel,omitempty"`
	BlockID     string       `json:"blockId,omitempty"`
	Delta       string       `json:"delta,omitempty"`
	Block       *OutputBlock `json:"block,omitempty"`
	Run         *TestRun     `json:"run,omitempty"`
	Code        string       `json:"code,omitempty"`
	SafeMessage string       `json:"safeMessage,omitempty"`
	RequestID   string       `json:"requestId,omitempty"`
}

type LiveRun = liveRun

type liveRun struct {
	mu        sync.Mutex
	run       TestRun
	events    []RunEvent
	waiters   []chan RunEvent
	cancel    context.CancelFunc
	done      chan struct{}
	startedAt time.Time
}
