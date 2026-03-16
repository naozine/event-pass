package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/naozine/nz-magic-link/magiclink"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/database"
	_ "modernc.org/sqlite"
)

// ---------------------------------------------------------------------------
// HTTP ベンチ用セットアップ
// ---------------------------------------------------------------------------

// setupHTTPBench は magiclink ハンドラ付きの Echo サーバーを構築する。
// DevBypassEmails を使い、メール送信をスキップしてレスポンスからマジックリンクを取得する。
func setupHTTPBench(b *testing.B, numUsers int) (*echo.Echo, *magiclink.MagicLink, *database.Queries) {
	b.Helper()

	dir := b.TempDir()
	dbPath := dir + "/http_bench.db"

	conn, err := sql.Open("sqlite", "file:"+dbPath+"?_pragma=busy_timeout(30000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(on)")
	if err != nil {
		b.Fatalf("DB接続に失敗: %v", err)
	}
	b.Cleanup(func() { conn.Close() })

	// users テーブル作成
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

	q := database.New(conn)

	// テストユーザーを作成し、bypass リストに追加
	bypassEmails := make(map[string]bool)
	ctx := context.Background()
	for i := 0; i < numUsers; i++ {
		email := fmt.Sprintf("user%d@test.com", i)
		_, err := q.CreateUser(ctx, database.CreateUserParams{
			Email: email, Name: fmt.Sprintf("User%d", i), Role: "viewer", IsActive: true,
		})
		if err != nil {
			b.Fatalf("ユーザー作成に失敗: %v", err)
		}
		bypassEmails[email] = true
	}

	// magiclink 初期化
	mlConfig := magiclink.DefaultConfig()
	mlConfig.DatabaseType = "sqlite"
	mlConfig.TokenExpiry = 30 * time.Minute
	mlConfig.SessionExpiry = 24 * time.Hour
	mlConfig.RedirectURL = "/projects"
	mlConfig.ErrorRedirectURL = "/auth/login"
	mlConfig.ServerAddr = "http://localhost:8080"
	mlConfig.DisableRateLimiting = true // ベンチマーク用にレート制限を無効化
	mlConfig.AllowLogin = func(c echo.Context, email string) error {
		_, err := q.GetUserByEmail(c.Request().Context(), email)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil
			}
			return fmt.Errorf("システムエラーが発生しました。")
		}
		return nil
	}

	ml, err := magiclink.NewWithDB(mlConfig, conn)
	if err != nil {
		b.Fatalf("magiclink 初期化に失敗: %v", err)
	}
	// DevBypassEmails を直接設定
	ml.DevBypassEmails = bypassEmails

	// Echo セットアップ
	e := echo.New()
	ml.RegisterHandlers(e)

	return e, ml, q
}

// ---------------------------------------------------------------------------
// HTTP リクエストヘルパー
// ---------------------------------------------------------------------------

// loginResponse は POST /auth/login のレスポンス
type loginResponse struct {
	Message   string `json:"message"`
	MagicLink string `json:"magic_link"`
}

// doHTTPLogin は POST /auth/login → マジックリンク取得を実行する
func doHTTPLogin(e *echo.Echo, email, remoteAddr string) (string, error) {
	body := fmt.Sprintf(`{"email":"%s"}`, email)
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.RemoteAddr = remoteAddr
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code == http.StatusTooManyRequests {
		return "", fmt.Errorf("rate limited (429)")
	}
	if rec.Code != http.StatusOK {
		return "", fmt.Errorf("login failed: status %d, body: %s", rec.Code, rec.Body.String())
	}

	var resp loginResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		return "", fmt.Errorf("response parse error: %w", err)
	}
	if resp.MagicLink == "" {
		return "", fmt.Errorf("magic_link is empty")
	}

	// トークンを抽出
	u, err := url.Parse(resp.MagicLink)
	if err != nil {
		return "", fmt.Errorf("magic_link URL parse error: %w", err)
	}
	return u.Query().Get("token"), nil
}

