package routes

import (
	"github.com/labstack/echo/v4"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/database"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/handlers"
	appMiddleware "github.com/naozine/project_crud_with_auth_tmpl/internal/middleware"
)

// RegisterBusinessRoutes はビジネスロジックのルートを登録する（派生プロジェクトでカスタマイズ可）。
// 新機能を追加する際は projects の実装（handlers, routes, templ）を参考にする。
// authMW は認証ミドルウェアで、本番では RequireAuth(ml, ...)、テストではスタブを渡す。
func RegisterBusinessRoutes(e *echo.Echo, queries *database.Queries, authMW echo.MiddlewareFunc) {
	projectHandler := handlers.NewProjectHandler(queries)

	projectGroup := e.Group("/projects")
	projectGroup.Use(authMW)

	// 読み取り — 認証済みユーザー全員
	projectGroup.GET("", projectHandler.ListProjects)
	projectGroup.GET("/:id", projectHandler.ShowProject)

	// 書き込み — admin または editor のみ
	requireWrite := appMiddleware.RequireRole("admin", "editor")
	projectGroup.GET("/new", projectHandler.NewProjectPage, requireWrite)
	projectGroup.POST("/new", projectHandler.CreateProject, requireWrite)
	projectGroup.GET("/:id/edit", projectHandler.EditProjectPage, requireWrite)
	projectGroup.POST("/:id/update", projectHandler.UpdateProject, requireWrite)
	projectGroup.POST("/:id/delete", projectHandler.DeleteProject, requireWrite)

	// ユーザー一括インポート（admin のみ）
	importHandler := handlers.NewUserImportHandler(queries)
	requireAdmin := appMiddleware.RequireRole("admin")
	e.GET("/admin/users/import", importHandler.ImportPage, authMW, requireAdmin)
	e.POST("/admin/users/import", importHandler.ExecuteImport, authMW, requireAdmin)
	e.GET("/admin/users/import/template", importHandler.TemplateDownload, authMW, requireAdmin)
}
