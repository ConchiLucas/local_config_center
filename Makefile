.PHONY: test run docker-bin

-include .env
export DATABASE_URL
HTTP_ADDR ?= 127.0.0.1:18527
GOOS_DOCKER ?= linux
GOARCH_DOCKER ?= arm64

test:
	cd server && go test ./...
	cd server && go vet ./...

GATEWAY_AUTH ?= local-gateway

run:
	cd server && DATABASE_URL='$(DATABASE_URL)' HTTP_ADDR='$(HTTP_ADDR)' WEB_DIR='../web' ARTIFACT_DIR='../data/artifacts' GATEWAY_AUTH='$(GATEWAY_AUTH)' go run ./cmd/workbench

docker-bin:
	cd server && CGO_ENABLED=0 GOOS='$(GOOS_DOCKER)' GOARCH='$(GOARCH_DOCKER)' go build -o bin/workbench ./cmd/workbench
