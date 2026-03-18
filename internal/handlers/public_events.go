package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/appcontext"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/database"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/logger"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/models"
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

	groups := groupEventsBySection(events)
	return renderPage(c, "イベント一覧", components.EventList(groups))
}

func groupEventsBySection(events []database.Event) []models.EventGroup {
	orderMap := map[string]int{}
	groupMap := map[string][]database.Event{}

	for _, e := range events {
		section := models.GetCustomFieldValue(e.CustomFields, "区分")
		if section == "" {
			section = "その他"
		}
		if _, exists := orderMap[section]; !exists {
			orderMap[section] = len(orderMap)
		}
		groupMap[section] = append(groupMap[section], e)
	}

	groups := make([]models.EventGroup, len(orderMap))
	for section, idx := range orderMap {
		groups[idx] = models.EventGroup{Section: section, Events: groupMap[section]}
	}
	return groups
}

func (h *PublicEventHandler) TimetableView(c echo.Context) error {
	ctx := c.Request().Context()
	events, err := h.Queries.ListPublishedEvents(ctx)
	if err != nil {
		logger.Error("failed to list published events", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to load events")
	}

	timetables := buildTimetables(events)
	return renderPage(c, "時間割", components.EventTimetable(timetables))
}

func buildTimetables(events []database.Event) []models.Timetable {
	// Filter events that have timetable data
	var ttEvents []database.Event
	for _, e := range events {
		if models.GetCustomFieldValue(e.CustomFields, "時間帯") != "" {
			ttEvents = append(ttEvents, e)
		}
	}
	if len(ttEvents) == 0 {
		return nil
	}

	// Collect unique rooms preserving insertion order
	roomOrder := []string{}
	roomSeen := map[string]bool{}

	// Find global min/max minutes and collect cells
	globalMin := 24 * 60
	globalMax := 0

	type cellInfo struct {
		event      database.Event
		room       string
		startMin   int
		endMin     int
		timeRange  string
		subject    string
	}
	var cells []cellInfo

	for _, e := range ttEvents {
		tr := models.GetCustomFieldValue(e.CustomFields, "時間帯")
		r := e.Venue
		if r != "" && !roomSeen[r] {
			roomSeen[r] = true
			roomOrder = append(roomOrder, r)
		}
		sMin, eMin := parseTimeRange(tr)
		if sMin < 0 {
			continue
		}
		if sMin < globalMin {
			globalMin = sMin
		}
		if eMin > globalMax {
			globalMax = eMin
		}
		cells = append(cells, cellInfo{
			event:     e,
			room:      r,
			startMin:  sMin,
			endMin:    eMin,
			timeRange: tr,
			subject:   models.GetCustomFieldValue(e.CustomFields, "科目"),
		})
	}

	// Collect unique start and end times for time labels
	timeOrder := []string{}
	timeSeen := map[string]bool{}
	for _, c := range cells {
		startLabel := fmt.Sprintf("%02d:%02d", c.startMin/60, c.startMin%60)
		endLabel := fmt.Sprintf("%02d:%02d", c.endMin/60, c.endMin%60)
		if !timeSeen[startLabel] {
			timeSeen[startLabel] = true
			timeOrder = append(timeOrder, startLabel)
		}
		if !timeSeen[endLabel] {
			timeSeen[endLabel] = true
			timeOrder = append(timeOrder, endLabel)
		}
	}
	sort.Strings(timeOrder)

	// Build grid: key is start time label
	const pxPerMin = 2.0
	grid := map[string]map[string]*models.TimetableCell{}
	for _, c := range cells {
		label := fmt.Sprintf("%02d:%02d", c.startMin/60, c.startMin%60)
		if grid[label] == nil {
			grid[label] = map[string]*models.TimetableCell{}
		}
		// CSS grid rows: 1-based, relative to globalMin
		gridStart := c.startMin - globalMin + 2 // +2: row 1 is header
		gridEnd := c.endMin - globalMin + 2
		grid[label][c.room] = &models.TimetableCell{
			Event:        c.event,
			Subject:      c.subject,
			TimeRange:    c.timeRange,
			GridRowStart: gridStart,
			GridRowEnd:   gridEnd,
		}
	}

	totalRows := globalMax - globalMin

	return []models.Timetable{{
		Section:     "全プログラム",
		TimeSlots:   timeOrder,
		Rooms:       roomOrder,
		Grid:        grid,
		StartMinute: globalMin,
		EndMinute:   globalMax,
		TotalRows:   totalRows,
		PxPerMinute: pxPerMin,
	}}
}

// parseTimeRange parses "09:20-10:00" into start/end minutes of day. Returns -1,-1 on error.
func parseTimeRange(tr string) (int, int) {
	idx := strings.Index(tr, "-")
	if idx <= 0 {
		return -1, -1
	}
	sMin := parseMinutes(tr[:idx])
	eMin := parseMinutes(tr[idx+1:])
	if sMin < 0 || eMin < 0 {
		return -1, -1
	}
	return sMin, eMin
}

func parseMinutes(hhmm string) int {
	parts := strings.Split(strings.TrimSpace(hhmm), ":")
	if len(parts) != 2 {
		return -1
	}
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return -1
	}
	return h*60 + m
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
