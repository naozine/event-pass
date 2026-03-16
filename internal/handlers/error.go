package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/logger"
	"github.com/naozine/project_crud_with_auth_tmpl/web/components"
	"github.com/naozine/project_crud_with_auth_tmpl/web/layouts"
)

// CustomHTTPErrorHandler はHTTPエラーをHTMLページとして表示するカスタムエラーハンドラ
func CustomHTTPErrorHandler(err error, c echo.Context) {
	code := http.StatusInternalServerError
	message := "予期しないエラーが発生しました"

	// Echo HTTPError の場合はステータスコードとメッセージを取得
	if he, ok := err.(*echo.HTTPError); ok {
		code = he.Code
		if m, ok := he.Message.(string); ok {
			message = m
		}
	}

	// メッセージのデフォルト値を設定
	if message == "" || message == "Not Found" {
		switch code {
		case http.StatusNotFound:
			message = "お探しのページは存在しません"
		case http.StatusForbidden:
			message = "このページへのアクセス権限がありません"
		case http.StatusInternalServerError:
			message = "サーバーで問題が発生しました"
		}
	}

	// 500番台のエラーはログに記録
	if code >= 500 {
		logger.Error("HTTP error",
			"status", code,
			"method", c.Request().Method,
			"path", c.Request().URL.Path,
			"error", err.Error(),
		)
	}

	// レスポンスがすでにコミットされている場合は何もしない
	if c.Response().Committed {
		return
	}

	// HTMLレスポンスを返す
	c.Response().WriteHeader(code)
	errorPage := components.ErrorPage(code, message)
	page := layouts.Base("エラー", errorPage)
	_ = page.Render(c.Request().Context(), c.Response())
}
