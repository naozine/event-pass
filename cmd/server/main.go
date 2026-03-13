package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/naozine/project_crud_with_auth_tmpl/db"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/database"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/handlers"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/logger"
	appMiddleware "github.com/naozine/project_crud_with_auth_tmpl/internal/middleware"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/routes"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/version"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/naozine/nz-magic-link/magiclink"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found or could not be loaded, using OS environment variables.")
	}

	// Initialize Logger
	logConfig := logger.DefaultConfig()
	logConfig.LogDir = os.Getenv("LOG_DIR") // 空ならstdout/stderr
	if err := logger.Init(logConfig); err != nil {
		log.Fatal("Failed to initialize logger:", err)
	}
	defer logger.Close()

	// 1. Database Setup (for projects)
	// DB_PATH 環境変数でパスを指定可能（Docker ボリューム対応）
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "app.db"
	}
	conn, err := sql.Open("sqlite", "file:"+dbPath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(on)")
	if err != nil {
		log.Fatal("Failed to connect to app.db:", err)
	}
	defer conn.Close()

	// Run Migrations (using goose)
	goose.SetBaseFS(db.MigrationsFS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		log.Fatal("Failed to set goose dialect:", err)
	}
	// "migrations" is the directory name inside embed.FS (db/migrations)
	// Since db.MigrationsFS is rooted at "db", the path is just "migrations"
	if err := goose.Up(conn, "migrations"); err != nil {
		log.Fatal("Failed to apply migrations:", err)
	}

	// Initialize Admin User if needed
	if err := ensureAdminUser(conn); err != nil {
		log.Printf("Warning: Failed to initialize admin user: %v", err)
	}

	// 2. MagicLink Setup
	mlConfig := magiclink.DefaultConfig()
	// Use existing SQLite connection, so DatabasePath is not used for connection but kept for config consistency
	mlConfig.DatabaseType = "sqlite"

	// ServerAddr: 開発時はPORT環境変数から動的生成、本番はビルド時注入
	mlConfig.ServerAddr = resolveServerAddr()

	// 開発モードかつ HTTP の場合のみ CookieSecure を false にする
	// （本番で誤って HTTP を設定しても CookieSecure は true のまま）
	if os.Getenv("APP_ENV") == "dev" && strings.HasPrefix(mlConfig.ServerAddr, "http://") {
		mlConfig.CookieSecure = false
	}

	// Only use bypass file if it exists (mainly for local development)
	if _, err := os.Stat(".bypass_emails"); err == nil {
		mlConfig.DevBypassEmailFilePath = ".bypass_emails"
	}

	mlConfig.RedirectURL = "/projects"        // Redirect to projects list after login
	mlConfig.ErrorRedirectURL = "/auth/login" // Redirect to login page on error
	mlConfig.LoginSuccessMessage = "ログイン用のメールを送信しました"

	// CookieName を ProjectName から生成（派生プロジェクト間のクッキー衝突を防ぐ）
	mlConfig.CookieName = generateCookieName(version.ProjectName)

	// AllowLogin callback to check against users table
	mlConfig.AllowLogin = func(c echo.Context, email string) error {
		// We need to access the database here. Since we can't easily pass the queries object directly
		// into this config function definition before it's created, we'll create a new queries instance
		// or rely on a closure if we restructure.
		// However, `queries` is created *after* this config.
		// To fix this dependency cycle, we can defer the actual DB check or restructure initialization.
		// A simple way is to use the `conn` we already have.

		q := database.New(conn)
		user, err := q.GetUserByEmail(c.Request().Context(), email)
		if err != nil {
			if err == sql.ErrNoRows {
				return fmt.Errorf("このメールアドレスは登録されていません。")
			}
			logger.Error("Database error in AllowLogin", "error", err, "email", email)
			return fmt.Errorf("システムエラーが発生しました。")
		}

		if !user.IsActive {
			return fmt.Errorf("このアカウントは無効化されています。")
		}

		return nil
	}

	// Configure SMTP
	mlConfig.SMTPHost = os.Getenv("SMTP_HOST")
	mlConfig.SMTPPort = mustAtoi(os.Getenv("SMTP_PORT"), 587)
	mlConfig.SMTPUsername = os.Getenv("SMTP_USERNAME")
	mlConfig.SMTPPassword = os.Getenv("SMTP_PASSWORD")
	mlConfig.SMTPFrom = os.Getenv("SMTP_FROM")
	mlConfig.SMTPFromName = os.Getenv("SMTP_FROM_NAME")

	// WebAuthn Configuration（ServerAddr から自動導出）
	mlConfig.WebAuthnEnabled = true
	mlConfig.WebAuthnRPID = extractHost(mlConfig.ServerAddr)
	mlConfig.WebAuthnRPName = os.Getenv("WEBAUTHN_RP_NAME")
	if mlConfig.WebAuthnRPName == "" {
		mlConfig.WebAuthnRPName = "Project CRUD"
	}
	mlConfig.WebAuthnRedirectURL = "/projects" // Redirect to projects list after passkey login
	mlConfig.WebAuthnAllowedOrigins = []string{mlConfig.ServerAddr}

	// Allow business logic to configure MagicLink settings
	ConfigureBusinessSettings(&mlConfig)

	// Initialize MagicLink with existing DB connection
	ml, err := magiclink.NewWithDB(mlConfig, conn)
	if err != nil {
		log.Fatal("Failed to initialize MagicLink:", err)
	}

	// 3. Initialize Handlers
	queries := database.New(conn)
	authHandler := handlers.NewAuthHandler(ml)
	profileHandler := handlers.NewProfileHandler(queries, ml)
	setupHandler := handlers.NewSetupHandler(queries, ml)

	// 4. Echo Setup
	e := echo.New()
	e.HTTPErrorHandler = handlers.CustomHTTPErrorHandler // カスタムエラーハンドラ
	e.Use(appMiddleware.AccessLogMiddleware(logger.AccessWriter()))
	e.Use(middleware.Recover())
	e.Use(appMiddleware.UserContextMiddleware(ml, conn)) // Add UserContext middleware globally

	e.Static("/static", "web/static")

	// 5. Routes
	// Health Check (公開エンドポイント)
	e.GET("/health", handlers.HealthCheck)

	// Public Routes
	e.GET("/", func(c echo.Context) error {
		return c.Redirect(http.StatusSeeOther, "/auth/login")
	})
	e.GET("/auth/login", authHandler.LoginPage)

	// Initial Setup Routes (only accessible when no users exist)
	e.GET("/setup", setupHandler.SetupPage)
	e.POST("/setup", setupHandler.CreateInitialAdmin)

	// MagicLink internal handlers
	ml.RegisterHandlers(e)

	// Register Business Logic Routes
	authMW := appMiddleware.RequireAuth(ml, "/auth/login")
	routes.RegisterBusinessRoutes(e, queries, authMW)
	routes.RegisterAdminRoutes(e, queries, authMW)

	// Profile Routes
	e.GET("/profile", profileHandler.ShowProfile, appMiddleware.RequireAuth(ml, "/auth/login"))
	e.POST("/profile", profileHandler.UpdateProfile, appMiddleware.RequireAuth(ml, "/auth/login"))
	e.POST("/profile/passkeys/delete", profileHandler.DeletePasskeys, appMiddleware.RequireAuth(ml, "/auth/login"))

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Fatal(e.Start(":" + port))
}

