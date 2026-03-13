package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/appcontext"
)

func TestRequireRole(t *testing.T) {
	tests := []struct {
		name           string
		userRole       string
		allowedRoles   []string
		wantStatusCode int
	}{
		{
			name:           "admin は admin 専用ルートにアクセスできる",
			userRole:       "admin",
			allowedRoles:   []string{"admin"},
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "editor は admin 専用ルートにアクセスできない",
			userRole:       "editor",
			allowedRoles:   []string{"admin"},
			wantStatusCode: http.StatusForbidden,
		},
		{
			name:           "viewer は admin 専用ルートにアクセスできない",
			userRole:       "viewer",
			allowedRoles:   []string{"admin"},
			wantStatusCode: http.StatusForbidden,
		},
		{
			name:           "admin は書き込みルートにアクセスできる",
			userRole:       "admin",
			allowedRoles:   []string{"admin", "editor"},
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "editor は書き込みルートにアクセスできる",
			userRole:       "editor",
			allowedRoles:   []string{"admin", "editor"},
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "viewer は書き込みルートにアクセスできない",
			userRole:       "viewer",
			allowedRoles:   []string{"admin", "editor"},
			wantStatusCode: http.StatusForbidden,
		},
		{
			name:           "ロール未設定はアクセスできない",
			userRole:       "",
			allowedRoles:   []string{"admin"},
			wantStatusCode: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			// コンテキストにユーザーロールを設定
			ctx := appcontext.WithUser(req.Context(), "test@example.com", true, false, tt.userRole, 1)
			c.SetRequest(req.WithContext(ctx))

			// ミドルウェアを通してハンドラを実行
			handler := RequireRole(tt.allowedRoles...)(func(c echo.Context) error {
				return c.String(http.StatusOK, "ok")
			})

			err := handler(c)

			if tt.wantStatusCode == http.StatusOK {
				if err != nil {
					t.Errorf("アクセスが許可されるべきだが、エラーが返された: %v", err)
				}
				if rec.Code != http.StatusOK {
					t.Errorf("ステータスコード = %d, want %d", rec.Code, http.StatusOK)
				}
			} else {
				if err == nil {
					t.Error("アクセスが拒否されるべきだが、エラーが返されなかった")
					return
				}
				httpErr, ok := err.(*echo.HTTPError)
				if !ok {
					t.Errorf("echo.HTTPError が期待されるが、%T が返された", err)
					return
				}
				if httpErr.Code != tt.wantStatusCode {
					t.Errorf("ステータスコード = %d, want %d", httpErr.Code, tt.wantStatusCode)
				}
			}
		})
	}
}
