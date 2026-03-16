package middleware

import (
	"database/sql"
	"net/http"

	"github.com/naozine/project_crud_with_auth_tmpl/internal/appcontext"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/database"

	"github.com/labstack/echo/v4"
	"github.com/naozine/nz-magic-link/magiclink"
)

func UserContextMiddleware(ml *magiclink.MagicLink, dbConn *sql.DB) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userEmail, isLoggedIn := ml.GetUserID(c)

			var hasPasskey bool
			var role string
			var userID int64

			if isLoggedIn {
				// Check passkey
				creds, err := ml.DB.GetPasskeyCredentialsByUserID(userEmail)
				if err == nil && len(creds) > 0 {
					hasPasskey = true
				}

				// Get user info from app DB
				q := database.New(dbConn)
				user, err := q.GetUserByEmail(c.Request().Context(), userEmail)
				if err == nil {
					role = user.Role
					userID = user.ID
				}
			}

			// Set user info to request context
			ctx := c.Request().Context()
			ctx = appcontext.WithUser(ctx, userEmail, isLoggedIn, hasPasskey, role, userID)
			c.SetRequest(c.Request().WithContext(ctx))

			return next(c)
		}
	}
}

// RequireRole は指定されたロールのいずれかを持つユーザーのみアクセスを許可するミドルウェア。
// 認証済みであることが前提のため、RequireAuth の後に適用すること。
func RequireRole(roles ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			role := appcontext.GetUserRole(c.Request().Context())
			for _, r := range roles {
				if role == r {
					return next(c)
				}
			}
			return echo.NewHTTPError(http.StatusForbidden, "アクセス権限がありません")
		}
	}
}

// RequireAuth は認証を必須とするミドルウェア。
// 未認証の場合はログインページへリダイレクトする。
func RequireAuth(ml *magiclink.MagicLink, loginURL string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			_, isLoggedIn := ml.GetUserID(c)

			if !isLoggedIn {
				return c.Redirect(http.StatusSeeOther, loginURL)
			}

			return next(c)
		}
	}
}
