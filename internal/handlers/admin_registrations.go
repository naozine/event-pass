package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/database"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/logger"
	"github.com/naozine/project_crud_with_auth_tmpl/web/components"
)

type RegistrationAdminHandler struct {
	Queries *database.Queries
}

func NewRegistrationAdminHandler(queries *database.Queries) *RegistrationAdminHandler {
	return &RegistrationAdminHandler{Queries: queries}
}

func (h *RegistrationAdminHandler) ListRegistrations(c echo.Context) error {
	ctx := c.Request().Context()
	eventID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid ID")
	}

	event, err := h.Queries.GetEvent(ctx, eventID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Event not found")
	}

	registrations, err := h.Queries.ListRegistrationsByEvent(ctx, eventID)
	if err != nil {
		logger.Error("failed to list registrations", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to load registrations")
	}

	return renderPage(c, event.Title+" の登録一覧", components.AdminRegistrationList(event, registrations))
}

func (h *RegistrationAdminHandler) UpdateStatus(c echo.Context) error {
	ctx := c.Request().Context()
	eventID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid event ID")
	}

	regID, err := strconv.ParseInt(c.Param("reg_id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid registration ID")
	}

	status := c.FormValue("status")
	if status != "registered" && status != "cancelled" {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid status")
	}

	_, err = h.Queries.UpdateRegistrationStatus(ctx, database.UpdateRegistrationStatusParams{
		ID:     regID,
		Status: status,
	})
	if err != nil {
		logger.Error("failed to update registration status", "error", err, "id", regID)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to update status")
	}

	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/events/%d/registrations", eventID))
}

func (h *RegistrationAdminHandler) ExportCSV(c echo.Context) error {
	ctx := c.Request().Context()
	eventID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid ID")
	}

	event, err := h.Queries.GetEvent(ctx, eventID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Event not found")
	}

	registrations, err := h.Queries.ListRegistrationsByEvent(ctx, eventID)
	if err != nil {
		logger.Error("failed to list registrations for CSV", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to export")
	}

	filename := fmt.Sprintf("event_%d_registrations.csv", event.ID)
	c.Response().Header().Set("Content-Type", "text/csv; charset=utf-8")
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Response().WriteHeader(http.StatusOK)

	// BOM for Excel compatibility
	c.Response().Write([]byte{0xEF, 0xBB, 0xBF})

	w := csv.NewWriter(c.Response().Writer)
	w.Write([]string{"ID", "Name", "Email", "Status", "Registered At"})
	for _, r := range registrations {
		createdAt := ""
		if r.CreatedAt.Valid {
			createdAt = r.CreatedAt.Time.Format("2006-01-02 15:04:05")
		}
		w.Write([]string{
			fmt.Sprintf("%d", r.ID),
			r.Name,
			r.UserEmail,
			r.Status,
			createdAt,
		})
	}
	w.Flush()
	return nil
}

func (h *RegistrationAdminHandler) Dashboard(c echo.Context) error {
	ctx := c.Request().Context()
	events, err := h.Queries.ListAllEvents(ctx)
	if err != nil {
		logger.Error("failed to list events for dashboard", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to load dashboard")
	}

	return renderPage(c, "ダッシュボード", components.AdminDashboard(events))
}
