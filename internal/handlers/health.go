package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/version"
)

// HealthResponse はヘルスチェックのレスポンス
type HealthResponse struct {
	Status    string `json:"status"`
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
}

// HealthCheck はアプリケーションのヘルス状態を返す
func HealthCheck(c echo.Context) error {
	return c.JSON(http.StatusOK, HealthResponse{
		Status:    "ok",
		Version:   version.Version,
		Commit:    version.Commit,
		BuildDate: version.BuildDate,
	})
}
