package integration

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/naozine/nz-magic-link/magiclink"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/database"
	_ "modernc.org/sqlite"
)

// setupBenchDB はベンチマーク用のファイル SQLite を作成する。
// 本番と同じ WAL モード + busy_timeout を使う。
func setupBenchDB(b *testing.B) (*sql.DB, *database.Queries, *magiclink.MagicLink) {
	b.Helper()

	// ファイル DB（テスト終了時に自動削除される tmpdir に配置）
	dir := b.TempDir()
	dbPath := dir + "/bench.db"

	conn, err := sql.Open("sqlite", "file:"+dbPath+"?_pragma=busy_timeout(30000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(on)")
	if err != nil {
		b.Fatalf("DB接続に失敗: %v", err)
	}
	b.Cleanup(func() { conn.Close() })

	// アプリのマイグレーション（直接テーブルを作成）
	_, err = conn.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'viewer',
			is_active BOOLEAN NOT NULL DEFAULT 1,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(email);
	`)
	if err != nil {
		b.Fatalf("users テーブル作成に失敗: %v", err)
	}

	// magiclink を公開 API で初期化（内部で Init() も呼ばれる）
	ml, err := magiclink.NewWithDB(magiclink.Config{
		DatabaseType: "sqlite",
		TokenExpiry:  30 * time.Minute,
		SessionExpiry: 24 * time.Hour,
	}, conn)
	if err != nil {
		b.Fatalf("magiclink 初期化に失敗: %v", err)
	}

	// テストユーザーを作成
	q := database.New(conn)
	_, err = q.CreateUser(context.Background(), database.CreateUserParams{
		Email: "bench@test.com", Name: "Bench User", Role: "admin", IsActive: true,
	})
	if err != nil {
		b.Fatalf("ユーザー作成に失敗: %v", err)
	}

	return conn, q, ml
}

func generateToken() (string, string) {
	buf := make([]byte, 32)
	rand.Read(buf)
	token := base64.URLEncoding.EncodeToString(buf)
	hash := sha256.Sum256([]byte(token))
	tokenHash := base64.URLEncoding.EncodeToString(hash[:])
	return token, tokenHash
}

// BenchmarkLoginFlow_Sequential はログインフローの DB 操作を直列で実行する（ベースライン）
func BenchmarkLoginFlow_Sequential(b *testing.B) {
	_, q, ml := setupBenchDB(b)
	ctx := context.Background()
	email := "bench@test.com"
	expires := time.Now().Add(30 * time.Minute)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Step 1: AllowLogin — ユーザー存在確認
		_, err := q.GetUserByEmail(ctx, email)
		if err != nil {
			b.Fatalf("GetUserByEmail に失敗: %v", err)
		}

		// Step 2: レート制限チェック
		_, err = ml.DB.CountRecentTokens(email, time.Now().Add(-15*time.Minute))
		if err != nil {
			b.Fatalf("CountRecentTokens に失敗: %v", err)
		}

		// Step 3: トークン保存
		token, tokenHash := generateToken()
		err = ml.DB.SaveToken(token, tokenHash, email, expires)
		if err != nil {
			b.Fatalf("SaveToken に失敗: %v", err)
		}
	}
}

// BenchmarkLoginFlow_Parallel は並行ログインリクエストをシミュレートする
func BenchmarkLoginFlow_Parallel(b *testing.B) {
	_, q, ml := setupBenchDB(b)
	email := "bench@test.com"
	expires := time.Now().Add(30 * time.Minute)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ctx := context.Background()
		for pb.Next() {
			_, err := q.GetUserByEmail(ctx, email)
			if err != nil {
				b.Errorf("GetUserByEmail に失敗: %v", err)
				return
			}

			_, err = ml.DB.CountRecentTokens(email, time.Now().Add(-15*time.Minute))
			if err != nil {
				b.Errorf("CountRecentTokens に失敗: %v", err)
				return
			}

			token, tokenHash := generateToken()
			err = ml.DB.SaveToken(token, tokenHash, email, expires)
			if err != nil {
				b.Errorf("SaveToken に失敗: %v", err)
				return
			}
		}
	})
}

// BenchmarkVerifyFlow_Parallel はトークン検証+セッション作成の並行負荷をテストする
func BenchmarkVerifyFlow_Parallel(b *testing.B) {
	_, _, ml := setupBenchDB(b)
	email := "bench@test.com"
	expires := time.Now().Add(30 * time.Minute)

	// 事前にトークンを大量に作成
	tokenHashes := make([]string, b.N)
	for i := 0; i < b.N; i++ {
		token, tokenHash := generateToken()
		if err := ml.DB.SaveToken(token, tokenHash, email, expires); err != nil {
			b.Fatalf("事前トークン作成に失敗: %v", err)
		}
		tokenHashes[i] = tokenHash
	}

	var idx atomic.Int64

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := idx.Add(1) - 1
			if i >= int64(len(tokenHashes)) {
				return
			}
			tokenHash := tokenHashes[i]

			// Step 4: トークン検証
			_, _, _, _, err := ml.DB.GetTokenByHash(tokenHash)
			if err != nil {
				b.Errorf("GetTokenByHash に失敗: %v", err)
				return
			}

			// Step 5: 使用済みマーク
			err = ml.DB.MarkTokenAsUsed(tokenHash)
			if err != nil {
				b.Errorf("MarkTokenAsUsed に失敗: %v", err)
				return
			}

			// Step 6: セッション作成
			sessToken, sessHash := generateToken()
			err = ml.DB.SaveSession(sessToken, sessHash, email, expires)
			if err != nil {
				b.Errorf("SaveSession に失敗: %v", err)
				return
			}
		}
	})
}

// BenchmarkBurst は N 件の同時ログインリクエストをシミュレートする
func BenchmarkBurst(b *testing.B) {
	for _, n := range []int{100, 500, 1000} {
		b.Run(fmt.Sprintf("concurrent_%d", n), func(b *testing.B) {
			benchBurst(b, n)
		})
	}
}

func benchBurst(b *testing.B, concurrency int) {
	_, q, ml := setupBenchDB(b)

	// ユーザーを事前作成
	ctx := context.Background()
	for i := 1; i <= concurrency; i++ {
		_, err := q.CreateUser(ctx, database.CreateUserParams{
			Email:    fmt.Sprintf("user%d@test.com", i),
			Name:     fmt.Sprintf("User%d", i),
			Role:     "viewer",
			IsActive: true,
		})
		if err != nil {
			b.Fatalf("ユーザー %d の作成に失敗: %v", i, err)
		}
	}

	expires := time.Now().Add(30 * time.Minute)

	b.ResetTimer()
	for iter := 0; iter < b.N; iter++ {
		start := time.Now()
		errCh := make(chan error, concurrency)
		for i := 1; i <= concurrency; i++ {
			go func(userIdx int) {
				email := fmt.Sprintf("user%d@test.com", userIdx)
				ctx := context.Background()

				// ログインフロー全体
				_, err := q.GetUserByEmail(ctx, email)
				if err != nil {
					errCh <- fmt.Errorf("user%d GetUserByEmail: %w", userIdx, err)
					return
				}

				_, err = ml.DB.CountRecentTokens(email, time.Now().Add(-15*time.Minute))
				if err != nil {
					errCh <- fmt.Errorf("user%d CountRecentTokens: %w", userIdx, err)
					return
				}

				token, tokenHash := generateToken()
				err = ml.DB.SaveToken(token, tokenHash, email, expires)
				if err != nil {
					errCh <- fmt.Errorf("user%d SaveToken: %w", userIdx, err)
					return
				}

				errCh <- nil
			}(i)
		}

		// 全 goroutine の完了を待つ
		var failed int
		for i := 0; i < concurrency; i++ {
			if err := <-errCh; err != nil {
				failed++
				if failed <= 3 {
					b.Logf("エラー: %v", err)
				}
			}
		}
		elapsed := time.Since(start)
		b.Logf("同時 %d 件: %d 件成功, %d 件失敗, 所要時間 %v", concurrency, concurrency-failed, failed, elapsed)
	}
}
