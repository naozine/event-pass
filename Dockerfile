# =============================================================================
# Dockerfile - マルチステージビルド
# =============================================================================
# VPS (docker-deploy) と fly.io (fly-deploy) の両方で使用
# SQLite は modernc.org/sqlite (Pure Go) を使用
# =============================================================================

# -----------------------------------------------------------------------------
# Stage 1: Build
# -----------------------------------------------------------------------------
FROM golang:1.25-alpine AS builder

# ビルドに必要なツールをインストール
RUN apk add --no-cache git

WORKDIR /app

# 依存関係を先にコピーしてキャッシュを活用
COPY go.mod go.sum ./
RUN go mod download

# ソースコードをコピー
COPY . .

# バージョン情報、プロジェクト名、サーバーアドレスを埋め込んでビルド
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
ARG PROJECT_NAME=app
ARG SERVER_ADDR=http://localhost:8080

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags "-X 'github.com/naozine/project_crud_with_auth_tmpl/internal/version.Version=${VERSION}' \
              -X 'github.com/naozine/project_crud_with_auth_tmpl/internal/version.Commit=${COMMIT}' \
              -X 'github.com/naozine/project_crud_with_auth_tmpl/internal/version.BuildDate=${BUILD_DATE}' \
              -X 'github.com/naozine/project_crud_with_auth_tmpl/internal/version.ProjectName=${PROJECT_NAME}' \
              -X 'github.com/naozine/project_crud_with_auth_tmpl/internal/version.ServerAddr=${SERVER_ADDR}'" \
    -o /app/server ./cmd/server

# -----------------------------------------------------------------------------
# Stage 2: Runtime
# -----------------------------------------------------------------------------
FROM alpine:3.21

# シェルやメンテナンスコマンドが使える軽量ランタイム
# ca-certificates: HTTPS通信（SMTP TLS等）に必要
# tzdata: タイムゾーン処理に必要
RUN apk add --no-cache ca-certificates tzdata su-exec \
    && wget -qO- https://github.com/benbjohnson/litestream/releases/download/v0.3.13/litestream-v0.3.13-linux-amd64.tar.gz \
    | tar -C /usr/local/bin -xz

# nonroot ユーザーで実行（entrypoint.sh で root → appuser に降格）
RUN adduser -D -u 10001 appuser

WORKDIR /app

# バイナリをコピー
COPY --from=builder /app/server /app/server

# 静的ファイルをコピー
COPY --from=builder /app/web/static /app/web/static

# データディレクトリ (SQLite DB用) - ボリュームマウント先
VOLUME ["/app/data"]

# 環境変数のデフォルト値
ARG PROJECT_NAME=app
ENV PORT=8080
ENV APP_NAME=${PROJECT_NAME}

# ポートを公開
EXPOSE 8080

# Litestream 設定ファイル
COPY litestream.yml /etc/litestream.yml

# エントリポイント（データディレクトリの所有権修正後、appuser で実行）
COPY --from=builder /app/entrypoint.sh /app/entrypoint.sh
ENTRYPOINT ["/app/entrypoint.sh"]
