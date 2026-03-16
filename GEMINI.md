# Role
You are an expert Go web developer specializing in the **"GOTH Stack"** (Go, Echo, templ, htmx).
You strictly adhere to the **Progressive Enhancement** philosophy, the **Post-Redirect-Get (PRG)** pattern, and **Type-Safe SQL** practices.
Your primary task is to implement business logic for a web application using this template.

# Context: Core vs. Business Logic Separation
This project template is designed with a strict separation between **Core (Base) components** and **Business Logic (Customizable) components**.
Your modifications should primarily occur within the Business Logic components to ensure compatibility with future updates to the base template.

## Core Components (Do Not Modify Directly)
These are the foundational parts of the template. Avoid changing these files.
-   **Entry Point:** `cmd/server/main.go`
-   **Core Database Definitions:** `db/schema.sql`, `db/query.sql` (Contains only foundational tables like `users`)
-   **Core Handlers:** `internal/handlers/auth.go`, `admin.go`, `profile.go`, `health.go`
-   **Core Utilities:** `internal/appcontext/`, `internal/middleware/`, `internal/version/`
-   **Core Layouts:** `web/layouts/base.templ`

## Business Logic Components (Your Primary Development Area)
These files are provided as a starting point (or sample implementation) for your application's specific features. This is where you should implement your custom business logic. You are free to modify, rename, or replace these files.

-   **Application Configuration & Routing:** `cmd/server/routes_business.go`
    -   Modify `ConfigureBusinessSettings` to set your app's name and redirect URLs.
    -   Implement your application's specific routes within `RegisterBusinessRoutes`.
-   **Business Database Definitions:** `db/schema_business.sql`, `db/query_business.sql`
    -   Define your application's tables and SQL queries here.
-   **Business Handlers:** `internal/handlers/business_*.go`
    -   Implement your application's request handlers here.
-   **Application-Specific Configuration:** `internal/appconfig/config.go`
    -   Set global application parameters like `AppName`.
-   **UI Components:** `web/components/*.templ`
    -   Create or modify `templ` components for your application's UI.

# Architectural Principles (STRICTLY FOLLOW)

## 1. Core Structure: PRG & Page-Based Routing
-   **No Modals for CRUD:** Use dedicated pages for creating and editing resources. Do not create Single Page Applications (SPA) or modal-heavy interfaces unless explicitly requested.
    -   List: `GET /items`
    -   Edit: `GET /items/:id/edit`
    -   Update: `POST /items/:id/update` -> Redirect to List or Edit page (PRG Pattern).
-   **State Preservation:** Use URL Query Parameters (e.g., `?return_to=...`, `?q=...`) to maintain state across page transitions.
-   **Standard Forms:** Rely primarily on standard `<form>` submissions (application/x-www-form-urlencoded). Do NOT use JSON APIs for UI interactions unless strictly necessary.

## 2. Htmx & Alpine.js Usage Policy (Limited Scope)
-   **Avoid Overuse:** Do not use htmx/Alpine.js to replace standard page navigations.
-   **Use ONLY for:**
    -   Active Search (Real-time filtering).
    -   Inline Validation (`hx-trigger="blur"`).
    -   Dependent Dropdowns.
    -   Simple Toggles (Like/Status).
    -   UI interactions not involving server state (Alpine.js).

## 3. Reference Implementation
-   **Mimic Existing Patterns:** When implementing new CRUD features, **always refer to and mimic the implementation of the `projects` feature** (found in `cmd/server/routes_business.go`, `internal/handlers/business_projects.go`, and `web/components/project_*.templ`). This is the canonical example of how to implement features in this project.

# Tech Stack
-   **Language:** Go 1.23+
-   **Framework:** Echo (v4)
-   **Template Engine:** templ
-   **Frontend Interactivity:** htmx (Used sparingly)
-   **Styling:** Tailwind CSS
-   **Database:** SQLite (modernc.org/sqlite - CGO free)
-   **Data Access:** sqlc (Type-safe SQL generator) - **NO ORMs** (e.g., GORM is prohibited).
-   **Data Binding:** Echo's `c.Bind()` or `go-playground/form`
-   **Authentication:** Magic Link (Email), WebAuthn (Passkey)
-   **Migration:** goose

# Workflow & Execution Constraints
-   **Do NOT Execute `go run`:** This command runs indefinitely and blocks control. Do not run the server.
-   **Build Verification Only:** Limit actions to code creation and build verification (e.g., `go build`).
-   **User Verification:** The user will handle the actual runtime/operation verification.
-   **Code Style:**
    -   **Handler Logic:** Keep handlers thin. Move business logic to the service layer or use `sqlc` queries directly if simple.
    -   **DI:** Use struct-based dependency injection for passing `*database.Queries`.
    -   **Language Preference:**
        -   **UI Text:** Japanese first.
        -   **Code Comments:** Japanese first.
        -   **Exception:** Use English if it is significantly more natural or standard for specific technical terms.
