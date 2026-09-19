package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"local_config_center/internal/testhub"
)

const maxRequestBody = 12 << 20

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeProblem(w http.ResponseWriter, status int, code, detail string) {
	writeJSON(w, status, map[string]any{
		"type":   "https://config-center.local/problems/" + code,
		"title":  code,
		"status": status,
		"detail": detail,
		"code":   code,
	})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			return fmt.Errorf("request body too large")
		}
		return fmt.Errorf("invalid JSON: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("request body must contain exactly one JSON value")
	}
	return nil
}

func writeAPIError(w http.ResponseWriter, err error) {
	if api, ok := err.(testhub.APIError); ok {
		writeProblem(w, api.Status, api.Code, api.Message)
		return
	}
	writeProblem(w, http.StatusInternalServerError, "INTERNAL", testhub.Sanitize(err.Error()))
}
