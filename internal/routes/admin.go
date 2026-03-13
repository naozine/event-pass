// Package routes はアプリケーションのルート登録を行う。
// 認証ミドルウェアを引数で受け取ることで、本番とテストで差し替え可能にしている。
package routes

import (
	"github.com/labstack/echo/v4"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/database"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/handlers"
	appMiddleware "github.com/naozine/project_crud_with_auth_tmpl/internal/middleware"
)

// RegisterAdminRoutes は管理者用ルートを登録する（コア — 変更不要）。
func RegisterAdminRoutes(e *echo.Echo, queries *database.Queries, authMW echo.MiddlewareFunc) {
	adminHandler := handlers.NewAdminHandler(queries)

	adminGroup := e.Group("/admin")
	adminGroup.Use(authMW)
	adminGroup.Use(appMiddleware.RequireRole("admin"))

	adminGroup.GET("/users", adminHandler.ListUsers)
	adminGroup.GET("/users/new", adminHandler.NewUserPage)
	adminGroup.POST("/users", adminHandler.CreateUser)
	adminGroup.GET("/users/:id/edit", adminHandler.EditUserPage)
	adminGroup.POST("/users/:id", adminHandler.UpdateUser)
	adminGroup.DELETE("/users/:id", adminHandler.DeleteUser)
}
