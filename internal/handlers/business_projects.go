package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/naozine/project_crud_with_auth_tmpl/internal/appcontext"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/database"
	"github.com/naozine/project_crud_with_auth_tmpl/web/components"

	"github.com/labstack/echo/v4"
)

type ProjectHandler struct {
	DB *database.Queries
}

func NewProjectHandler(db *database.Queries) *ProjectHandler {
	return &ProjectHandler{DB: db}
}

// checkPermission checks if the current user has write access (admin or editor)
func (h *ProjectHandler) checkPermission(c echo.Context) error {
	role := appcontext.GetUserRole(c.Request().Context())
	if role != "admin" && role != "editor" {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied: Write permission required")
	}
	return nil
}

func (h *ProjectHandler) ListProjects(c echo.Context) error {
	ctx := c.Request().Context()
	projects, err := h.DB.ListProjects(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}
	return renderPage(c, "プロジェクト一覧", components.ProjectList(projects))
}

func (h *ProjectHandler) NewProjectPage(c echo.Context) error {
	if err := h.checkPermission(c); err != nil {
		return err
	}
	return renderPage(c, "新規プロジェクト作成", components.ProjectForm())
}

func (h *ProjectHandler) CreateProject(c echo.Context) error {
	if err := h.checkPermission(c); err != nil {
		return err
	}
	ctx := c.Request().Context()
	name := c.FormValue("name")
	_, err := h.DB.CreateProject(ctx, name)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}
	return c.Redirect(http.StatusSeeOther, "/projects")
}

func (h *ProjectHandler) ShowProject(c echo.Context) error {
	ctx := c.Request().Context()
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid ID")
	}

	project, err := h.DB.GetProject(ctx, int64(id))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Project not found")
	}

	return renderPage(c, project.Name, components.ProjectDetail(project))
}

func (h *ProjectHandler) EditProjectPage(c echo.Context) error {
	if err := h.checkPermission(c); err != nil {
		return err
	}
	ctx := c.Request().Context()
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid ID")
	}

	project, err := h.DB.GetProject(ctx, int64(id))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Project not found")
	}

	return renderPage(c, "編集: "+project.Name, components.ProjectEdit(project))
}

func (h *ProjectHandler) UpdateProject(c echo.Context) error {
	if err := h.checkPermission(c); err != nil {
		return err
	}
	ctx := c.Request().Context()
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid ID")
	}

	name := c.FormValue("name")
	_, err = h.DB.UpdateProject(ctx, database.UpdateProjectParams{
		Name: name,
		ID:   int64(id),
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/projects/%d", id))
}

func (h *ProjectHandler) DeleteProject(c echo.Context) error {
	if err := h.checkPermission(c); err != nil {
		return err
	}
	ctx := c.Request().Context()
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid ID")
	}

	err = h.DB.DeleteProject(ctx, int64(id))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	return c.Redirect(http.StatusSeeOther, "/projects")
}
