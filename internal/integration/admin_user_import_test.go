package integration

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/database"
	"github.com/xuri/excelize/v2"
)

// ---------------------------------------------------------------------------
// ヘルパー: Excel ファイル作成 → multipart リクエスト
// ---------------------------------------------------------------------------

// excelRow は Excel の1行分のデータ。
type excelRow struct {
	Name  string
	Email string
	Role  string
}

// createExcelBytes は Excel ファイルをバイト列として生成する。
func createExcelBytes(t *testing.T, rows []excelRow) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()

	sheet := "Sheet1"
	f.SetCellValue(sheet, "A1", "名前")
	f.SetCellValue(sheet, "B1", "メールアドレス")
	f.SetCellValue(sheet, "C1", "ロール")

	for i, row := range rows {
		r := i + 2
		f.SetCellValue(sheet, fmt.Sprintf("A%d", r), row.Name)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", r), row.Email)
		f.SetCellValue(sheet, fmt.Sprintf("C%d", r), row.Role)
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatalf("Excel 書き出しに失敗: %v", err)
	}
	return buf.Bytes()
}

// doFileUpload は multipart/form-data でファイルをアップロードする。
func doFileUpload(e *echo.Echo, path string, user *database.User, fieldName, fileName string, fileData []byte) *httptest.ResponseRecorder {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, _ := writer.CreateFormFile(fieldName, fileName)
	io.Copy(part, bytes.NewReader(fileData))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, path, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if user != nil {
		req.Header.Set("X-Test-User-ID", fmt.Sprintf("%d", user.ID))
	}

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

// ---------------------------------------------------------------------------
// テスト: インポートページ表示
// ---------------------------------------------------------------------------

func TestUserImport_PageAccess(t *testing.T) {
	conn := SetupTestDB(t)
	e := SetupTestServer(t, conn)
	seed := SeedTestData(t, conn)

	// admin はアクセスできる
	rec := DoRequest(e, http.MethodGet, "/admin/users/import", &seed.AdminUser)
	if rec.Code != http.StatusOK {
		t.Errorf("admin: got %d, want %d", rec.Code, http.StatusOK)
	}

	// viewer はアクセスできない
	rec = DoRequest(e, http.MethodGet, "/admin/users/import", &seed.ViewerUser)
	if rec.Code != http.StatusForbidden {
		t.Errorf("viewer: got %d, want %d", rec.Code, http.StatusForbidden)
	}

	// 未認証はログインにリダイレクト
	rec = DoRequest(e, http.MethodGet, "/admin/users/import", nil)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("unauth: got %d, want %d", rec.Code, http.StatusSeeOther)
	}
}

// ---------------------------------------------------------------------------
// テスト: テンプレートダウンロード
// ---------------------------------------------------------------------------

func TestUserImport_TemplateDownload(t *testing.T) {
	conn := SetupTestDB(t)
	e := SetupTestServer(t, conn)
	seed := SeedTestData(t, conn)

	rec := DoRequest(e, http.MethodGet, "/admin/users/import/template", &seed.AdminUser)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want %d", rec.Code, http.StatusOK)
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		t.Errorf("Content-Type = %q, want xlsx", ct)
	}

	cd := rec.Header().Get("Content-Disposition")
	if cd == "" {
		t.Error("Content-Disposition header is missing")
	}
}

// ---------------------------------------------------------------------------
// テスト: 正常インポート
// ---------------------------------------------------------------------------

func TestUserImport_Success(t *testing.T) {
	conn := SetupTestDB(t)
	e := SetupTestServer(t, conn)
	seed := SeedTestData(t, conn)

	rows := []excelRow{
		{Name: "ユーザーA", Email: "usera@test.com", Role: "viewer"},
		{Name: "ユーザーB", Email: "userb@test.com", Role: "editor"},
		{Name: "ユーザーC", Email: "userc@test.com", Role: "admin"},
	}
	data := createExcelBytes(t, rows)

	rec := doFileUpload(e, "/admin/users/import", &seed.AdminUser, "file", "users.xlsx", data)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	// DB に登録されたか確認
	q := queryFromConn(conn)
	for _, row := range rows {
		user, err := q.GetUserByEmail(t.Context(), row.Email)
		if err != nil {
			t.Errorf("ユーザー %s が見つからない: %v", row.Email, err)
			continue
		}
		if user.Name != row.Name {
			t.Errorf("Name = %q, want %q", user.Name, row.Name)
		}
		if user.Role != row.Role {
			t.Errorf("Role = %q, want %q", user.Role, row.Role)
		}
		if !user.IsActive {
			t.Errorf("IsActive = false, want true")
		}
	}

	// レスポンスに成功メッセージが含まれるか
	body := rec.Body.String()
	if !bytes.Contains([]byte(body), []byte("3件")) {
		t.Errorf("レスポンスに成功件数が含まれていない")
	}
}

// ---------------------------------------------------------------------------
// テスト: バリデーションエラー
// ---------------------------------------------------------------------------

