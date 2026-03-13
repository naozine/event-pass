package integration

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/database"
)

// routeTestCase は権限マトリクスの1行を表す
type routeTestCase struct {
	Name         string
	Method       string
	Path         string // %d は seed.Project.ID で置換される
	Body         string // POST のフォームボディ
	AdminStatus  int
	EditorStatus int
	ViewerStatus int
	UnauthStatus int
}

func TestPermissionMatrix_ProjectRoutes(t *testing.T) {
	conn := SetupTestDB(t)
	e := SetupTestServer(t, conn)
	seed := SeedTestData(t, conn)

	projectID := seed.Project.ID

	routes := []routeTestCase{
		{
			Name: "GET /projects（一覧）",
			Method: http.MethodGet, Path: "/projects",
			AdminStatus: http.StatusOK, EditorStatus: http.StatusOK,
			ViewerStatus: http.StatusOK, UnauthStatus: http.StatusSeeOther,
		},
		{
			Name: "GET /projects/:id（詳細）",
			Method: http.MethodGet, Path: fmt.Sprintf("/projects/%d", projectID),
			AdminStatus: http.StatusOK, EditorStatus: http.StatusOK,
			ViewerStatus: http.StatusOK, UnauthStatus: http.StatusSeeOther,
		},
		{
			Name: "GET /projects/new（新規作成フォーム）",
			Method: http.MethodGet, Path: "/projects/new",
			AdminStatus: http.StatusOK, EditorStatus: http.StatusOK,
			ViewerStatus: http.StatusForbidden, UnauthStatus: http.StatusSeeOther,
		},
		{
			Name: "POST /projects/new（作成実行）",
			Method: http.MethodPost, Path: "/projects/new",
			Body: "name=新規プロジェクト",
			AdminStatus: http.StatusSeeOther, EditorStatus: http.StatusSeeOther,
			ViewerStatus: http.StatusForbidden, UnauthStatus: http.StatusSeeOther,
		},
		{
			Name: "GET /projects/:id/edit（編集フォーム）",
			Method: http.MethodGet, Path: fmt.Sprintf("/projects/%d/edit", projectID),
			AdminStatus: http.StatusOK, EditorStatus: http.StatusOK,
			ViewerStatus: http.StatusForbidden, UnauthStatus: http.StatusSeeOther,
		},
		{
			Name: "POST /projects/:id/update（更新実行）",
			Method: http.MethodPost, Path: fmt.Sprintf("/projects/%d/update", projectID),
			Body: "name=更新済み",
			AdminStatus: http.StatusSeeOther, EditorStatus: http.StatusSeeOther,
			ViewerStatus: http.StatusForbidden, UnauthStatus: http.StatusSeeOther,
		},
		{
			Name: "POST /projects/:id/delete（削除実行）",
			Method: http.MethodPost, Path: fmt.Sprintf("/projects/%d/delete", projectID),
			AdminStatus: http.StatusSeeOther, EditorStatus: http.StatusSeeOther,
			ViewerStatus: http.StatusForbidden, UnauthStatus: http.StatusSeeOther,
		},
	}

	runPermissionMatrix(t, e, seed, routes)
}

func TestPermissionMatrix_AdminRoutes(t *testing.T) {
	conn := SetupTestDB(t)
	e := SetupTestServer(t, conn)
	seed := SeedTestData(t, conn)

	// 編集・更新・削除対象として viewer ユーザーを使う
	targetID := seed.ViewerUser.ID

	routes := []routeTestCase{
		{
			Name: "GET /admin/users（ユーザー一覧）",
			Method: http.MethodGet, Path: "/admin/users",
			AdminStatus: http.StatusOK, EditorStatus: http.StatusForbidden,
			ViewerStatus: http.StatusForbidden, UnauthStatus: http.StatusSeeOther,
		},
		{
			Name: "GET /admin/users/new（ユーザー作成フォーム）",
			Method: http.MethodGet, Path: "/admin/users/new",
			AdminStatus: http.StatusOK, EditorStatus: http.StatusForbidden,
			ViewerStatus: http.StatusForbidden, UnauthStatus: http.StatusSeeOther,
		},
		{
			Name: "POST /admin/users（ユーザー作成実行）",
			Method: http.MethodPost, Path: "/admin/users",
			Body: "name=NewUser&email=new@test.com&role=viewer",
			AdminStatus: http.StatusOK, EditorStatus: http.StatusForbidden,
			ViewerStatus: http.StatusForbidden, UnauthStatus: http.StatusSeeOther,
		},
		{
			Name: "GET /admin/users/:id/edit（ユーザー編集フォーム）",
			Method: http.MethodGet, Path: fmt.Sprintf("/admin/users/%d/edit", targetID),
			AdminStatus: http.StatusOK, EditorStatus: http.StatusForbidden,
			ViewerStatus: http.StatusForbidden, UnauthStatus: http.StatusSeeOther,
		},
		{
			Name: "POST /admin/users/:id（ユーザー更新実行）",
			Method: http.MethodPost, Path: fmt.Sprintf("/admin/users/%d", targetID),
			Body: "name=UpdatedViewer&role=viewer&status=active",
			AdminStatus: http.StatusOK, EditorStatus: http.StatusForbidden,
			ViewerStatus: http.StatusForbidden, UnauthStatus: http.StatusSeeOther,
		},
		{
			Name: "DELETE /admin/users/:id（ユーザー削除実行）",
			Method: http.MethodDelete, Path: fmt.Sprintf("/admin/users/%d", seed.DeletableUser.ID),
			AdminStatus: http.StatusOK, EditorStatus: http.StatusForbidden,
			ViewerStatus: http.StatusForbidden, UnauthStatus: http.StatusSeeOther,
		},
	}

	runPermissionMatrix(t, e, seed, routes)
}

// runPermissionMatrix は各ルートに対して全ロール + 未認証のテストを実行する
func runPermissionMatrix(t *testing.T, e interface{ ServeHTTP(http.ResponseWriter, *http.Request) }, seed SeedData, routes []routeTestCase) {
	t.Helper()

	type roleCase struct {
		name string
		user *database.User
	}

	for _, rt := range routes {
		t.Run(rt.Name, func(t *testing.T) {
			cases := []struct {
				role       roleCase
				wantStatus int
			}{
				{roleCase{"admin", &seed.AdminUser}, rt.AdminStatus},
				{roleCase{"editor", &seed.EditorUser}, rt.EditorStatus},
				{roleCase{"viewer", &seed.ViewerUser}, rt.ViewerStatus},
				{roleCase{"未認証", nil}, rt.UnauthStatus},
			}

			for _, tc := range cases {
				t.Run(tc.role.name, func(t *testing.T) {
					rec := DoRequest(e.(*echo.Echo), rt.Method, rt.Path, tc.role.user, rt.Body)
					if rec.Code != tc.wantStatus {
						t.Errorf("ステータスコード = %d, want %d", rec.Code, tc.wantStatus)
					}
				})
			}
		})
	}
}
