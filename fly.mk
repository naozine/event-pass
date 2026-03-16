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

# Litestream Settings
LITESTREAM_BUCKET_NAME ?= $(PROJECT_NAME)-litestream

.PHONY: fly-setup fly-deploy fly-secrets fly-secrets-list fly-litestream-setup fly-litestream-secrets fly-litestream-status fly-logs fly-status fly-dns-setup

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

# Litestream 初回セットアップ（R2 バケット作成）
# wrangler CLI が必要: npm install -g wrangler
# 既存バケットがあればスキップ
fly-litestream-setup:
	@echo ">> Setting up Litestream R2 bucket: $(LITESTREAM_BUCKET_NAME)"
	@if wrangler r2 bucket list 2>/dev/null | grep -q "$(LITESTREAM_BUCKET_NAME)"; then \
		echo ">> Bucket already exists, skipping creation"; \
	else \
		echo ">> Creating R2 bucket..."; \
		wrangler r2 bucket create $(LITESTREAM_BUCKET_NAME); \
	fi
	@echo ""
	@echo ">> fly-litestream-setup complete!"
	@echo ">> Next: Cloudflare ダッシュボードで API トークンを作成してください"
	@echo ">>   1. https://dash.cloudflare.com/ → R2 → API トークンを管理"
	@echo ">>   2. 「API トークンを作成」→ 権限: オブジェクトの読み取りと書き込み"
	@echo ">>   3. バケットを「$(LITESTREAM_BUCKET_NAME)」のみに制限"
	@echo ">>   4. 取得した Access Key ID / Secret Access Key を以下で設定:"
	@echo ">>      make fly-litestream-secrets \\"
	@echo ">>        LITESTREAM_ACCESS_KEY_ID=xxx \\"
	@echo ">>        LITESTREAM_SECRET_ACCESS_KEY=xxx \\"
	@echo ">>        LITESTREAM_R2_ENDPOINT=https://<account-id>.r2.cloudflarestorage.com"

# fly.io に Litestream (Cloudflare R2) 用の secrets を設定
# バケット命名規則: <プロジェクト名>-litestream（未指定時のデフォルト）
# プロジェクトごとにバケットと API トークンを分けること（セキュリティ分離）
# 事前に環境変数を設定してから実行:
#   LITESTREAM_ACCESS_KEY_ID, LITESTREAM_SECRET_ACCESS_KEY, LITESTREAM_R2_ENDPOINT
#   LITESTREAM_BUCKET (任意: デフォルト <project-name>-litestream)
#   LITESTREAM_PATH (任意: デフォルト replica)
fly-litestream-secrets:
	@if [ -z "$(LITESTREAM_ACCESS_KEY_ID)" ] || [ -z "$(LITESTREAM_SECRET_ACCESS_KEY)" ] || [ -z "$(LITESTREAM_R2_ENDPOINT)" ]; then \
		echo "Error: 以下の環境変数を設定してください:"; \
		echo "  LITESTREAM_ACCESS_KEY_ID, LITESTREAM_SECRET_ACCESS_KEY, LITESTREAM_R2_ENDPOINT"; \
		exit 1; \
	fi
	@echo ">> Setting Litestream secrets for fly.io app: $(PROJECT_NAME)"
	@echo ">> Bucket: $(or $(LITESTREAM_BUCKET),$(PROJECT_NAME)-litestream) (デフォルト)"
	fly secrets set -a $(PROJECT_NAME) \
		LITESTREAM_ACCESS_KEY_ID="$(LITESTREAM_ACCESS_KEY_ID)" \
		LITESTREAM_SECRET_ACCESS_KEY="$(LITESTREAM_SECRET_ACCESS_KEY)" \
		LITESTREAM_R2_ENDPOINT="$(LITESTREAM_R2_ENDPOINT)" \
		LITESTREAM_BUCKET="$(or $(LITESTREAM_BUCKET),$(LITESTREAM_BUCKET_NAME))" \
		$(if $(LITESTREAM_PATH),LITESTREAM_PATH="$(LITESTREAM_PATH)",)
	@echo ">> Litestream secrets set successfully!"
	@echo ">> 確認: fly secrets list -a $(PROJECT_NAME)"

# fly.io VM 上の Litestream レプリケーション状態を確認
fly-litestream-status:
	@echo ">> Generations:"
	@fly ssh console -a $(PROJECT_NAME) -C "litestream generations -config /etc/litestream.yml /app/data/app.db"
	@echo ""
	@echo ">> Snapshots:"
	@fly ssh console -a $(PROJECT_NAME) -C "litestream snapshots -config /etc/litestream.yml /app/data/app.db"

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
