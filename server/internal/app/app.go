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
	"local_config_center/internal/rewrite"
	"local_config_center/internal/storage"
	"local_config_center/internal/testhub"
)

type Settings struct {
	DatabaseURL string
	HTTPAddr    string
	WebDir      string
	ArtifactDir string
	GatewayAuth string
}

func SettingsFromEnvironment() (Settings, error) {
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return Settings{}, fmt.Errorf("DATABASE_URL is required")
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
	return Settings{DatabaseURL: databaseURL, HTTPAddr: address, WebDir: webDir, ArtifactDir: artifactDir, GatewayAuth: gatewayAuth}, nil
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
	handler := httpapi.New(service, hub, settings.WebDir, settings.GatewayAuth)

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
