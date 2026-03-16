# =============================================================================
# VPS Deploy (Docker + Caddy + Cloudflare DNS)
# =============================================================================
# プロジェクト固有の設定を読み込む（存在する場合）
-include deploy.config

PROJECT_NAME := $(subst _,-,$(notdir $(CURDIR)))
VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT     := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
SERVER_ADDR   ?= http://localhost:8080
PUBLIC_HOST   ?= localhost

# VPS Connection Info (deploy.config または環境変数で上書き可能)
VPS_USER     ?= user
VPS_HOST     ?= 192.168.1.100
SSH_PORT     ?= 22
VPS_BASE_DIR ?= /var/www

# Docker Settings (デフォルトはフォルダ名から自動生成)
IMAGE_NAME    ?= $(PROJECT_NAME)
IMAGE_TAG     ?= $(VERSION)
APP_NAME      ?= $(PROJECT_NAME)

# Caddy Settings (共有リバースプロキシ)
CADDY_DIR     ?= /home/$(VPS_USER)/caddy
CADDY_NETWORK ?= caddy-net

# VPS上の実際のデプロイパス
VPS_DEPLOY_DIR := $(VPS_BASE_DIR)/$(PROJECT_NAME)

# VPS IP（DNS 設定で使用）
VPS_IP        ?=

# Cloudflare Settings
CF_API_TOKEN  ?=
CF_ZONE_ID    ?=

.PHONY: caddy-setup caddy-status caddy-logs caddy-reload \
        dns-setup docker-deploy docker-restart docker-remote-logs

# -----------------------------------------------------------------------------
# Caddy (共有リバースプロキシ) 関連
# -----------------------------------------------------------------------------

# Caddy 初回セットアップ（VPSに Caddy がなければ構築）
caddy-setup:
	@echo ">> Setting up Caddy on $(VPS_HOST)..."
	# 1. Caddy ディレクトリを作成
	ssh -p $(SSH_PORT) $(VPS_USER)@$(VPS_HOST) "mkdir -p $(CADDY_DIR)/sites"
	# 2. Caddy 設定ファイルを転送
	rsync -avz -e "ssh -p $(SSH_PORT)" caddy/docker-compose.yaml $(VPS_USER)@$(VPS_HOST):$(CADDY_DIR)/
	rsync -avz -e "ssh -p $(SSH_PORT)" caddy/Caddyfile $(VPS_USER)@$(VPS_HOST):$(CADDY_DIR)/
	# 3. Docker ネットワークを作成（存在しなければ）
	ssh -p $(SSH_PORT) $(VPS_USER)@$(VPS_HOST) "docker network create $(CADDY_NETWORK) 2>/dev/null || true"
	# 4. Caddy を起動
	ssh -p $(SSH_PORT) $(VPS_USER)@$(VPS_HOST) "cd $(CADDY_DIR) && docker compose up -d"
	@echo ">> Caddy setup complete!"

# Caddy の状態確認
caddy-status:
	@echo ">> Caddy status on $(VPS_HOST)..."
	ssh -p $(SSH_PORT) $(VPS_USER)@$(VPS_HOST) "cd $(CADDY_DIR) && docker compose ps"

# Caddy のログ確認
caddy-logs:
	ssh -p $(SSH_PORT) $(VPS_USER)@$(VPS_HOST) "cd $(CADDY_DIR) && docker compose logs -f"

# Caddy をリロード（設定変更を反映）
caddy-reload:
	ssh -p $(SSH_PORT) $(VPS_USER)@$(VPS_HOST) "docker exec caddy caddy reload --config /etc/caddy/Caddyfile"

# -----------------------------------------------------------------------------
# Cloudflare DNS 設定
# -----------------------------------------------------------------------------

# DNS レコードを設定（Cloudflare）
# 必要な環境変数: CF_API_TOKEN, CF_ZONE_ID, VPS_IP
dns-setup:
	@if [ -z "$(CF_API_TOKEN)" ] || [ -z "$(CF_ZONE_ID)" ] || [ -z "$(VPS_IP)" ]; then \
		echo "Error: CF_API_TOKEN, CF_ZONE_ID, VPS_IP を deploy.config に設定してください"; \
		exit 1; \
	fi
	@echo ">> Setting up DNS for $(PUBLIC_HOST)..."
	@curl -s -X POST "https://api.cloudflare.com/client/v4/zones/$(CF_ZONE_ID)/dns_records" \
		-H "Authorization: Bearer $(CF_API_TOKEN)" \
		-H "Content-Type: application/json" \
		--data '{"type":"A","name":"$(PROJECT_NAME)","content":"$(VPS_IP)","proxied":true,"ttl":1}' \
		| jq -r 'if .success then "DNS record created: \(.result.name)" else "Error: \(.errors[0].message)" end'
	@echo ""
	@echo ">> DNS の伝播を待ってから make docker-deploy を実行してください（1-2分程度）"
	@echo ">> 確認: dig $(PUBLIC_HOST) で $(VPS_IP) ではなく Cloudflare の IP が返れば OK"

