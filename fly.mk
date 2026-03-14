# =============================================================================
# fly.io Deploy
# =============================================================================
# プロジェクト固有の設定を読み込む（存在する場合）
-include deploy.config

PROJECT_NAME := $(subst _,-,$(notdir $(CURDIR)))
VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT     := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
SERVER_ADDR   ?= http://localhost:8080
PUBLIC_HOST   ?= localhost

# Cloudflare Settings
CF_API_TOKEN  ?=
CF_ZONE_ID    ?=

# fly.io 用 SERVER_ADDR 自動解決
# カスタムドメインがあればそれを、なければ $(PROJECT_NAME).fly.dev を使用
FLY_SERVER_ADDR = $(if $(filter localhost,$(PUBLIC_HOST)),https://$(PROJECT_NAME).fly.dev,https://$(PUBLIC_HOST))

.PHONY: fly-setup fly-deploy fly-secrets fly-secrets-list fly-logs fly-status fly-dns-setup

# fly.io 初回セットアップ（アプリ作成 + ボリューム作成 + fly.toml生成）
# 既存アプリ/ボリューム/fly.tomlがある場合はスキップ
fly-setup:
	@echo ">> Setting up fly.io app: $(PROJECT_NAME)"
	@if fly apps list --json | jq -e '.[] | select(.Name == "$(PROJECT_NAME)")' > /dev/null 2>&1; then \
		echo ">> App already exists, skipping creation"; \
	else \
		echo ">> Creating app..."; \
		fly apps create $(PROJECT_NAME); \
	fi
	@if fly volumes list -a $(PROJECT_NAME) --json | jq -e '.[] | select(.Name == "data")' > /dev/null 2>&1; then \
		echo ">> Volume 'data' already exists, skipping creation"; \
	else \
		echo ">> Creating volume for SQLite data..."; \
		fly volumes create data --region nrt --size 1 -a $(PROJECT_NAME) -y; \
	fi
	@if [ -f fly.toml ]; then \
		echo ">> fly.toml already exists, skipping generation"; \
	else \
		echo ">> Generating fly.toml from template..."; \
		sed 's/^app = .*/app = "$(PROJECT_NAME)"/' fly.toml.example > fly.toml; \
	fi
	@echo ">> fly-setup complete!"
	@echo ">> Next: fly secrets set ... && make fly-deploy"

# fly.io へデプロイ
fly-deploy:
	$(MAKE) generate
	@echo ">> Deploying to fly.io..."
	@echo ">> App: $(PROJECT_NAME)"
	@echo ">> Server: $(FLY_SERVER_ADDR)"
	fly deploy -a $(PROJECT_NAME) \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		--build-arg PROJECT_NAME=$(PROJECT_NAME) \
		--build-arg SERVER_ADDR=$(FLY_SERVER_ADDR)

# fly.io に .env.production の環境変数を設定
# .env.production ファイルの内容を fly secrets にインポート
fly-secrets:
	@if [ ! -f ".env.production" ]; then \
		echo "Error: .env.production ファイルが見つかりません"; \
		exit 1; \
	fi
	@echo ">> Importing secrets from .env.production to fly.io app: $(PROJECT_NAME)"
	@cat .env.production | fly secrets import -a $(PROJECT_NAME)
	@echo ">> Secrets imported successfully!"
	@echo ">> 確認: fly secrets list -a $(PROJECT_NAME)"

# fly.io の secrets を一覧表示
fly-secrets-list:
	fly secrets list -a $(PROJECT_NAME)

# fly.io のログを表示
fly-logs:
	fly logs -a $(PROJECT_NAME)

# fly.io のステータス確認
fly-status:
	fly status -a $(PROJECT_NAME)

# fly.io 用 DNS 設定（Cloudflare + fly certs）
# 事前に fly-deploy でアプリがデプロイ済みであること
# PUBLIC_HOST にカスタムドメインを設定してから実行
# 既存の A/AAAA/CNAME レコードがあれば削除してから CNAME を作成
# 証明書発行のため Proxy OFF で作成 → 発行後にユーザーが Proxy ON にする
# Cloudflare Proxy 経由での証明書自動更新に必要な DNS レコードも追加:
#   - _fly-ownership TXT: fly.io へのドメイン所有権証明
#   - _acme-challenge CNAME: Let's Encrypt DNS チャレンジ用
fly-dns-setup:
	@if [ -z "$(CF_API_TOKEN)" ] || [ -z "$(CF_ZONE_ID)" ]; then \
		echo "Error: CF_API_TOKEN, CF_ZONE_ID を deploy.config に設定してください"; \
		exit 1; \
	fi
	@if [ "$(PUBLIC_HOST)" = "localhost" ]; then \
		echo "Error: PUBLIC_HOST にカスタムドメインを設定してください"; \
		exit 1; \
	fi
	@echo ">> Setting up DNS for $(PUBLIC_HOST) -> $(PROJECT_NAME).fly.dev"
	# 1. 既存の A/AAAA/CNAME レコードを削除
	@echo ">> Checking for existing DNS records..."
	@RECORD_ID=$$(curl -s -X GET "https://api.cloudflare.com/client/v4/zones/$(CF_ZONE_ID)/dns_records?name=$(PUBLIC_HOST)" \
		-H "Authorization: Bearer $(CF_API_TOKEN)" \
		| jq -r '.result[] | select(.type == "A" or .type == "AAAA" or .type == "CNAME") | .id' | head -1); \
	if [ -n "$$RECORD_ID" ]; then \
		echo ">> Deleting existing record: $$RECORD_ID"; \
		curl -s -X DELETE "https://api.cloudflare.com/client/v4/zones/$(CF_ZONE_ID)/dns_records/$$RECORD_ID" \
			-H "Authorization: Bearer $(CF_API_TOKEN)" | jq -r '.success'; \
	fi
	# 2. Cloudflare に CNAME 追加 (Proxy OFF - 証明書発行のため)
	@curl -s -X POST "https://api.cloudflare.com/client/v4/zones/$(CF_ZONE_ID)/dns_records" \
		-H "Authorization: Bearer $(CF_API_TOKEN)" \
		-H "Content-Type: application/json" \
		--data '{"type":"CNAME","name":"$(PUBLIC_HOST)","content":"$(PROJECT_NAME).fly.dev","proxied":false,"ttl":1}' \
		| jq -r 'if .success then "DNS record created: \(.result.name) (Proxy OFF)" else "Error: \(.errors[0].message)" end'
	# 3. fly.io にカスタムドメインの証明書を追加
	@echo ">> Adding certificate for $(PUBLIC_HOST) on fly.io..."
	-@fly certs add $(PUBLIC_HOST) -a $(PROJECT_NAME)
	# 4. fly certs から CNAME ターゲットを取得し、アプリ ID を抽出
	#    例: CNAME → 535p2xn.project-name.fly.dev → APP_ID = 535p2xn
	@APP_ID=$$(fly certs setup $(PUBLIC_HOST) -a $(PROJECT_NAME) 2>/dev/null \
		| grep -oE '[a-z0-9]+\.$(PROJECT_NAME)\.fly\.dev' \
		| head -1 | cut -d. -f1); \
	if [ -z "$$APP_ID" ]; then \
		echo "Warning: アプリ ID を取得できませんでした。_fly-ownership / _acme-challenge は手動で設定してください"; \
	else \
		echo ">> App ID: $$APP_ID"; \
		HOSTNAME=$$(echo "$(PUBLIC_HOST)" | sed 's/\.[^.]*\.[^.]*$$//'); \
		echo ">> Setting up _fly-ownership TXT record..."; \
		EXISTING=$$(curl -s -X GET "https://api.cloudflare.com/client/v4/zones/$(CF_ZONE_ID)/dns_records?name=_fly-ownership.$(PUBLIC_HOST)&type=TXT" \
			-H "Authorization: Bearer $(CF_API_TOKEN)" \
			| jq -r '.result[0].id // empty'); \
		if [ -n "$$EXISTING" ]; then \
			curl -s -X PUT "https://api.cloudflare.com/client/v4/zones/$(CF_ZONE_ID)/dns_records/$$EXISTING" \
				-H "Authorization: Bearer $(CF_API_TOKEN)" \
				-H "Content-Type: application/json" \
				--data "{\"type\":\"TXT\",\"name\":\"_fly-ownership.$(PUBLIC_HOST)\",\"content\":\"app-$$APP_ID\",\"ttl\":1}" \
				| jq -r 'if .success then "TXT record updated: _fly-ownership -> app-\("'"$$APP_ID"'")" else "Error: \(.errors[0].message)" end'; \
		else \
			curl -s -X POST "https://api.cloudflare.com/client/v4/zones/$(CF_ZONE_ID)/dns_records" \
				-H "Authorization: Bearer $(CF_API_TOKEN)" \
				-H "Content-Type: application/json" \
				--data "{\"type\":\"TXT\",\"name\":\"_fly-ownership.$(PUBLIC_HOST)\",\"content\":\"app-$$APP_ID\",\"ttl\":1}" \
				| jq -r 'if .success then "TXT record created: _fly-ownership -> app-\("'"$$APP_ID"'")" else "Error: \(.errors[0].message)" end'; \
		fi; \
		echo ">> Setting up _acme-challenge CNAME record..."; \
		EXISTING=$$(curl -s -X GET "https://api.cloudflare.com/client/v4/zones/$(CF_ZONE_ID)/dns_records?name=_acme-challenge.$(PUBLIC_HOST)&type=CNAME" \
			-H "Authorization: Bearer $(CF_API_TOKEN)" \
			| jq -r '.result[0].id // empty'); \
		if [ -n "$$EXISTING" ]; then \
			curl -s -X PUT "https://api.cloudflare.com/client/v4/zones/$(CF_ZONE_ID)/dns_records/$$EXISTING" \
				-H "Authorization: Bearer $(CF_API_TOKEN)" \
				-H "Content-Type: application/json" \
				--data "{\"type\":\"CNAME\",\"name\":\"_acme-challenge.$(PUBLIC_HOST)\",\"content\":\"$(PUBLIC_HOST).$$APP_ID.flydns.net\",\"proxied\":false,\"ttl\":1}" \
				| jq -r 'if .success then "CNAME record updated: _acme-challenge" else "Error: \(.errors[0].message)" end'; \
		else \
			curl -s -X POST "https://api.cloudflare.com/client/v4/zones/$(CF_ZONE_ID)/dns_records" \
				-H "Authorization: Bearer $(CF_API_TOKEN)" \
				-H "Content-Type: application/json" \
				--data "{\"type\":\"CNAME\",\"name\":\"_acme-challenge.$(PUBLIC_HOST)\",\"content\":\"$(PUBLIC_HOST).$$APP_ID.flydns.net\",\"proxied\":false,\"ttl\":1}" \
				| jq -r 'if .success then "CNAME record created: _acme-challenge" else "Error: \(.errors[0].message)" end'; \
		fi; \
	fi
	# 6. 証明書発行を待機
	@echo ">> Waiting for certificate issuance..."
	@for i in 1 2 3 4 5 6; do \
		sleep 10; \
		STATUS=$$(fly certs show $(PUBLIC_HOST) -a $(PROJECT_NAME) 2>/dev/null | grep "^Status" | awk '{print $$NF}'); \
		echo ">> Certificate status: $$STATUS"; \
		if [ "$$STATUS" = "Ready" ]; then \
			echo ">> Certificate issued successfully!"; \
			break; \
		fi; \
		if [ $$i -eq 6 ]; then \
			echo ">> Certificate not ready yet. Check with: fly certs show $(PUBLIC_HOST) -a $(PROJECT_NAME)"; \
		fi; \
	done
	@echo ""
	@echo ">> fly-dns-setup complete!"
	@echo ">> Next: Cloudflare ダッシュボードで $(PUBLIC_HOST) の Proxy を ON (オレンジ雲) にしてください"
	@echo ">> Access: https://$(PUBLIC_HOST)"
