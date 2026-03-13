package integration

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/naozine/nz-magic-link/magiclink"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/database"
	_ "modernc.org/sqlite"
)

// ---------------------------------------------------------------------------
// インメモリトークンストア（nz-magic-link #15 の想定シミュレーション）
// ---------------------------------------------------------------------------

type tokenEntry struct {
	token     string
	email     string
	expiresAt time.Time
	used      bool
	createdAt time.Time
}

type memoryTokenStore struct {
	mu     sync.RWMutex
	tokens map[string]*tokenEntry // key: tokenHash
}

func newMemoryTokenStore() *memoryTokenStore {
	return &memoryTokenStore{tokens: make(map[string]*tokenEntry)}
}

func (s *memoryTokenStore) saveToken(token, tokenHash, email string, expiresAt time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[tokenHash] = &tokenEntry{
		token: token, email: email, expiresAt: expiresAt,
		createdAt: time.Now(),
	}
}

func (s *memoryTokenStore) getTokenByHash(tokenHash string) (string, string, time.Time, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.tokens[tokenHash]
	if !ok {
		return "", "", time.Time{}, false, nil
	}
	return e.token, e.email, e.expiresAt, e.used, nil
}

func (s *memoryTokenStore) markTokenAsUsed(tokenHash string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e, ok := s.tokens[tokenHash]; ok {
		e.used = true
	}
}

func (s *memoryTokenStore) countRecentTokens(email string, since time.Time) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, e := range s.tokens {
		if e.email == email && e.createdAt.After(since) {
			count++
		}
	}
	return count
}

// ---------------------------------------------------------------------------
// セットアップ
// ---------------------------------------------------------------------------

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
		DatabaseType:  "sqlite",
		TokenExpiry:   30 * time.Minute,
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

// ---------------------------------------------------------------------------
// ストレージモード: DB 全操作 vs インメモリトークン + DB セッション
// ---------------------------------------------------------------------------

type storageMode int

const (
	modeAllDB         storageMode = iota // 従来: 全操作 DB
	modeMemoryToken                      // 想定: トークンはメモリ、セッションは DB
)

func (m storageMode) String() string {
	switch m {
	case modeAllDB:
		return "AllDB"
	case modeMemoryToken:
		return "MemoryToken"
	default:
		return "Unknown"
	}
}

// loginOps はログインフロー（フェーズ1）の操作をまとめた構造体
type loginOps struct {
	mode  storageMode
	q     *database.Queries
	ml    *magiclink.MagicLink
	mem   *memoryTokenStore
}

func (ops *loginOps) doLogin(ctx context.Context, email string, expires time.Time) error {
	// Step 1: ユーザー存在確認（常に DB）
	_, err := ops.q.GetUserByEmail(ctx, email)
	if err != nil {
		return fmt.Errorf("GetUserByEmail: %w", err)
	}

	// Step 2: レート制限チェック
	switch ops.mode {
	case modeAllDB:
		_, err = ops.ml.DB.CountRecentTokens(email, time.Now().Add(-15*time.Minute))
		if err != nil {
			return fmt.Errorf("CountRecentTokens: %w", err)
		}
	case modeMemoryToken:
		ops.mem.countRecentTokens(email, time.Now().Add(-15*time.Minute))
	}

	// Step 3: トークン保存
	token, tokenHash := generateToken()
	switch ops.mode {
	case modeAllDB:
		if err := ops.ml.DB.SaveToken(token, tokenHash, email, expires); err != nil {
			return fmt.Errorf("SaveToken: %w", err)
		}
	case modeMemoryToken:
		ops.mem.saveToken(token, tokenHash, email, expires)
	}

	return nil
}

// verifyOps はトークン検証フロー（フェーズ2）の操作をまとめた構造体
type verifyOps struct {
	mode storageMode
	ml   *magiclink.MagicLink
	mem  *memoryTokenStore
}

func (ops *verifyOps) doVerify(tokenHash, email string, expires time.Time) error {
	// Step 4: トークン検証
	switch ops.mode {
	case modeAllDB:
		_, _, _, _, err := ops.ml.DB.GetTokenByHash(tokenHash)
		if err != nil {
			return fmt.Errorf("GetTokenByHash: %w", err)
		}
	case modeMemoryToken:
		_, _, _, _, err := ops.mem.getTokenByHash(tokenHash)
		if err != nil {
			return fmt.Errorf("getTokenByHash: %w", err)
		}
	}

	// Step 5: 使用済みマーク
	switch ops.mode {
	case modeAllDB:
		if err := ops.ml.DB.MarkTokenAsUsed(tokenHash); err != nil {
			return fmt.Errorf("MarkTokenAsUsed: %w", err)
		}
	case modeMemoryToken:
		ops.mem.markTokenAsUsed(tokenHash)
	}

	// Step 6: セッション作成（常に DB）
	sessToken, sessHash := generateToken()
	if err := ops.ml.DB.SaveSession(sessToken, sessHash, email, expires); err != nil {
		return fmt.Errorf("SaveSession: %w", err)
	}

	return nil
}

