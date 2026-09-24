package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"local_config_center/internal/config"
	"local_config_center/internal/httpapi"
	"local_config_center/internal/imagejobs"
	"local_config_center/internal/rewrite"
	"local_config_center/internal/storage"
	"local_config_center/internal/testhub"
)

type Settings struct {
	DatabaseURL      string
	HTTPAddr         string
	WebDir           string
	ArtifactDir      string
	GatewayAuth      string
	ImageRealEnabled bool
}

func SettingsFromEnvironment() (Settings, error) {
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		databaseURL = "postgres://shared_config:shared_config_internal@127.0.0.1:5432/shared_config?sslmode=disable"
	}
	address := strings.TrimSpace(os.Getenv("HTTP_ADDR"))
	if address == "" {
		address = "127.0.0.1:18527"
	}
	webDir := strings.TrimSpace(os.Getenv("WEB_DIR"))
	if webDir == "" {
		webDir = defaultWebDir()
	}
	artifactDir := strings.TrimSpace(os.Getenv("ARTIFACT_DIR"))
	if artifactDir == "" {
		artifactDir = filepath.Join("..", "data", "artifacts")
	}
	gatewayAuth := strings.TrimSpace(os.Getenv("GATEWAY_AUTH"))
	if gatewayAuth == "" {
		gatewayAuth = "local-gateway"
	}
	return Settings{DatabaseURL: databaseURL, HTTPAddr: address, WebDir: webDir, ArtifactDir: artifactDir, GatewayAuth: gatewayAuth, ImageRealEnabled: strings.EqualFold(strings.TrimSpace(os.Getenv("GATEWAY_IMAGE_REAL_ENABLED")), "true")}, nil
}

func defaultWebDir() string {
	candidates := []string{"web", "../web", filepath.Join("..", "web")}
	for _, candidate := range candidates {
		if info, err := os.Stat(filepath.Join(candidate, "index.html")); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return "../web"
}

func Run(ctx context.Context, settings Settings, logger *slog.Logger) error {
	repository, err := storage.OpenPostgres(ctx, settings.DatabaseURL)
	if err != nil {
		return err
	}
	defer repository.Close()

	artifacts, err := testhub.NewArtifactStore(settings.ArtifactDir)
	if err != nil {
		return fmt.Errorf("artifact store: %w", err)
	}
	service := config.NewService(repository)
	hub := testhub.NewHub(service, artifacts, rewrite.NewHostSide())
	jobs, err := imagejobs.Open(ctx, settings.DatabaseURL)
	if err != nil {
		return fmt.Errorf("image job store initialization failed: %w", err)
	}
	defer jobs.Close()
	workerCtx, stopWorker := context.WithCancel(ctx)
	workerDone := make(chan error, 1)
	go func() {
		workerDone <- jobs.Run(workerCtx, httpapi.ImageExecutor(service, hub, settings.ImageRealEnabled))
	}()
	workerFinished := false
	defer func() {
		stopWorker()
		if !workerFinished {
			<-workerDone
		}
	}()
	handler := httpapi.NewWithImageJobs(service, hub, settings.WebDir, settings.GatewayAuth, jobs, settings.ImageRealEnabled)

	server := &http.Server{
		Addr:              settings.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 8 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      10 * time.Minute,
		IdleTimeout:       60 * time.Second,
	}
	result := make(chan error, 1)
	go func() {
		logger.Info("config workbench listening", "address", settings.HTTPAddr, "web", settings.WebDir)
		result <- server.ListenAndServe()
	}()
	select {
	case err := <-workerDone:
		workerFinished = true
		_ = server.Close()
		return fmt.Errorf("image job executor stopped: %w", err)
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdown)
	case err := <-result:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}