# -----------------------------------------------------------------------------
# Docker Deploy (VPS上でDockerビルド・実行 + Caddy 設定)
# -----------------------------------------------------------------------------

# Deploy to VPS using Docker
# ソースを転送し、VPS上でDockerマルチステージビルド
docker-deploy:
	$(MAKE) generate
	@echo ">> Deploying Docker to $(VPS_HOST)..."
	@echo ">> App: $(APP_NAME) -> $(PUBLIC_HOST)"
	@echo ">> Deploy path: $(VPS_DEPLOY_DIR)"
	# 1. Caddy が起動しているか確認（なければセットアップ）
	@ssh -p $(SSH_PORT) $(VPS_USER)@$(VPS_HOST) "docker ps -q -f name=caddy" | grep -q . || $(MAKE) -f vps.mk caddy-setup
	# 2. ソースコードを転送（ホワイトリスト方式：必要なものだけ）
	ssh -p $(SSH_PORT) $(VPS_USER)@$(VPS_HOST) "mkdir -p $(VPS_DEPLOY_DIR)"
	rsync -avz -e "ssh -p $(SSH_PORT)" --delete \
		--include='go.mod' \
		--include='go.sum' \
		--include='Dockerfile' \
		--include='.dockerignore' \
		--include='docker-compose.yaml' \
		--include='cmd/***' \
		--include='internal/***' \
		--include='db/***' \
		--include='web/***' \
		--exclude='*' \
		./ $(VPS_USER)@$(VPS_HOST):$(VPS_DEPLOY_DIR)/
	# 3. 本番用 .env を転送（SMTP設定など。SERVER_ADDR, WEBAUTHN_* はビルド時注入）
	@if [ -f ".env.production" ]; then \
		rsync -avz -e "ssh -p $(SSH_PORT)" .env.production $(VPS_USER)@$(VPS_HOST):$(VPS_DEPLOY_DIR)/.env; \
	else \
		ssh -p $(SSH_PORT) $(VPS_USER)@$(VPS_HOST) "touch $(VPS_DEPLOY_DIR)/.env"; \
	fi
	# 4. Docker イメージをビルド（マルチステージビルド）
	@echo ">> Building Docker image on VPS..."
	ssh -p $(SSH_PORT) $(VPS_USER)@$(VPS_HOST) "cd $(VPS_DEPLOY_DIR) && \
		docker build \
			--build-arg VERSION=$(VERSION) \
			--build-arg COMMIT=$(COMMIT) \
			--build-arg BUILD_DATE=$(BUILD_DATE) \
			--build-arg PROJECT_NAME=$(PROJECT_NAME) \
			--build-arg SERVER_ADDR=$(SERVER_ADDR) \
			-t $(APP_NAME):latest ."
	# 5. データディレクトリの作成と権限設定（nonroot ユーザー用）
	@echo ">> Setting data directory permissions..."
	ssh -p $(SSH_PORT) $(VPS_USER)@$(VPS_HOST) "mkdir -p $(VPS_DEPLOY_DIR)/data && chmod 777 $(VPS_DEPLOY_DIR)/data"
	# 6. コンテナ起動（イメージが更新されたので再作成）
	@echo ">> Starting container..."
	ssh -p $(SSH_PORT) $(VPS_USER)@$(VPS_HOST) "cd $(VPS_DEPLOY_DIR) && APP_NAME=$(APP_NAME) docker compose up -d --force-recreate"
	# 7. Caddy 設定を追加
	@echo ">> Configuring Caddy for $(PUBLIC_HOST)..."
	@echo '$(PUBLIC_HOST) {\n    reverse_proxy $(APP_NAME):8080\n}' | \
		ssh -p $(SSH_PORT) $(VPS_USER)@$(VPS_HOST) "cat > $(CADDY_DIR)/sites/$(APP_NAME).caddy"
	# 8. Caddy をリロード
	ssh -p $(SSH_PORT) $(VPS_USER)@$(VPS_HOST) "docker exec caddy caddy reload --config /etc/caddy/Caddyfile"
	@echo ">> Docker deployment complete!"
	@echo ">> Access: https://$(PUBLIC_HOST)"

# Restart containers on VPS
docker-restart:
	ssh -p $(SSH_PORT) $(VPS_USER)@$(VPS_HOST) "cd $(VPS_DEPLOY_DIR) && docker compose restart"

# View container logs on VPS
docker-remote-logs:
	ssh -p $(SSH_PORT) $(VPS_USER)@$(VPS_HOST) "cd $(VPS_DEPLOY_DIR) && docker compose logs -f"
