package integration

import (
	"net/http"
	"testing"
)

func TestAdminCRUD_CreateUser(t *testing.T) {
	conn := SetupTestDB(t)
	e := SetupTestServer(t, conn)
	seed := SeedTestData(t, conn)

	// ユーザー作成
	rec := DoRequest(e, http.MethodPost, "/admin/users/new",
		&seed.AdminUser, "name=NewUser&email=newuser@test.com&role=editor")

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ステータスコード = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/admin/users" {
		t.Fatalf("リダイレクト先 = %q, want /admin/users", loc)
	}

	// DB に作成されたか確認
	q := queryFromConn(conn)
	user, err := q.GetUserByEmail(t.Context(), "newuser@test.com")
	if err != nil {
		t.Fatalf("作成されたユーザーが見つからない: %v", err)
	}
	if user.Name != "NewUser" {
		t.Errorf("Name = %q, want %q", user.Name, "NewUser")
	}
	if user.Role != "editor" {
		t.Errorf("Role = %q, want %q", user.Role, "editor")
	}
	if !user.IsActive {
		t.Error("IsActive = false, want true")
	}
}

func TestAdminCRUD_CreateUser_ValidationError(t *testing.T) {
	conn := SetupTestDB(t)
	e := SetupTestServer(t, conn)
	seed := SeedTestData(t, conn)

	// 必須フィールドが空
	rec := DoRequest(e, http.MethodPost, "/admin/users/new",
		&seed.AdminUser, "name=&email=&role=")

	if rec.Code != http.StatusBadRequest {
		t.Errorf("ステータスコード = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAdminCRUD_UpdateUser(t *testing.T) {
	conn := SetupTestDB(t)
	e := SetupTestServer(t, conn)
	seed := SeedTestData(t, conn)

	targetID := seed.ViewerUser.ID

	// ロールと名前を変更
	rec := DoRequest(e, http.MethodPost, sprintf("/admin/users/%d/update", targetID),
		&seed.AdminUser, "name=UpdatedName&role=editor&status=active")

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ステータスコード = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	// DB で更新されたか確認
	q := queryFromConn(conn)
	user, err := q.GetUserByID(t.Context(), targetID)
	if err != nil {
		t.Fatalf("ユーザーの取得に失敗: %v", err)
	}
	if user.Name != "UpdatedName" {
		t.Errorf("Name = %q, want %q", user.Name, "UpdatedName")
	}
	if user.Role != "editor" {
		t.Errorf("Role = %q, want %q", user.Role, "editor")
	}
}

func TestAdminCRUD_UpdateUser_Deactivate(t *testing.T) {
	conn := SetupTestDB(t)
	e := SetupTestServer(t, conn)
	seed := SeedTestData(t, conn)

	targetID := seed.ViewerUser.ID

	// ステータスを無効にする
	rec := DoRequest(e, http.MethodPost, sprintf("/admin/users/%d/update", targetID),
		&seed.AdminUser, "name=Viewer&role=viewer&status=inactive")

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ステータスコード = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	q := queryFromConn(conn)
	user, err := q.GetUserByID(t.Context(), targetID)
	if err != nil {
		t.Fatalf("ユーザーの取得に失敗: %v", err)
	}
	if user.IsActive {
		t.Error("IsActive = true, want false")
	}
}

func TestAdminCRUD_DeleteUser(t *testing.T) {
	conn := SetupTestDB(t)
	e := SetupTestServer(t, conn)
	seed := SeedTestData(t, conn)

	targetID := seed.DeletableUser.ID

	rec := DoRequest(e, http.MethodPost, sprintf("/admin/users/%d/delete", targetID),
		&seed.AdminUser, "")

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ステータスコード = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	// DB から削除されたか確認
	q := queryFromConn(conn)
	_, err := q.GetUserByID(t.Context(), targetID)
	if err == nil {
		t.Error("削除されたはずのユーザーが見つかった")
	}
}

func TestAdminCRUD_DeleteUser_PreventSelfDeletion(t *testing.T) {
	conn := SetupTestDB(t)
	e := SetupTestServer(t, conn)
	seed := SeedTestData(t, conn)

	// 自分自身を削除しようとする
	rec := DoRequest(e, http.MethodPost, sprintf("/admin/users/%d/delete", seed.AdminUser.ID),
		&seed.AdminUser, "")

	if rec.Code != http.StatusBadRequest {
		t.Errorf("ステータスコード = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	// 削除されていないことを確認
	q := queryFromConn(conn)
	_, err := q.GetUserByID(t.Context(), seed.AdminUser.ID)
	if err != nil {
		t.Error("自分自身が削除されてしまった")
	}
}
