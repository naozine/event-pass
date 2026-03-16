# -----------------------------------------------------------------------------
# Configuration
# -----------------------------------------------------------------------------
# プロジェクト固有の設定を読み込む（存在する場合）
-include deploy.config

BUILD_DIR   ?= bin
CMD_PATH    ?= ./cmd/server

# Project Name (フォルダ名から自動取得、_ は - に変換)
# Docker イメージ名、コンテナ名、fly.io app名、クッキー名などに使用
PROJECT_NAME := $(subst _,-,$(notdir $(CURDIR)))

# Version Info (git から自動取得)
VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT     := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS     = -X 'github.com/naozine/project_crud_with_auth_tmpl/internal/version.Version=$(VERSION)' \
              -X 'github.com/naozine/project_crud_with_auth_tmpl/internal/version.Commit=$(COMMIT)' \
              -X 'github.com/naozine/project_crud_with_auth_tmpl/internal/version.BuildDate=$(BUILD_DATE)' \
              -X 'github.com/naozine/project_crud_with_auth_tmpl/internal/version.ProjectName=$(PROJECT_NAME)' \
              -X 'github.com/naozine/project_crud_with_auth_tmpl/internal/version.ServerAddr=$(SERVER_ADDR)'

# Server Configuration（ローカル開発時は http://localhost:8080）
SERVER_ADDR   ?= http://localhost:8080

# Docker Settings (デフォルトはフォルダ名から自動生成)
IMAGE_NAME    ?= $(PROJECT_NAME)
IMAGE_TAG     ?= $(VERSION)

# -----------------------------------------------------------------------------
# Local Development Targets
# -----------------------------------------------------------------------------
.PHONY: build generate dev-build migrate-new \
        docker-build docker-push docker-up docker-down docker-logs docker-dev docker-dev-down

# Generate all auto-generated code (sqlc, templ, tailwind)
generate:
	@echo ">> Generating code..."
	sqlc generate
	templ generate
	tailwindcss -i web/static/css/input.css -o web/static/css/style.css --minify

# Local build (開発・テスト用)
build: generate
	@echo ">> Building $(VERSION)..."
	go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/server $(CMD_PATH)

# Air 用ビルド (ホットリロード開発用)
# generate は Air の include_ext で監視しているため省略
dev-build:
	go build -ldflags "$(LDFLAGS)" -o ./tmp/main $(CMD_PATH)

# Utility: Create New Migration
# Usage: make migrate-new NAME=add_users_table
migrate-new:
	@if [ -z "$(NAME)" ]; then echo "Usage: make migrate-new NAME=description"; exit 1; fi
	go run github.com/pressly/goose/v3/cmd/goose -dir db/migrations create $(NAME) sql

# -----------------------------------------------------------------------------
# Docker Targets (ローカル)
# -----------------------------------------------------------------------------

# Build Docker image for linux/amd64 (VPS deployment)
# M4 Mac (arm64) から x86 VPS へのデプロイに対応
# 方法: VPS上でビルドするか、マルチアーキテクチャ対応の場合は buildx を使用
docker-build: generate
	@echo ">> Building Docker image $(IMAGE_NAME):$(IMAGE_TAG)..."
	@if docker buildx version >/dev/null 2>&1; then \
		echo "Using buildx for cross-platform build (linux/amd64)..."; \
		docker buildx build \
			--platform linux/amd64 \
			--build-arg VERSION=$(VERSION) \
			--build-arg COMMIT=$(COMMIT) \
			--build-arg BUILD_DATE=$(BUILD_DATE) \
			--build-arg PROJECT_NAME=$(PROJECT_NAME) \
			-t $(IMAGE_NAME):$(IMAGE_TAG) \
			-t $(IMAGE_NAME):latest \
			--load \
			.; \
	else \
		echo "buildx not available, building for local architecture..."; \
		echo "Note: VPS上でイメージをビルドするか、buildxをインストールしてください"; \
		docker build \
			--build-arg VERSION=$(VERSION) \
			--build-arg COMMIT=$(COMMIT) \
			--build-arg BUILD_DATE=$(BUILD_DATE) \
			--build-arg PROJECT_NAME=$(PROJECT_NAME) \
			-t $(IMAGE_NAME):$(IMAGE_TAG) \
			-t $(IMAGE_NAME):latest \
			.; \
	fi

