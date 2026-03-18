package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/appcontext"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/database"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/logger"
	"github.com/naozine/project_crud_with_auth_tmpl/web/components"
)

type PublicEventHandler struct {
	Queries *database.Queries
}

func NewPublicEventHandler(queries *database.Queries) *PublicEventHandler {
	return &PublicEventHandler{Queries: queries}
}

func (h *PublicEventHandler) ListUpcomingEvents(c echo.Context) error {
	ctx := c.Request().Context()
	events, err := h.Queries.ListPublishedEvents(ctx)
	if err != nil {
		logger.Error("failed to list published events", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to load events")
	}
	return renderPage(c, "イベント一覧", components.EventList(events))
}

func (h *PublicEventHandler) ShowEventDetail(c echo.Context) error {
	ctx := c.Request().Context()
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid ID")
	}

	event, err := h.Queries.GetEvent(ctx, id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Event not found")
	}

	if !event.IsPublished {
		return echo.NewHTTPError(http.StatusNotFound, "Event not found")
	}

	regCount, err := h.Queries.CountRegistrationsByEvent(ctx, id)
	if err != nil {
		logger.Error("failed to count registrations", "error", err)
		regCount = 0
	}

	return renderPage(c, event.Title, components.EventDetail(event, regCount))
}

func (h *PublicEventHandler) ShowRegistrationForm(c echo.Context) error {
	ctx := c.Request().Context()
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid ID")
	}

	event, err := h.Queries.GetEvent(ctx, id)
	if err != nil || !event.IsPublished {
		return echo.NewHTTPError(http.StatusNotFound, "Event not found")
	}

	// Pre-fill if user is logged in
	var name, email string
	_, loggedIn, _ := appcontext.GetUser(ctx)
	if loggedIn {
		userID := appcontext.GetUserID(ctx)
		if userID > 0 {
			user, err := h.Queries.GetUserByID(ctx, userID)
			if err == nil {
				name = user.Name
				email = user.Email
			}
		}
	}

	return renderPage(c, event.Title+" への参加登録", components.RegistrationForm(event, name, email, ""))
}

func (h *PublicEventHandler) SubmitRegistration(c echo.Context) error {
	ctx := c.Request().Context()
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid ID")
	}

	event, err := h.Queries.GetEvent(ctx, id)
	if err != nil || !event.IsPublished {
		return echo.NewHTTPError(http.StatusNotFound, "Event not found")
	}

	name := c.FormValue("name")
	email := c.FormValue("email")

	if name == "" || email == "" {
		return renderPage(c, event.Title+" への参加登録", components.RegistrationForm(event, name, email, "お名前とメールアドレスは必須です。"))
	}

	// Check capacity
	if event.Capacity > 0 {
		regCount, err := h.Queries.CountRegistrationsByEvent(ctx, id)
		if err != nil {
			logger.Error("failed to count registrations", "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to process registration")
		}
		if regCount >= event.Capacity {
			return renderPage(c, event.Title+" への参加登録", components.RegistrationForm(event, name, email, "このイベントは満席です。"))
		}
	}

	// Find or create user
	user, err := h.Queries.GetUserByEmail(ctx, email)
	if err != nil {
		if err == sql.ErrNoRows {
			user, err = h.Queries.CreateUser(ctx, database.CreateUserParams{
				Email:    email,
				Name:     name,
				Role:     "viewer",
				IsActive: true,
			})
			if err != nil {
				logger.Error("failed to create user", "error", err)
				return echo.NewHTTPError(http.StatusInternalServerError, "Failed to process registration")
			}
		} else {
			logger.Error("failed to lookup user", "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to process registration")
		}
	}

	// Check duplicate registration
	_, err = h.Queries.GetRegistrationByEventAndUser(ctx, database.GetRegistrationByEventAndUserParams{
		EventID: id,
		UserID:  user.ID,
	})
	if err == nil {
		return renderPage(c, event.Title+" への参加登録", components.RegistrationForm(event, name, email, "このイベントには既に登録済みです。"))
	}

	// Create registration
	_, err = h.Queries.CreateRegistration(ctx, database.CreateRegistrationParams{
		EventID:      id,
		UserID:       user.ID,
		Name:         name,
		CustomFields: "[]",
	})
	if err != nil {
		logger.Error("failed to create registration", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to process registration")
	}

	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/events/%d/registered", id))
}

func (h *PublicEventHandler) RegistrationConfirm(c echo.Context) error {
	ctx := c.Request().Context()
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid ID")
	}

	event, err := h.Queries.GetEvent(ctx, id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Event not found")
	}

	return renderPage(c, "登録完了", components.RegistrationConfirm(event))
}
