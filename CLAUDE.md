# Tech Stack

- Go 1.23+ / Echo v4 / templ / htmx / Alpine.js / Tailwind CSS
- SQLite (modernc.org/sqlite, CGO free) + sqlc + goose
- 認証: nz-magic-link (Magic Link + WebAuthn/Passkey)
- バックアップ: Litestream → Cloudflare R2
- デプロイ: fly.io (Docker)

# ルール

- `go run` 実行禁止（サーバーが止まらない）
- htmx でページ遷移を置き換えない（検索・バリデーション・トグル等の部分更新のみ）
- モーダルで CRUD しない（専用ページを使う）
- JSON API で UI を作らない（フォーム送信を使う）