// doHTTPVerify は GET /auth/verify?token=xxx を実行し、セッション Cookie を返す
func doHTTPVerify(e *echo.Echo, token string) ([]*http.Cookie, error) {
	req := httptest.NewRequest(http.MethodGet, "/auth/verify?token="+token, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	// Verify はリダイレクト (302) を返す
	if rec.Code != http.StatusFound {
		return nil, fmt.Errorf("verify failed: status %d, body: %s", rec.Code, rec.Body.String())
	}

	// エラーリダイレクトかどうかチェック（Location に error= が含まれる）
	location := rec.Header().Get("Location")
	if strings.Contains(location, "error=") {
		return nil, fmt.Errorf("verify error redirect: %s", location)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		return nil, fmt.Errorf("no session cookie set")
	}
	return cookies, nil
}

// ---------------------------------------------------------------------------
// ベンチマーク: Verify フロー（DB 書き込みがあるパス）
// ---------------------------------------------------------------------------

// BenchmarkHTTPVerify_Sequential は Verify フローを直列で実行する
func BenchmarkHTTPVerify_Sequential(b *testing.B) {
	e, _, _ := setupHTTPBench(b, 1)
	email := "user0@test.com"

	// Login API 経由でトークンを事前取得（ハッシュ形式の不一致を回避）
	tokens := make([]string, b.N)
	for i := 0; i < b.N; i++ {
		remoteAddr := fmt.Sprintf("10.0.%d.%d:1234", (i/254)%256, (i%254)+1)
		token, err := doHTTPLogin(e, email, remoteAddr)
		if err != nil {
			b.Fatalf("トークン事前取得に失敗: %v", err)
		}
		tokens[i] = token
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := doHTTPVerify(e, tokens[i])
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkHTTPVerify_Parallel は Verify フローを並行実行する
func BenchmarkHTTPVerify_Parallel(b *testing.B) {
	e, _, _ := setupHTTPBench(b, 1)
	email := "user0@test.com"

	// Login API 経由でトークンを事前取得
	tokens := make([]string, b.N)
	for i := 0; i < b.N; i++ {
		remoteAddr := fmt.Sprintf("10.0.%d.%d:1234", (i/254)%256, (i%254)+1)
		token, err := doHTTPLogin(e, email, remoteAddr)
		if err != nil {
			b.Fatalf("トークン事前取得に失敗: %v", err)
		}
		tokens[i] = token
	}

	var idx atomic.Int64

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := idx.Add(1) - 1
			if i >= int64(len(tokens)) {
				return
			}
			_, err := doHTTPVerify(e, tokens[i])
			if err != nil {
				b.Error(err)
				return
			}
		}
	})
}

// ---------------------------------------------------------------------------
// ベンチマーク: フルフロー（Login + Verify）
// ---------------------------------------------------------------------------