func mustAtoi(s string, defaultValue int) int {
	if s == "" {
		return defaultValue
	}
	i, err := strconv.Atoi(s)
	if err != nil {
		log.Printf("Warning: Could not parse %q as int, using default %d. Error: %v", s, defaultValue, err)
		return defaultValue
	}
	return i
}

func ensureAdminUser(conn *sql.DB) error {
	// Check if any user exists
	var count int
	err := conn.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to count users: %w", err)
	}

	if count > 0 {
		return nil // Users already exist, skip
	}

	// No users, check for env var
	adminEmail := os.Getenv("ADMIN_EMAIL")
	if adminEmail == "" {
		log.Println("No users found and ADMIN_EMAIL not set. Skipping admin creation.")
		return nil
	}

	adminName := os.Getenv("ADMIN_NAME")
	if adminName == "" {
		adminName = "Admin"
	}

	log.Printf("Creating initial admin user: %s (%s)", adminName, adminEmail)

	q := database.New(conn)
	_, err = q.CreateUser(context.Background(), database.CreateUserParams{
		Email:    adminEmail,
		Name:     adminName,
		Role:     "admin",
		IsActive: true,
	})
	if err != nil {
		return fmt.Errorf("failed to create admin user: %w", err)
	}

	log.Println("Initial admin user created successfully.")
	return nil
}

// generateCookieName は ProjectName からクッキー名を生成する
// クッキー名は英数字とアンダースコアのみ使用可能（RFC 6265準拠）
func generateCookieName(projectName string) string {
	// 小文字に変換
	name := strings.ToLower(projectName)
	// 英数字以外をアンダースコアに置換
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	name = reg.ReplaceAllString(name, "_")
	// 先頭・末尾のアンダースコアを削除
	name = strings.Trim(name, "_")
	// 空になった場合はデフォルト値
	if name == "" {
		name = "app"
	}
	return name + "_session"
}

// resolveServerAddr は ServerAddr を解決する
// 優先順位: 1. SERVER_ADDR環境変数 2. 開発モード時はlocalhost 3. ビルド時注入値
func resolveServerAddr() string {
	// SERVER_ADDR 環境変数が設定されていれば最優先で使用
	// LAN内アクセス等で localhost 以外を使いたい場合に有用
	if serverAddr := os.Getenv("SERVER_ADDR"); serverAddr != "" {
		return serverAddr
	}

	// 開発モード（Air: APP_ENV=dev）なら PORT から動的生成
	if os.Getenv("APP_ENV") == "dev" {
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
		return "http://localhost:" + port
	}

	// 本番：ビルド時注入値を使用
	return version.ServerAddr
}

// extractHost は URL からホスト名を抽出する（WebAuthn RP ID 用）
// 例: "https://example.com:8080" -> "example.com"
func extractHost(serverAddr string) string {
	u, err := url.Parse(serverAddr)
	if err != nil || u.Host == "" {
		return "localhost"
	}
	// ポート番号を除去
	host := u.Hostname()
	if host == "" {
		return "localhost"
	}
	return host
}