func TestUserImport_ValidationErrors(t *testing.T) {
	conn := SetupTestDB(t)
	e := SetupTestServer(t, conn)
	seed := SeedTestData(t, conn)

	rows := []excelRow{
		{Name: "", Email: "noname@test.com", Role: "viewer"},        // 名前なし
		{Name: "NoEmail", Email: "", Role: "viewer"},                 // メールなし
		{Name: "BadEmail", Email: "not-an-email", Role: "viewer"},    // 不正メール
		{Name: "BadRole", Email: "badrole@test.com", Role: "owner"},  // 不正ロール
		{Name: "Valid", Email: "valid@test.com", Role: "viewer"},     // 正常行
	}
	data := createExcelBytes(t, rows)

	rec := doFileUpload(e, "/admin/users/import", &seed.AdminUser, "file", "users.xlsx", data)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want %d", rec.Code, http.StatusOK)
	}

	// 正常行のみ登録される
	q := queryFromConn(conn)
	_, err := q.GetUserByEmail(t.Context(), "valid@test.com")
	if err != nil {
		t.Errorf("正常行のユーザーが登録されていない: %v", err)
	}

	// エラー行は登録されない
	_, err = q.GetUserByEmail(t.Context(), "noname@test.com")
	if err == nil {
		t.Error("名前なしユーザーが登録されてしまった")
	}
}

// ---------------------------------------------------------------------------
// テスト: 重複チェック
// ---------------------------------------------------------------------------

func TestUserImport_DuplicateErrors(t *testing.T) {
	conn := SetupTestDB(t)
	e := SetupTestServer(t, conn)
	seed := SeedTestData(t, conn)

	rows := []excelRow{
		// DB に既存のメールアドレス
		{Name: "Duplicate Admin", Email: "admin@test.com", Role: "viewer"},
		// ファイル内重複
		{Name: "UserX", Email: "userx@test.com", Role: "viewer"},
		{Name: "UserX Copy", Email: "userx@test.com", Role: "editor"},
		// 正常行
		{Name: "UserY", Email: "usery@test.com", Role: "viewer"},
	}
	data := createExcelBytes(t, rows)

	rec := doFileUpload(e, "/admin/users/import", &seed.AdminUser, "file", "users.xlsx", data)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want %d", rec.Code, http.StatusOK)
	}

	q := queryFromConn(conn)

	// DB 重複: 登録されない
	body := rec.Body.String()
	if !bytes.Contains([]byte(body), []byte("既に登録されています")) {
		t.Error("DB 重複エラーメッセージが表示されていない")
	}

	// ファイル内重複: 2行目は登録、3行目はエラー
	_, err := q.GetUserByEmail(t.Context(), "userx@test.com")
	if err != nil {
		t.Errorf("ファイル内最初の userx が登録されていない: %v", err)
	}

	// 正常行
	_, err = q.GetUserByEmail(t.Context(), "usery@test.com")
	if err != nil {
		t.Errorf("正常行の usery が登録されていない: %v", err)
	}
}

// ---------------------------------------------------------------------------
// テスト: 空行スキップ
// ---------------------------------------------------------------------------

func TestUserImport_SkipEmptyRows(t *testing.T) {
	conn := SetupTestDB(t)
	e := SetupTestServer(t, conn)
	seed := SeedTestData(t, conn)

	rows := []excelRow{
		{Name: "UserA", Email: "usera@test.com", Role: "viewer"},
		{Name: "", Email: "", Role: ""},  // 空行
		{Name: "UserB", Email: "userb@test.com", Role: "editor"},
	}
	data := createExcelBytes(t, rows)

	rec := doFileUpload(e, "/admin/users/import", &seed.AdminUser, "file", "users.xlsx", data)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want %d", rec.Code, http.StatusOK)
	}

	// 2件のみ登録
	body := rec.Body.String()
	if !bytes.Contains([]byte(body), []byte("2件")) {
		t.Errorf("成功件数が2件でない: %s", body)
	}
}

// ---------------------------------------------------------------------------
// テスト: 権限チェック（POST）
// ---------------------------------------------------------------------------

func TestUserImport_Forbidden(t *testing.T) {
	conn := SetupTestDB(t)
	e := SetupTestServer(t, conn)
	seed := SeedTestData(t, conn)

	rows := []excelRow{
		{Name: "User", Email: "user@test.com", Role: "viewer"},
	}
	data := createExcelBytes(t, rows)

	// editor は実行できない
	rec := doFileUpload(e, "/admin/users/import", &seed.EditorUser, "file", "users.xlsx", data)
	if rec.Code != http.StatusForbidden {
		t.Errorf("editor: got %d, want %d", rec.Code, http.StatusForbidden)
	}

	// viewer は実行できない
	rec = doFileUpload(e, "/admin/users/import", &seed.ViewerUser, "file", "users.xlsx", data)
	if rec.Code != http.StatusForbidden {
		t.Errorf("viewer: got %d, want %d", rec.Code, http.StatusForbidden)
	}
}
