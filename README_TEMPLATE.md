# GOTH Stack Base Template

**Go + Echo + templ + Tailwind CSS + sqlc + SQLite**

Web アプリケーション開発のベーステンプレートです。
認証（Magic Link / WebAuthn）、ユーザー管理、DB マイグレーション、ホットリロード、バックアップを備えています。

## 技術スタック

| カテゴリ | 技術 |
|---|---|
| 言語 | Go 1.25+ |
| フレームワーク | Echo v4 |
| テンプレート | templ |
| スタイル | Tailwind CSS（CLI ローカルビルド） |
| データベース | SQLite (modernc.org/sqlite, CGO free) |
| SQL 生成 | sqlc |
| マイグレーション | goose |
| 認証 | nz-magic-link (Magic Link + WebAuthn/Passkey) |
| バックアップ | Litestream → Cloudflare R2 |
| デプロイ | fly.io (Docker) / VPS (Docker + Caddy) |

## 使い方

### 1. プロジェクトの作成

このリポジトリをテンプレートとして使用するか、クローンして新しいリポジトリを作成します。

### 2. ビジネスロジックの実装

既存の `projects` 機能を参考に、以下のファイルを編集・追加します。

- `cmd/server/routes_business.go` — アプリ名・リダイレクト設定
- `db/schema_business.sql`, `db/query_business.sql` — テーブル・クエリ定義
- `internal/handlers/business_*.go` — ハンドラー
- `internal/routes/business.go` — ルーティング
- `web/components/*.templ` — UI コンポーネント

### 3. 開発コマンド

| コマンド | 説明 |
|---|---|
| `air` | ホットリロードでサーバー起動 |
| `make generate` | コード生成（sqlc + templ + Tailwind CSS） |
| `make build` | ビルド |
| `make migrate-new NAME=create_xxx` | マイグレーションファイル作成 |

### 4. CSS の開発

Tailwind CSS はローカルビルドです（CDN 不使用）。

- 入力: `web/static/css/input.css`
- 出力: `web/static/css/style.css`（生成ファイル）
- `make generate` または `air` で自動ビルドされます
- Tailwind CLI のインストールが必要です

## デプロイ

### fly.io

```bash
# 初回セットアップ
make fly-setup

# シークレット設定
fly secrets set -a <アプリ名> \
  ADMIN_EMAIL=admin@example.com \
  SMTP_HOST=smtp.example.com \
  SMTP_PORT=587 \
  SMTP_USERNAME=user \
  SMTP_PASSWORD=pass \
  SMTP_FROM=noreply@example.com

# デプロイ
make fly-deploy

# カスタムドメイン設定（オプション）
make fly-dns-setup
```

その他: `make fly-status`, `make fly-logs`

### VPS (Docker)

```bash
# deploy.config を設定
cp deploy.config.example deploy.config

# 環境変数を設定
cp .env.production.example .env.production

# デプロイ
make docker-deploy

# Caddy 設定（初回のみ）
make caddy-setup
```

その他: `make docker-remote-logs`, `make docker-restart`

### Litestream バックアップ（オプション）

Cloudflare R2 への SQLite リアルタイムバックアップ。環境変数の有無で有効/無効が切り替わります。

```bash
# R2 バケット作成
make fly-litestream-setup

# シークレット設定
make fly-litestream-secrets

# 状態確認
make fly-litestream-status
```

ローカルからのリストア: `make ls-restore`, `make ls-restore-timestamp TIMESTAMP="2026-03-15T04:00:00Z"`

### ビルド時に自動設定される値

以下はデプロイ時に自動設定されます（.env での設定不要）:

- `SERVER_ADDR`: `https://$(PUBLIC_HOST)` から生成
- `WEBAUTHN_RP_ID`: `SERVER_ADDR` のホスト名部分
- `WEBAUTHN_ALLOWED_ORIGINS`: `SERVER_ADDR` と同じ

開発時（`air` 使用時）は `http://localhost:PORT` が自動で使用されます。

## テンプレート更新の取り込み

### リモートリポジトリの追加（初回のみ）

```bash
git remote add template https://github.com/naozine/project_crud_with_auth_tmpl.git
```

### 更新のマージ

```bash
git fetch template
git merge template/master --allow-unrelated-histories
```

コンフリクトが発生した場合は、ビジネスロジック領域（`*_business.*` 等）は自分の変更を優先し、その後 `go mod tidy && make generate` を実行します。

派生プロジェクトが大きく乖離している場合は、マージよりも AI エージェントに「この機能を参考に実装して」と指示する方が現実的なこともあります。

## ファイル構成

- `*_business.*` はビジネスロジック領域。自由に変更・リネーム可。
- `internal/database/*.go`（sqlc 自動生成）は `.gitignore` で除外。CI/CD ではビルド前に `make generate` を実行。
- `db/*.sql` に日本語を書かないこと（sqlc のコード生成がバグる）。
