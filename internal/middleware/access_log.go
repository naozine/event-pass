package middleware

import (
	"encoding/json"
	"io"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/appcontext"
)

// AccessLogEntry はアクセスログの1エントリ
type AccessLogEntry struct {
	// Loki標準 (検索・フィルタリング用)
	Time  string `json:"time"`
	Level string `json:"level"`
	Msg   string `json:"msg"`

	// 追跡 (Correlation)
	TraceID    string `json:"trace_id,omitempty"`
	CFRay      string `json:"cf_ray,omitempty"`
	UpstreamID string `json:"upstream_id,omitempty"`

	// HTTP基本情報
	Method    string `json:"method"`
	Path      string `json:"path"`
	Status    int    `json:"status"`
	LatencyMs int64  `json:"latency_ms"`
	IP        string `json:"ip"`
	UA        string `json:"ua,omitempty"`

	// アプリケーション情報
	UserID string `json:"user_id,omitempty"`
	Htmx   bool   `json:"htmx"`
	Error  string `json:"error,omitempty"`
}

// getLogLevel はステータスコードとエラーからログレベルを判定する
func getLogLevel(status int, err error) string {
	if err != nil || status >= 500 {
		return "ERROR"
	}
	if status >= 400 {
		return "WARN"
	}
	return "INFO"
}

// getTraceID はトレースIDを決定する（CF-Ray優先、なければX-Request-ID）
func getTraceID(cfRay, upstreamID string) string {
	if cfRay != "" {
		return cfRay
	}
	return upstreamID
}

// getErrorMessage はエラーメッセージを取得する
func getErrorMessage(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}

// AccessLogMiddleware はJSON形式のアクセスログを出力するミドルウェア
func AccessLogMiddleware(out io.Writer) echo.MiddlewareFunc {
	encoder := json.NewEncoder(out)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			// 次のハンドラを実行
			err := next(c)

			// レイテンシ計算
			latency := time.Since(start)

			// ユーザー情報を取得
			userEmail, _, _ := appcontext.GetUser(c.Request().Context())

			// ヘッダーから追跡IDを取得
			cfRay := c.Request().Header.Get("CF-Ray")
			upstreamID := c.Request().Header.Get("X-Request-ID")

			// ステータスコード取得
			status := c.Response().Status

			// ログエントリを作成
			entry := AccessLogEntry{
				// Loki標準
				Time:  start.UTC().Format(time.RFC3339),
				Level: getLogLevel(status, err),
				Msg:   "http_request",

				// 追跡
				TraceID:    getTraceID(cfRay, upstreamID),
				CFRay:      cfRay,
				UpstreamID: upstreamID,

				// HTTP基本情報
				Method:    c.Request().Method,
				Path:      c.Request().URL.Path,
				Status:    status,
				LatencyMs: latency.Milliseconds(),
				IP:        c.RealIP(),
				UA:        c.Request().UserAgent(),

				// アプリケーション情報
				UserID: userEmail,
				Htmx:   c.Request().Header.Get("HX-Request") != "",
				Error:  getErrorMessage(err),
			}

			// JSON出力
			_ = encoder.Encode(entry)

			return err
		}
	}
}