// ---------------------------------------------------------------------------
// ベンチマーク: ログインフロー（フェーズ1）
// ---------------------------------------------------------------------------

// BenchmarkLoginFlow_Sequential はログインフローの DB 操作を直列で実行する（ベースライン）
func BenchmarkLoginFlow_Sequential(b *testing.B) {
	_, q, ml := setupBenchDB(b)
	mem := newMemoryTokenStore()
	expires := time.Now().Add(30 * time.Minute)

	for _, mode := range []storageMode{modeAllDB, modeMemoryToken} {
		ops := &loginOps{mode: mode, q: q, ml: ml, mem: mem}
		b.Run(mode.String(), func(b *testing.B) {
			ctx := context.Background()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := ops.doLogin(ctx, "bench@test.com", expires); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkLoginFlow_Parallel は並行ログインリクエストをシミュレートする
func BenchmarkLoginFlow_Parallel(b *testing.B) {
	_, q, ml := setupBenchDB(b)
	mem := newMemoryTokenStore()
	expires := time.Now().Add(30 * time.Minute)

	for _, mode := range []storageMode{modeAllDB, modeMemoryToken} {
		ops := &loginOps{mode: mode, q: q, ml: ml, mem: mem}
		b.Run(mode.String(), func(b *testing.B) {
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				ctx := context.Background()
				for pb.Next() {
					if err := ops.doLogin(ctx, "bench@test.com", expires); err != nil {
						b.Error(err)
						return
					}
				}
			})
		})
	}
}

// ---------------------------------------------------------------------------
// ベンチマーク: 検証フロー（フェーズ2）
// ---------------------------------------------------------------------------

// BenchmarkVerifyFlow_Parallel はトークン検証+セッション作成の並行負荷をテストする
func BenchmarkVerifyFlow_Parallel(b *testing.B) {
	_, _, ml := setupBenchDB(b)
	email := "bench@test.com"
	expires := time.Now().Add(30 * time.Minute)

	for _, mode := range []storageMode{modeAllDB, modeMemoryToken} {
		b.Run(mode.String(), func(b *testing.B) {
			mem := newMemoryTokenStore()

			// 事前にトークンを大量に作成
			tokenHashes := make([]string, b.N)
			for i := 0; i < b.N; i++ {
				token, tokenHash := generateToken()
				switch mode {
				case modeAllDB:
					if err := ml.DB.SaveToken(token, tokenHash, email, expires); err != nil {
						b.Fatalf("事前トークン作成に失敗: %v", err)
					}
				case modeMemoryToken:
					mem.saveToken(token, tokenHash, email, expires)
				}
				tokenHashes[i] = tokenHash
			}

			ops := &verifyOps{mode: mode, ml: ml, mem: mem}
			var idx atomic.Int64

			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					i := idx.Add(1) - 1
					if i >= int64(len(tokenHashes)) {
						return
					}
					if err := ops.doVerify(tokenHashes[i], email, expires); err != nil {
						b.Error(err)
						return
					}
				}
			})
		})
	}
}

// ---------------------------------------------------------------------------
// ベンチマーク: バーストテスト（フェーズ1 の同時リクエスト）
// ---------------------------------------------------------------------------

// BenchmarkBurst は N 件の同時ログインリクエストをシミュレートする
func BenchmarkBurst(b *testing.B) {
	for _, mode := range []storageMode{modeAllDB, modeMemoryToken} {
		for _, n := range []int{100, 500, 1000} {
			name := fmt.Sprintf("%s/concurrent_%d", mode, n)
			b.Run(name, func(b *testing.B) {
				benchBurst(b, mode, n)
			})
		}
	}
}

func benchBurst(b *testing.B, mode storageMode, concurrency int) {
	_, q, ml := setupBenchDB(b)
	mem := newMemoryTokenStore()

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

	ops := &loginOps{mode: mode, q: q, ml: ml, mem: mem}
	expires := time.Now().Add(30 * time.Minute)

	b.ResetTimer()
	for iter := 0; iter < b.N; iter++ {
		start := time.Now()
		errCh := make(chan error, concurrency)
		for i := 1; i <= concurrency; i++ {
			go func(userIdx int) {
				email := fmt.Sprintf("user%d@test.com", userIdx)
				errCh <- ops.doLogin(context.Background(), email, expires)
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
		b.Logf("[%s] 同時 %d 件: %d 件成功, %d 件失敗, 所要時間 %v",
			mode, concurrency, concurrency-failed, failed, elapsed)
	}
}
