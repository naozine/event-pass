package routes

import (
	"github.com/labstack/echo/v4"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/database"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/handlers"
	appMiddleware "github.com/naozine/project_crud_with_auth_tmpl/internal/middleware"
)

// RegisterBusinessRoutes registers business logic routes.
// authMW is the auth middleware (RequireAuth in prod, stub in tests).
func RegisterBusinessRoutes(e *echo.Echo, queries *database.Queries, authMW echo.MiddlewareFunc) {
	requireAdmin := appMiddleware.RequireRole("admin")

	// --- Public: Events (no auth) ---
	publicHandler := handlers.NewPublicEventHandler(queries)
	e.GET("/events", publicHandler.ListUpcomingEvents)
	e.GET("/events/timetable", publicHandler.TimetableView)
	e.GET("/events/:id", publicHandler.ShowEventDetail)
	e.GET("/events/:id/register", publicHandler.ShowRegistrationForm)
	e.POST("/events/:id/register", publicHandler.SubmitRegistration)
	e.GET("/events/:id/registered", publicHandler.RegistrationConfirm)

	// --- Attendee: My Registrations (auth required) ---
	attendeeHandler := handlers.NewAttendeeHandler(queries)
	myGroup := e.Group("/my", authMW)
	myGroup.GET("", attendeeHandler.MyRegistrations)
	myGroup.GET("/registrations/:id/pass", attendeeHandler.ShowPass)
	myGroup.POST("/registrations/:id/cancel", attendeeHandler.CancelRegistration)

	// --- Admin: Events ---
	eventHandler := handlers.NewEventHandler(queries)
	adminEvents := e.Group("/admin/events", authMW, requireAdmin)
	adminEvents.GET("", eventHandler.ListEvents)
	adminEvents.GET("/new", eventHandler.NewEventPage)
	adminEvents.POST("/new", eventHandler.CreateEvent)
	adminEvents.GET("/:id", eventHandler.ShowEvent)
	adminEvents.GET("/:id/edit", eventHandler.EditEventPage)
	adminEvents.POST("/:id/update", eventHandler.UpdateEvent)
	adminEvents.POST("/:id/delete", eventHandler.DeleteEvent)
	adminEvents.POST("/:id/toggle-publish", eventHandler.TogglePublish)

	// --- Admin: Registrations & Dashboard ---
	regAdminHandler := handlers.NewRegistrationAdminHandler(queries)
	e.GET("/admin/dashboard", regAdminHandler.Dashboard, authMW, requireAdmin)
	adminEvents.GET("/:id/registrations", regAdminHandler.ListRegistrations)
	adminEvents.POST("/:id/registrations/:reg_id/status", regAdminHandler.UpdateStatus)
	adminEvents.GET("/:id/registrations/csv", regAdminHandler.ExportCSV)

	// --- Projects (template legacy, kept for reference) ---
	projectHandler := handlers.NewProjectHandler(queries)
	projectGroup := e.Group("/projects", authMW)
	projectGroup.GET("", projectHandler.ListProjects)
	projectGroup.GET("/:id", projectHandler.ShowProject)
	requireWrite := appMiddleware.RequireRole("admin", "editor")
	projectGroup.GET("/new", projectHandler.NewProjectPage, requireWrite)
	projectGroup.POST("/new", projectHandler.CreateProject, requireWrite)
	projectGroup.GET("/:id/edit", projectHandler.EditProjectPage, requireWrite)
	projectGroup.POST("/:id/update", projectHandler.UpdateProject, requireWrite)
	projectGroup.POST("/:id/delete", projectHandler.DeleteProject, requireWrite)

	// --- User import (admin only) ---
	importHandler := handlers.NewUserImportHandler(queries)
	e.GET("/admin/users/import", importHandler.ImportPage, authMW, requireAdmin)
	e.POST("/admin/users/import", importHandler.ExecuteImport, authMW, requireAdmin)
	e.GET("/admin/users/import/template", importHandler.TemplateDownload, authMW, requireAdmin)
}
