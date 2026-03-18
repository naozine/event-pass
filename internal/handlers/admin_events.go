package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/database"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/logger"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/models"
	"github.com/naozine/project_crud_with_auth_tmpl/web/components"
)

type EventHandler struct {
	Queries *database.Queries
}

func NewEventHandler(queries *database.Queries) *EventHandler {
	return &EventHandler{Queries: queries}
}

func (h *EventHandler) ListEvents(c echo.Context) error {
	ctx := c.Request().Context()
	events, err := h.Queries.ListAllEvents(ctx)
	if err != nil {
		logger.Error("failed to list events", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to load events")
	}
	return renderPage(c, "イベント管理", components.AdminEventList(events))
}

func (h *EventHandler) NewEventPage(c echo.Context) error {
	return renderPage(c, "新規イベント", components.AdminEventForm(nil))
}

func (h *EventHandler) CreateEvent(c echo.Context) error {
	ctx := c.Request().Context()

	params, err := parseEventForm(c)
	if err != nil {
		return err
	}

	_, err = h.Queries.CreateEvent(ctx, database.CreateEventParams{
		Code:         params.Code,
		Title:        params.Title,
		Description:  params.Description,
		Venue:        params.Venue,
		EventDate:    params.EventDate,
		Capacity:     params.Capacity,
		ColorBg:      params.ColorBg,
		ColorText:    params.ColorText,
		IsPublished:  params.IsPublished,
		CustomFields: params.CustomFields,
	})
	if err != nil {
		logger.Error("failed to create event", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to create event")
	}

	return c.Redirect(http.StatusSeeOther, "/admin/events")
}

func (h *EventHandler) ShowEvent(c echo.Context) error {
	ctx := c.Request().Context()
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid ID")
	}

	event, err := h.Queries.GetEvent(ctx, id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Event not found")
	}

	regCount, err := h.Queries.CountRegistrationsByEvent(ctx, id)
	if err != nil {
		logger.Error("failed to count registrations", "error", err)
		regCount = 0
	}

	return renderPage(c, event.Title, components.AdminEventDetail(event, regCount))
}

func (h *EventHandler) EditEventPage(c echo.Context) error {
	ctx := c.Request().Context()
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid ID")
	}

	event, err := h.Queries.GetEvent(ctx, id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Event not found")
	}

	return renderPage(c, "編集: "+event.Title, components.AdminEventForm(&event))
}

func (h *EventHandler) UpdateEvent(c echo.Context) error {
	ctx := c.Request().Context()
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid ID")
	}

	params, err := parseEventForm(c)
	if err != nil {
		return err
	}

	_, err = h.Queries.UpdateEvent(ctx, database.UpdateEventParams{
		ID:           id,
		Code:         params.Code,
		Title:        params.Title,
		Description:  params.Description,
		Venue:        params.Venue,
		EventDate:    params.EventDate,
		Capacity:     params.Capacity,
		ColorBg:      params.ColorBg,
		ColorText:    params.ColorText,
		IsPublished:  params.IsPublished,
		CustomFields: params.CustomFields,
	})
	if err != nil {
		logger.Error("failed to update event", "error", err, "id", id)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to update event")
	}

	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/events/%d", id))
}

func (h *EventHandler) DeleteEvent(c echo.Context) error {
	ctx := c.Request().Context()
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid ID")
	}

	err = h.Queries.DeleteEvent(ctx, id)
	if err != nil {
		logger.Error("failed to delete event", "error", err, "id", id)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to delete event")
	}

	return c.Redirect(http.StatusSeeOther, "/admin/events")
}

func (h *EventHandler) TogglePublish(c echo.Context) error {
	ctx := c.Request().Context()
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid ID")
	}

	event, err := h.Queries.GetEvent(ctx, id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Event not found")
	}

	_, err = h.Queries.UpdateEvent(ctx, database.UpdateEventParams{
		ID:           id,
		Code:         event.Code,
		Title:        event.Title,
		Description:  event.Description,
		Venue:        event.Venue,
		EventDate:    event.EventDate,
		Capacity:     event.Capacity,
		ColorBg:      event.ColorBg,
		ColorText:    event.ColorText,
		IsPublished:  !event.IsPublished,
		CustomFields: event.CustomFields,
	})
	if err != nil {
		logger.Error("failed to toggle publish", "error", err, "id", id)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to update event")
	}

	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/events/%d", id))
}

type eventFormParams struct {
	Code         string
	Title        string
	Description  string
	Venue        string
	EventDate    time.Time
	Capacity     int64
	ColorBg      string
	ColorText    string
	IsPublished  bool
	CustomFields string
}

func parseEventForm(c echo.Context) (eventFormParams, error) {
	title := c.FormValue("title")
	if title == "" {
		return eventFormParams{}, echo.NewHTTPError(http.StatusBadRequest, "Title is required")
	}

	eventDateStr := c.FormValue("event_date")
	eventDate, err := time.Parse("2006-01-02T15:04", eventDateStr)
	if err != nil {
		return eventFormParams{}, echo.NewHTTPError(http.StatusBadRequest, "Invalid date format")
	}

	capacity, _ := strconv.ParseInt(c.FormValue("capacity"), 10, 64)
	isPublished := c.FormValue("is_published") == "on"

	// Parse custom fields from repeated form fields
	keys := c.Request().Form["custom_field_key"]
	values := c.Request().Form["custom_field_value"]
	var fields []models.KeyValue
	for i := range keys {
		if keys[i] == "" {
			continue
		}
		v := ""
		if i < len(values) {
			v = values[i]
		}
		fields = append(fields, models.KeyValue{Key: keys[i], Value: v})
	}
	customFieldsJSON := "[]"
	if len(fields) > 0 {
		b, err := json.Marshal(fields)
		if err == nil {
			customFieldsJSON = string(b)
		}
	}

	colorBg := c.FormValue("color_bg")
	if colorBg == "" {
		colorBg = "#000000"
	}
	colorText := c.FormValue("color_text")
	if colorText == "" {
		colorText = "#ffffff"
	}

	return eventFormParams{
		Code:         c.FormValue("code"),
		Title:        title,
		Description:  c.FormValue("description"),
		Venue:        c.FormValue("venue"),
		EventDate:    eventDate,
		Capacity:     capacity,
		ColorBg:      colorBg,
		ColorText:    colorText,
		IsPublished:  isPublished,
		CustomFields: customFieldsJSON,
	}, nil
}
