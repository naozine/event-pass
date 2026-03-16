# 負荷テスト

## テストシナリオ

マジックリンクログインフロー（Login → Verify）を段階的に同時ユーザー数を増やして実行する。
各 VU は異なるテストユーザー（`loadtest-NNNN@loadtest.example.com`）でログインし、セッション蓄積の負荷も検証する。

### ステージ構成

| 時間   | 同時 VU |
|--------|---------|
| 0-10s  | 0→10    |
| 10-30s | 10→25   |
| 30-50s | 25→50   |
| 50-70s | 50→100  |
| 70-80s | 100→0   |

## 前提条件

- `.bypass_emails` に `*@loadtest.example.com` が登録済み
- テストユーザーが事前に登録済み（`loadtest/loadtest_users.xlsx` を管理画面からインポート）
- `DISABLE_RATE_LIMITING=true` を環境変数に設定（IP ベースレート制限の無効化）

## 実行方法

```bash
# テストユーザー Excel 生成
go run loadtest/gen_users.go

# k6 実行（fly.dev に直接。Cloudflare 経由だと WAF にブロックされる）
k6 run loadtest/login_flow.js --env BASE_URL=https://your-app.fly.dev
```

## テスト結果

### 2026-03-14 fly.io (shared-cpu-1x, 256MB, nrt リージョン)

```
全 12,260 チェック: 100% 成功
失敗リクエスト: 0% (4,906 リクエスト中 0)

Login 応答時間:  avg 148ms, p95 157ms
Verify 応答時間: avg 149ms, p95 157ms

最大 100 VU 同時接続で安定動作
スループット: 約 60 req/s（約 30 ログイン/秒）
```

### 2026-03-14 fly.io (shared-cpu-2x, 512MB, nrt リージョン)

```
全 12,115 チェック: 100% 成功
失敗リクエスト: 0% (4,848 リクエスト中 0)

Login 応答時間:  avg 157ms, p95 172ms
Verify 応答時間: avg 155ms, p95 171ms

最大 100 VU 同時接続で安定動作
スループット: 約 59 req/s（約 29 ログイン/秒）
```

### 考察

- CPU を倍（1x→2x）にしてもスループットは変わらなかった
- ボトルネックはサーバー処理ではなく、テストクライアント⇔fly.io 間のネットワーク往復（約 145ms）
- サーバー側の処理時間は数ms で、CPU に十分な余裕がある
- 3,000人が数分間に分散してアクセスする現実的なシナリオでは shared-cpu-1x で十分
- Cloudflare Proxy 経由の場合、WAF が同一 IP からの大量リクエストをブロックするため `.fly.dev` に直接アクセスが必要
- 実際の多数クライアントからの負荷では、SQLite の同時書き込み競合やコネクション数上限が別のボトルネックになる可能性がある

## 注意事項

- テスト後は `DISABLE_RATE_LIMITING` を必ず削除すること
- fly.io の `.bypass_emails` からテスト用パターンを削除すること
- テストユーザーは管理画面から削除するか、無効化すること
