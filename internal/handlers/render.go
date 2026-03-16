package handlers

import (
	"github.com/a-h/templ"
	"github.com/labstack/echo/v4"
	"github.com/naozine/project_crud_with_auth_tmpl/web/layouts"
)

// renderPage はテンプレートの描画を共通化するヘルパー。
// HX-Request ヘッダがある場合は部分HTMLを、そうでなければ Base レイアウトで
// ラップした全体HTMLを返す。
func renderPage(c echo.Context, title string, content templ.Component) error {
	ctx := c.Request().Context()
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTML)
	if c.Request().Header.Get("HX-Request") == "true" {
		return content.Render(ctx, c.Response().Writer)
	}
	return layouts.Base(title, content).Render(ctx, c.Response().Writer)
}
