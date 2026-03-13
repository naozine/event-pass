// Package integration はエンドツーエンドの統合テスト基盤を提供する。
// インメモリ SQLite + Echo ルート登録により、magiclink に依存せず
// ハンドラの動作と権限マトリクスを検証できる。
package integration

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/appcontext"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/database"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/handlers"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/routes"
	"github.com/pressly/goose/v3"

	db "github.com/naozine/project_crud_with_auth_tmpl/db"
	_ "modernc.org/sqlite"
)

// SeedData はテスト用の初期データを保持する
type SeedData struct {
	AdminUser      database.User
	EditorUser     database.User
	ViewerUser     database.User
	DeletableUser  database.User // 削除テスト用の使い捨てユーザー
	Project        database.Project
}

// SetupTestDB はインメモリ SQLite を作成し、マイグレーションを適用する
func SetupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := sql.Open("sqlite", "file::memory:?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(on)")
	if err != nil {
		t.Fatalf("DB接続に失敗: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	goose.SetBaseFS(db.MigrationsFS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("goose dialect設定に失敗: %v", err)
	}
	if err := goose.Up(conn, "migrations"); err != nil {
		t.Fatalf("マイグレーションに失敗: %v", err)
	}
	return conn
}

// SeedTestData はテスト用のユーザーとプロジェクトを作成する
func SeedTestData(t *testing.T, conn *sql.DB) SeedData {
	t.Helper()
	q := database.New(conn)
	ctx := context.Background()

	adminUser, err := q.CreateUser(ctx, database.CreateUserParams{
		Email: "admin@test.com", Name: "Admin", Role: "admin", IsActive: true,
	})
	if err != nil {
		t.Fatalf("adminユーザー作成に失敗: %v", err)
	}

	editorUser, err := q.CreateUser(ctx, database.CreateUserParams{
		Email: "editor@test.com", Name: "Editor", Role: "editor", IsActive: true,
	})
	if err != nil {
		t.Fatalf("editorユーザー作成に失敗: %v", err)
	}

	viewerUser, err := q.CreateUser(ctx, database.CreateUserParams{
		Email: "viewer@test.com", Name: "Viewer", Role: "viewer", IsActive: true,
	})
	if err != nil {
		t.Fatalf("viewerユーザー作成に失敗: %v", err)
	}

	deletableUser, err := q.CreateUser(ctx, database.CreateUserParams{
		Email: "deletable@test.com", Name: "Deletable", Role: "viewer", IsActive: true,
	})
	if err != nil {
		t.Fatalf("削除用ユーザー作成に失敗: %v", err)
	}

	project, err := q.CreateProject(ctx, "テストプロジェクト")
	if err != nil {
		t.Fatalf("プロジェクト作成に失敗: %v", err)
	}

	return SeedData{
		AdminUser:     adminUser,
		EditorUser:    editorUser,
		ViewerUser:    viewerUser,
		DeletableUser: deletableUser,
		Project:       project,
	}
}

// testRequireAuth は magiclink を使わず appcontext からログイン状態を判定するテスト用ミドルウェア
func testRequireAuth(loginURL string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			_, isLoggedIn, _ := appcontext.GetUser(c.Request().Context())
			if !isLoggedIn {
				return c.Redirect(http.StatusSeeOther, loginURL)
			}
			return next(c)
		}
	}
}

// testUserContextMiddleware は X-Test-User-ID ヘッダからユーザー情報を復元するテスト用ミドルウェア
func testUserContextMiddleware(queries *database.Queries) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userIDStr := c.Request().Header.Get("X-Test-User-ID")
			if userIDStr != "" {
				var userID int64
				if _, err := fmt.Sscanf(userIDStr, "%d", &userID); err == nil {
					u, err := queries.GetUserByID(c.Request().Context(), userID)
					if err == nil {
						ctx := appcontext.WithUser(c.Request().Context(), u.Email, true, false, u.Role, u.ID)
						c.SetRequest(c.Request().WithContext(ctx))
					}
				}
			}
			return next(c)
		}
	}
}

// SetupTestServer は統合テスト用の Echo インスタンスを作成し、ルートを登録する。
// 本番と同じ routes.Register* を使い、認証ミドルウェアのみテスト用に差し替える。
func SetupTestServer(t *testing.T, conn *sql.DB) *echo.Echo {
	t.Helper()
	queries := database.New(conn)

	e := echo.New()
	e.HTTPErrorHandler = handlers.CustomHTTPErrorHandler
	e.Use(testUserContextMiddleware(queries))

	authMW := testRequireAuth("/auth/login")
	routes.RegisterBusinessRoutes(e, queries, authMW)
	routes.RegisterAdminRoutes(e, queries, authMW)

	return e
}

// DoRequest は指定ロールで HTTP リクエストを実行し、レスポンスを返す。
// user が nil の場合は未認証リクエストとなる。
func DoRequest(e *echo.Echo, method, path string, user *database.User, body ...string) *httptest.ResponseRecorder {
	var req *http.Request
	if len(body) > 0 && body[0] != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body[0]))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	} else {
		req = httptest.NewRequest(method, path, nil)
	}

	if user != nil {
		req.Header.Set("X-Test-User-ID", fmt.Sprintf("%d", user.ID))
	}

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}
