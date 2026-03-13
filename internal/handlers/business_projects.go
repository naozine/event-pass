package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/database"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/logger"
	"github.com/naozine/project_crud_with_auth_tmpl/web/components"
)

type ProjectHandler struct {
	DB *database.Queries
}

func NewProjectHandler(db *database.Queries) *ProjectHandler {
	return &ProjectHandler{DB: db}
}

func (h *ProjectHandler) ListProjects(c echo.Context) error {
	ctx := c.Request().Context()
	projects, err := h.DB.ListProjects(ctx)
	if err != nil {
		logger.Error("プロジェクト一覧の取得に失敗", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "プロジェクト一覧の取得に失敗しました")
	}
	return renderPage(c, "プロジェクト一覧", components.ProjectList(projects))
}

func (h *ProjectHandler) NewProjectPage(c echo.Context) error {
	return renderPage(c, "新規プロジェクト作成", components.ProjectForm())
}

func (h *ProjectHandler) CreateProject(c echo.Context) error {
	ctx := c.Request().Context()
	name := c.FormValue("name")
	_, err := h.DB.CreateProject(ctx, name)
	if err != nil {
		logger.Error("プロジェクト作成に失敗", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "プロジェクトの作成に失敗しました")
	}
	return c.Redirect(http.StatusSeeOther, "/projects")
}

func (h *ProjectHandler) ShowProject(c echo.Context) error {
	ctx := c.Request().Context()
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "無効なIDです")
	}

	project, err := h.DB.GetProject(ctx, int64(id))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "プロジェクトが見つかりません")
	}

	return renderPage(c, project.Name, components.ProjectDetail(project))
}

func (h *ProjectHandler) EditProjectPage(c echo.Context) error {
	ctx := c.Request().Context()
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "無効なIDです")
	}

	project, err := h.DB.GetProject(ctx, int64(id))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "プロジェクトが見つかりません")
	}

	return renderPage(c, "編集: "+project.Name, components.ProjectEdit(project))
}

func (h *ProjectHandler) UpdateProject(c echo.Context) error {
	ctx := c.Request().Context()
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "無効なIDです")
	}

	name := c.FormValue("name")
	_, err = h.DB.UpdateProject(ctx, database.UpdateProjectParams{
		Name: name,
		ID:   int64(id),
	})
	if err != nil {
		logger.Error("プロジェクト更新に失敗", "error", err, "id", id)
		return echo.NewHTTPError(http.StatusInternalServerError, "プロジェクトの更新に失敗しました")
	}

	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/projects/%d", id))
}

func (h *ProjectHandler) DeleteProject(c echo.Context) error {
	ctx := c.Request().Context()
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "無効なIDです")
	}

	err = h.DB.DeleteProject(ctx, int64(id))
	if err != nil {
		logger.Error("プロジェクト削除に失敗", "error", err, "id", id)
		return echo.NewHTTPError(http.StatusInternalServerError, "プロジェクトの削除に失敗しました")
	}

	return c.Redirect(http.StatusSeeOther, "/projects")
}
