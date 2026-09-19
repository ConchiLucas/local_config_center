package testhub

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func (h *Hub) runStorage(ctx context.Context, live *liveRun, binding Binding, req CreateTestRequest) error {
	storage := binding.Storage
	if storage == nil {
		return fmt.Errorf("缺少对象存储配置")
	}
	if strings.TrimSpace(storage.AccessKeyID) == "" || strings.TrimSpace(storage.SecretAccessKey) == "" {
		return APIError{Status: 400, Code: "MISSING_CREDENTIAL", Message: "对象存储凭据未配置"}
	}
	endpoint := h.rewriter.ApplyEndpoint(storage.Endpoint)
	secure := storage.UseSSL
	if strings.HasPrefix(endpoint, "https://") {
		secure = true
		endpoint = strings.TrimPrefix(endpoint, "https://")
	} else if strings.HasPrefix(endpoint, "http://") {
		secure = false
		endpoint = strings.TrimPrefix(endpoint, "http://")
	}
	live.setStage("正在列举对象")
	h.emit(live, RunEvent{Type: "status", RunID: live.run.ID, State: StateRunning, StageLabel: "正在列举对象"})
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(storage.AccessKeyID, storage.SecretAccessKey, ""),
		Secure: secure,
	})
	if err != nil {
		return fmt.Errorf("无法创建存储客户端: %s", Sanitize(err.Error()))
	}
	prefix := strings.TrimSpace(req.Input.Prefix)
	if prefix == "" {
		prefix = strings.TrimSpace(req.Input.ObjectKey)
	}
	if prefix == "" {
		prefix = strings.Trim(storage.BasePath, "/")
	}
	if prefix != "" && !strings.HasSuffix(prefix, "/") && strings.TrimSpace(req.Input.ObjectKey) == "" {
		prefix += "/"
	}
	if base := strings.Trim(storage.BasePath, "/"); base != "" {
		if prefix == "" {
			prefix = base + "/"
		} else if !strings.HasPrefix(strings.Trim(prefix, "/"), base) {
			return APIError{Status: 400, Code: "INVALID_INPUT", Message: "只能访问已登记前缀下的对象"}
		}
	}
	if strings.Contains(prefix, "..") {
		return APIError{Status: 400, Code: "INVALID_INPUT", Message: "对象键不合法"}
	}

	listCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	objects := client.ListObjects(listCtx, storage.BucketName, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})
	type listed struct {
		Key  string
		Size int64
		MIME string
	}
	items := make([]listed, 0, 20)
	for object := range objects {
		if object.Err != nil {
			return fmt.Errorf("列举对象失败: %s", Sanitize(object.Err.Error()))
		}
		if object.Key == "" || strings.HasSuffix(object.Key, "/") {
			continue
		}
		items = append(items, listed{Key: object.Key, Size: object.Size, MIME: object.ContentType})
		if len(items) >= 12 {
			break
		}
	}

	rows := make([][]any, 0, len(items))
	previews := make([]OutputBlock, 0)
	for i, item := range items {
		rows = append(rows, []any{item.Key, fmt.Sprintf("%d bytes", item.Size)})
		if binding.Kind != "media" && i >= 2 {
			continue
		}
		getCtx, getCancel := context.WithTimeout(ctx, 15*time.Second)
		obj, err := client.GetObject(getCtx, storage.BucketName, item.Key, minio.GetObjectOptions{})
		if err != nil {
			getCancel()
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(obj, 8<<20))
		_ = obj.Close()
		getCancel()
		if readErr != nil && len(data) == 0 {
			continue
		}
		if len(data) == 0 {
			continue
		}
		mime := item.MIME
		if mime == "" {
			mime = guessMIME(item.Key, data)
		}
		if !isPreviewable(mime) {
			continue
		}
		art, err := h.artifacts.Save(path.Base(item.Key), mime, data)
		if err != nil {
			continue
		}
		ref := art.Ref()
		kind := "image"
		if strings.HasPrefix(mime, "audio/") {
			kind = "audio"
			block := OutputBlock{ID: fmt.Sprintf("obj-audio-%d", i), Kind: "audio", Asset: &ref, Transcript: item.Key}
			previews = append(previews, block)
			continue
		}
		if strings.HasPrefix(mime, "video/") {
			kind = "video"
			block := OutputBlock{ID: fmt.Sprintf("obj-video-%d", i), Kind: "video", Asset: &ref}
			previews = append(previews, block)
			continue
		}
		_ = kind
		block := OutputBlock{ID: fmt.Sprintf("obj-image-%d", i), Kind: "image", Assets: []ArtifactRef{ref}}
		previews = append(previews, block)
	}

	table := OutputBlock{
		ID:      "storage-table",
		Kind:    "table",
		Columns: []string{"对象", "大小"},
		Rows:    rows,
	}
	if len(rows) == 0 {
		table.Rows = [][]any{{"（无对象）", prefix}}
	}
	live.setOutput(table)
	h.emit(live, RunEvent{Type: "output", RunID: live.run.ID, Block: &table})
	for i := range previews {
		block := previews[i]
		live.setOutput(block)
		h.emit(live, RunEvent{Type: "output", RunID: live.run.ID, Block: &block})
	}
	return nil
}