# Push Docker image to registry (要: DOCKER_REGISTRY 設定)
docker-push: docker-build
	@if [ -z "$(DOCKER_REGISTRY)" ]; then \
		echo "Error: DOCKER_REGISTRY is not set"; \
		exit 1; \
	fi
	@echo ">> Pushing to $(DOCKER_REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)..."
	docker tag $(IMAGE_NAME):$(IMAGE_TAG) $(DOCKER_REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)
	docker tag $(IMAGE_NAME):latest $(DOCKER_REGISTRY)/$(IMAGE_NAME):latest
	docker push $(DOCKER_REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)
	docker push $(DOCKER_REGISTRY)/$(IMAGE_NAME):latest

# Start production containers
docker-up:
	@echo ">> Starting containers..."
	docker compose up -d

# Stop production containers
docker-down:
	@echo ">> Stopping containers..."
	docker compose down

# View container logs
docker-logs:
	docker compose logs -f

# Start development environment with hot reload
docker-dev:
	@echo ">> Starting development environment..."
	docker compose -f docker-compose.dev.yaml up --build

# Stop development environment
docker-dev-down:
	docker compose -f docker-compose.dev.yaml down

# -----------------------------------------------------------------------------
# Litestream Targets (ローカルから R2 レプリカを操作)
# -----------------------------------------------------------------------------
# .env.production から Litestream 関連の環境変数を読み込む
# LITESTREAM_BUCKET 未設定時は <プロジェクト名>-litestream をデフォルト使用
LITESTREAM_BUCKET_NAME ?= $(PROJECT_NAME)-litestream
LITESTREAM_ENV = $(shell grep '^LITESTREAM_' .env.production 2>/dev/null)

.PHONY: ls-restore ls-restore-timestamp ls-apply

# R2 から最新状態をローカルにリストア
# Usage: make ls-restore
ls-restore:
	@if [ -z "$(LITESTREAM_ENV)" ]; then echo "Error: .env.production に LITESTREAM_* を設定してください"; exit 1; fi
	@rm -f ./restored.db
	env $(LITESTREAM_ENV) LITESTREAM_BUCKET=$${LITESTREAM_BUCKET:-$(LITESTREAM_BUCKET_NAME)} \
		litestream restore -config litestream.yml -o ./restored.db /app/data/app.db
	@echo ">> リストア完了: ./restored.db"

# R2 から特定時点の状態をローカルにリストア
# Usage: make ls-restore-timestamp TIMESTAMP="2026-03-15T04:00:00Z"
ls-restore-timestamp:
	@if [ -z "$(LITESTREAM_ENV)" ]; then echo "Error: .env.production に LITESTREAM_* を設定してください"; exit 1; fi
	@if [ -z "$(TIMESTAMP)" ]; then echo "Usage: make ls-restore-timestamp TIMESTAMP=\"2026-03-15T04:00:00Z\""; exit 1; fi
	@rm -f ./restored.db
	env $(LITESTREAM_ENV) LITESTREAM_BUCKET=$${LITESTREAM_BUCKET:-$(LITESTREAM_BUCKET_NAME)} \
		litestream restore -config litestream.yml -timestamp "$(TIMESTAMP)" -o ./restored.db /app/data/app.db
	@echo ">> リストア完了: ./restored.db (時点: $(TIMESTAMP))"

# restored.db をローカルの app.db に反映
# 既存の app.db はバックアップしてから上書き
ls-apply:
	@if [ ! -f ./restored.db ]; then echo "Error: restored.db がありません。先に make ls-restore を実行してください"; exit 1; fi
	@if [ -f ./app.db ]; then \
		cp ./app.db ./app.db.bak; \
		echo ">> 既存 DB をバックアップ: ./app.db.bak"; \
	fi
	@mv ./restored.db ./app.db
	@rm -f ./app.db-shm ./app.db-wal
	@echo ">> 反映完了: ./app.db"
