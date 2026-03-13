package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/appcontext"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/database"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/logger"
	"github.com/naozine/project_crud_with_auth_tmpl/web/components"
)

type AdminHandler struct {
	Queries *database.Queries
}

func NewAdminHandler(queries *database.Queries) *AdminHandler {
	return &AdminHandler{Queries: queries}
}

// EnsureAdmin checks if the current user is an admin
func (h *AdminHandler) checkAdmin(c echo.Context) error {
	role := appcontext.GetUserRole(c.Request().Context())
	if role != "admin" {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied: Admin role required")
	}
	return nil
}

func (h *AdminHandler) ListUsers(c echo.Context) error {
	if err := h.checkAdmin(c); err != nil {
		return err
	}

	users, err := h.Queries.ListUsers(c.Request().Context())
	if err != nil {
		logger.Error("Failed to list users", "error", err)
		return c.String(http.StatusInternalServerError, "Failed to list users")
	}

	return renderPage(c, "ユーザー管理", components.UserList(users))
}

func (h *AdminHandler) NewUserPage(c echo.Context) error {
	if err := h.checkAdmin(c); err != nil {
		return err
	}
	return components.UserForm().Render(c.Request().Context(), c.Response().Writer)
}

func (h *AdminHandler) CreateUser(c echo.Context) error {
	if err := h.checkAdmin(c); err != nil {
		return err
	}

	name := c.FormValue("name")
	email := c.FormValue("email")
	role := c.FormValue("role")

	if name == "" || email == "" || role == "" {
		return c.String(http.StatusBadRequest, "Name, email and role are required")
	}

	// Validate role
	if role != "admin" && role != "editor" && role != "viewer" {
		return c.String(http.StatusBadRequest, "Invalid role")
	}

	_, err := h.Queries.CreateUser(c.Request().Context(), database.CreateUserParams{
		Email:    email,
		Name:     name,
		Role:     role,
		IsActive: true,
	})

	if err != nil {
		logger.Error("Failed to create user", "error", err, "email", email)
		// In a real app, handle duplicate email error specifically
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Failed to create user: %v", err))
	}

	// Return updated user list wrapped in Base, because HTMX target is main
	// We could just return UserList if target was the list container
	// But UserForm replaces "main" if we are not careful, wait.
	// UserForm hx-target="main" -> this replaces the <main> content with the response.
	// So we should return the UserList component.

	users, err := h.Queries.ListUsers(c.Request().Context())
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to list users")
	}

	return components.UserList(users).Render(c.Request().Context(), c.Response().Writer)
}

func (h *AdminHandler) EditUserPage(c echo.Context) error {
	if err := h.checkAdmin(c); err != nil {
		return err
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.String(http.StatusBadRequest, "Invalid ID")
	}

	// We need GetUserByID query, currently we only have GetUserByEmail.
	// Let's assume GetUserByEmail works or add GetUser query?
	// The generated code might not have GetUserByID if we didn't put it in query.sql.
	// Let's check query.sql or add it.
	// Wait, we didn't add GetUserByID in query.sql. We have GetUserByEmail.
	// We should add GetUserByID. But to avoid switching context, let's fetch all users and filter (inefficient)
	// OR better, let's assume the ID is passed and we can add the query.
	// I'll add the query first. No wait, I cannot change file inside this replace block.
	// I will check if I can update user by ID without fetching first?
	// Edit page NEEDS current data.
	// I'll iterate list for now as fallback, it's fast enough for small user base.

	users, err := h.Queries.ListUsers(c.Request().Context())
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to fetch users")
	}

	var targetUser database.User
	found := false
	for _, u := range users {
		if u.ID == int64(id) {
			targetUser = u
			found = true
			break
		}
	}

	if !found {
		return c.String(http.StatusNotFound, "User not found")
	}

	return components.UserEditForm(targetUser).Render(c.Request().Context(), c.Response().Writer)
}

func (h *AdminHandler) UpdateUser(c echo.Context) error {
	if err := h.checkAdmin(c); err != nil {
		return err
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.String(http.StatusBadRequest, "Invalid ID")
	}

	name := c.FormValue("name")
	role := c.FormValue("role")
	statusStr := c.FormValue("status")
	isActive := statusStr == "active"

	_, err = h.Queries.UpdateUser(c.Request().Context(), database.UpdateUserParams{
		Name:     name,
		Role:     role,
		IsActive: isActive,
		ID:       int64(id),
	})
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to update user")
	}

	// Return updated list
	users, err := h.Queries.ListUsers(c.Request().Context())
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to list users")
	}
	return components.UserList(users).Render(c.Request().Context(), c.Response().Writer)
}

func (h *AdminHandler) DeleteUser(c echo.Context) error {
	if err := h.checkAdmin(c); err != nil {
		return err
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.String(http.StatusBadRequest, "Invalid ID")
	}

	// Prevent self-deletion
	currentUserID := appcontext.GetUserID(c.Request().Context())
	if int64(id) == currentUserID {
		return c.String(http.StatusBadRequest, "自分自身を削除することはできません。")
	}

	err = h.Queries.DeleteUser(c.Request().Context(), int64(id))
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to delete user")
	}

	// Return updated list
	users, err := h.Queries.ListUsers(c.Request().Context())
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to list users")
	}
	return components.UserList(users).Render(c.Request().Context(), c.Response().Writer)
}
