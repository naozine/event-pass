# タイムアウト設計ガイド

## リクエストの流れとタイムアウトの関係

```
ブラウザ → [1. Read] → サーバー処理 → [2. Write] → ブラウザ
           ────────── [3. ContextTimeout] ──────────
           ──────── [4. busy_timeout（DB内部）] ────────
           ~~~~~~~ [5. Idle] ~~~~~~~ (次のリクエスト待ち)
```

## 各タイムアウトの説明

### 1. ReadTimeout (10s)

**「ブラウザからのデータを受け取る制限時間」**

ブラウザがフォームの内容を送ってくるのを待つ時間。ログインフォームの POST データなんてほんの数バイトなので、10秒あれば十分。超低速な回線や、わざと少しずつデータを送る攻撃（slowloris）を防ぐ。

### 2. WriteTimeout (30s)

**「ブラウザにレスポンスを返し終わるまでの制限時間」**

サーバーがリクエストを受け取ってからレスポンスを送り終わるまでの全体の時間。この中にサーバー処理時間も含まれる。ContextTimeout (15s) で処理は先に打ち切られるので、30s は安全マージン。

### 3. ContextTimeout (15s)

**「ハンドラの処理時間の制限」**

これが一番重要。Echo のミドルウェアで、リクエストの処理（DB アクセス含む）が15秒以内に終わらなければ 503 エラーを返す。DB のロック待ちで延々ハングするのを防ぐ。

### 4. busy_timeout (30s)

**「SQLite のロック待ち時間」**

SQLite は同時に1つしか書き込めない。他の処理が書き込み中だったら、この時間まで待ってリトライする。30秒に設定するが、ContextTimeout (15s) の方が先に発動するので、実際は15秒以上待つことはない。

### 5. IdleTimeout (120s)

**「接続を開いたまま次のリクエストを待つ時間」**

HTTP Keep-Alive の話。ブラウザは1回接続したらしばらく接続を使い回す。120秒間新しいリクエストがなければ接続を閉じる。サーバーのリソースを無駄に消費しないため。

## なぜ ContextTimeout < busy_timeout にするのか

```
ContextTimeout (15s) が先に発動 → ユーザーに「タイムアウトです」と返す
busy_timeout (30s) は保険 → ContextTimeout がない内部処理用
```

ContextTimeout がなかったら、busy_timeout の30秒間ユーザーは画面が固まったまま待たされることになる。15秒で切って早めにエラーを返す方がユーザー体験が良い。

## ContextTimeout の値の根拠

### 業界の一般的な値

| サービス/ガイド | 推奨タイムアウト |
|---|---|
| Heroku | 30秒（プロキシ固定）、アプリ側は **10〜15秒** を推奨 |
| fly.io | プロキシのデフォルトが **約30秒** |
| Cloudflare (Go ガイド) | ReadTimeout/WriteTimeout は用途に応じて **5〜30秒** |
| Zalando | API は **数秒〜10秒**、バックグラウンド処理は別管理 |

### このアプリの実測値

ログインフローの処理時間（MemoryToken モード、Phase 1 ベンチ結果）:

| 同時接続数 | ローカル (M4) | fly.io (shared 1コア) |
|---|---|---|
| 100 | 12ms | 21ms |
| 500 | 71ms | 719ms |
| 1000 | 156ms | 1.7s |

正常系は **2秒以内** で終わるため、5秒でも十分余裕がある。
15秒だとコネクションを長く占有してしまうので、**5秒** の方がこのアプリには適切。

### 参考リンク

- [Heroku Request Timeout](https://devcenter.heroku.com/articles/request-timeout)
- [The complete guide to Go net/http timeouts (Cloudflare)](https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/)
- [Zalando - All you need to know about timeouts](https://engineering.zalando.com/posts/2023/07/all-you-need-to-know-about-timeouts.html)
- [Echo Timeout Cookbook](https://echo.labstack.com/docs/cookbook/timeout)

## fly.io のコネクション制限について

`fly.toml` に以下の設定がある:

```toml
[http_service.concurrency]
    type = "connections"
    hard_limit = 25
    soft_limit = 20
```

これは **fly.io 側の制限ではなく、自分で設定するオートスケーリングのトリガー**。
「1インスタンスあたり25接続を超えたら新しいインスタンスを起動する」という意味で、25接続でリクエストが拒否されるわけではない。

ただし `min_machines_running = 0` の1インスタンス運用では、25接続前後がそのインスタンスのキャパシティの目安になる。3000ユーザー殺到シナリオでは、この値を上げるか複数インスタンスに分散するか検討が必要（Phase 3 で判断）。

## 設定値まとめ

| タイムアウト | 値 | 設定場所 |
|---|---|---|
| ReadTimeout | 10s | `cmd/server/main.go` (`http.Server`) |
| WriteTimeout | 10s | `cmd/server/main.go` (`http.Server`) |
| IdleTimeout | 120s | `cmd/server/main.go` (`http.Server`) |
| ContextTimeout | 5s | `cmd/server/main.go` (`middleware.ContextTimeout`) |
| busy_timeout | 30s | `cmd/server/main.go` (SQLite DSN) |