// BenchmarkHTTPFullFlow_Sequential はログイン→検証の完全フローを直列実行する
func BenchmarkHTTPFullFlow_Sequential(b *testing.B) {
	e, _, _ := setupHTTPBench(b, 1)
	email := "user0@test.com"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// IP をイテレーションごとに変えてレート制限を回避
		remoteAddr := fmt.Sprintf("10.0.%d.%d:1234", (i/254)%256, (i%254)+1)

		token, err := doHTTPLogin(e, email, remoteAddr)
		if err != nil {
			b.Fatal(err)
		}

		_, err = doHTTPVerify(e, token)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// ベンチマーク: バーストテスト（HTTP レベル）
// ---------------------------------------------------------------------------

// BenchmarkHTTPBurst は同時 N 件の Verify リクエストをシミュレートする
// Login（フェーズ1）はインメモリトークンで高速なので、Verify（フェーズ2: DB書き込み）が本命
func BenchmarkHTTPBurst(b *testing.B) {
	for _, n := range []int{25, 50, 100} {
		b.Run(fmt.Sprintf("verify_%d", n), func(b *testing.B) {
			benchHTTPBurstVerify(b, n)
		})
	}
}

func benchHTTPBurstVerify(b *testing.B, concurrency int) {
	e, _, _ := setupHTTPBench(b, concurrency)

	// Login API 経由で各ユーザーのトークンを事前取得（イテレーションごと）
	// IP はグローバルカウンターで一意にし、IP レート制限（バースト10回/IP）を回避
	type tokenSet struct {
		tokens []string
	}
	sets := make([]tokenSet, concurrency)
	ipCounter := 0
	for i := 0; i < concurrency; i++ {
		email := fmt.Sprintf("user%d@test.com", i)
		sets[i].tokens = make([]string, b.N)
		for j := 0; j < b.N; j++ {
			ipCounter++
			remoteAddr := fmt.Sprintf("10.%d.%d.%d:1234", (ipCounter/65536)%256, (ipCounter/256)%256, (ipCounter%256)+1)
			token, err := doHTTPLogin(e, email, remoteAddr)
			if err != nil {
				b.Fatalf("user%d iter%d トークン事前取得に失敗: %v", i, j, err)
			}
			sets[i].tokens[j] = token
		}
	}

	b.ResetTimer()
	for iter := 0; iter < b.N; iter++ {
		start := time.Now()

		var wg sync.WaitGroup
		var failed atomic.Int64
		var errors []string
		var mu sync.Mutex

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(userIdx int) {
				defer wg.Done()
				_, err := doHTTPVerify(e, sets[userIdx].tokens[iter])
				if err != nil {
					failed.Add(1)
					mu.Lock()
					if len(errors) < 3 {
						errors = append(errors, err.Error())
					}
					mu.Unlock()
				}
			}(i)
		}

		wg.Wait()
		elapsed := time.Since(start)

		f := failed.Load()
		for _, errMsg := range errors {
			b.Logf("エラー: %s", errMsg)
		}
		b.Logf("[HTTPBurst] 同時 %d 件 Verify: %d 件成功, %d 件失敗, 所要時間 %v",
			concurrency, int64(concurrency)-f, f, elapsed)
	}
}

// BenchmarkHTTPBurstFullFlow は Login + Verify のフルフローを同時実行する
func BenchmarkHTTPBurstFullFlow(b *testing.B) {
	for _, n := range []int{25, 50} {
		b.Run(fmt.Sprintf("full_%d", n), func(b *testing.B) {
			benchHTTPBurstFull(b, n)
		})
	}
}

func benchHTTPBurstFull(b *testing.B, concurrency int) {
	e, _, _ := setupHTTPBench(b, concurrency)

	b.ResetTimer()
	for iter := 0; iter < b.N; iter++ {
		start := time.Now()

		var wg sync.WaitGroup
		var failed atomic.Int64
		var errors []string
		var mu sync.Mutex

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(userIdx int) {
				defer wg.Done()
				email := fmt.Sprintf("user%d@test.com", userIdx)
				remoteAddr := fmt.Sprintf("10.%d.%d.%d:1234", (userIdx/65536)%256, (userIdx/256)%256, (userIdx%256)+1)

				token, err := doHTTPLogin(e, email, remoteAddr)
				if err != nil {
					failed.Add(1)
					mu.Lock()
					if len(errors) < 3 {
						errors = append(errors, fmt.Sprintf("user%d login: %s", userIdx, err))
					}
					mu.Unlock()
					return
				}

				_, err = doHTTPVerify(e, token)
				if err != nil {
					failed.Add(1)
					mu.Lock()
					if len(errors) < 3 {
						errors = append(errors, fmt.Sprintf("user%d verify: %s", userIdx, err))
					}
					mu.Unlock()
				}
			}(i)
		}

		wg.Wait()
		elapsed := time.Since(start)

		f := failed.Load()
		for _, errMsg := range errors {
			b.Logf("エラー: %s", errMsg)
		}
		b.Logf("[HTTPBurstFull] 同時 %d 件 Login+Verify: %d 件成功, %d 件失敗, 所要時間 %v",
			concurrency, int64(concurrency)-f, f, elapsed)
	}
}
