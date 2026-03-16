package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/mail"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/database"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/logger"
	"github.com/naozine/project_crud_with_auth_tmpl/internal/models"
	"github.com/naozine/project_crud_with_auth_tmpl/web/components"
	"github.com/xuri/excelize/v2"
)

// UserImportHandler はユーザー一括インポートのハンドラ。
type UserImportHandler struct {
	Queries *database.Queries
}

func NewUserImportHandler(queries *database.Queries) *UserImportHandler {
	return &UserImportHandler{Queries: queries}
}

// ImportPage はインポートフォームを表示する。
func (h *UserImportHandler) ImportPage(c echo.Context) error {
	return renderPage(c, "ユーザー一括インポート", components.AdminUserImport(nil))
}

// TemplateDownload はインポート用の Excel テンプレートをダウンロードする。
func (h *UserImportHandler) TemplateDownload(c echo.Context) error {
	f := excelize.NewFile()
	defer f.Close()

	sheet := "Sheet1"
	f.SetCellValue(sheet, "A1", "名前")
	f.SetCellValue(sheet, "B1", "メールアドレス")
	f.SetCellValue(sheet, "C1", "ロール")

	// ヘッダー行のスタイル
	style, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#E5E7EB"}, Pattern: 1},
	})
	f.SetCellStyle(sheet, "A1", "C1", style)

	// サンプルデータ
	f.SetCellValue(sheet, "A2", "田中太郎")
	f.SetCellValue(sheet, "B2", "tanaka@example.com")
	f.SetCellValue(sheet, "C2", "viewer")

	// 列幅
	f.SetColWidth(sheet, "A", "A", 20)
	f.SetColWidth(sheet, "B", "B", 30)
	f.SetColWidth(sheet, "C", "C", 15)

	c.Response().Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Response().Header().Set("Content-Disposition", "attachment; filename=users_import_template.xlsx")
	return f.Write(c.Response().Writer)
}

// ExecuteImport は Excel ファイルからユーザーを一括インポートする。
func (h *UserImportHandler) ExecuteImport(c echo.Context) error {
	// ファイル取得
	file, err := c.FormFile("file")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "ファイルを選択してください")
	}

	// サイズ制限（5MB）
	if file.Size > 5*1024*1024 {
		return echo.NewHTTPError(http.StatusBadRequest, "ファイルサイズは5MB以下にしてください")
	}

	src, err := file.Open()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "ファイルの読み取りに失敗しました")
	}
	defer src.Close()

	f, err := excelize.OpenReader(src)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Excel ファイルの読み取りに失敗しました。.xlsx 形式のファイルを使用してください")
	}
	defer f.Close()

	// 最初のシートを読み取り
	sheet := f.GetSheetName(0)
	rows, err := f.GetRows(sheet)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "シートの読み取りに失敗しました")
	}

	if len(rows) < 2 {
		return echo.NewHTTPError(http.StatusBadRequest, "データ行がありません（1行目はヘッダー、2行目以降にデータを入力してください）")
	}

	// 行数上限（1000行）
	if len(rows) > 1001 {
		return echo.NewHTTPError(http.StatusBadRequest, "一度にインポートできるのは1000件までです")
	}

	// パース＆バリデーション
	ctx := c.Request().Context()
	result := &models.ImportResult{}
	seenEmails := make(map[string]int) // email -> 初出行番号

	for i, row := range rows[1:] { // ヘッダー行をスキップ
		rowNum := i + 2 // Excel の行番号（1-indexed、ヘッダーが1行目）

		// 空行スキップ
		if isEmptyRow(row) {
			continue
		}

		name := cellValue(row, 0)
		email := strings.ToLower(cellValue(row, 1))
		role := strings.ToLower(cellValue(row, 2))

		// バリデーション
		if name == "" {
			result.Errors = append(result.Errors, models.ImportRowError{Row: rowNum, Message: "名前は必須です"})
			continue
		}
		if email == "" {
			result.Errors = append(result.Errors, models.ImportRowError{Row: rowNum, Message: "メールアドレスは必須です"})
			continue
		}
		if _, err := mail.ParseAddress(email); err != nil {
			result.Errors = append(result.Errors, models.ImportRowError{Row: rowNum, Message: "メールアドレスの形式が不正です"})
			continue
		}
		if role != "viewer" && role != "editor" && role != "admin" {
			result.Errors = append(result.Errors, models.ImportRowError{Row: rowNum, Message: "ロールは viewer, editor, admin のいずれかを指定してください"})
			continue
		}

		// ファイル内重複チェック
		if firstRow, exists := seenEmails[email]; exists {
			result.Errors = append(result.Errors, models.ImportRowError{
				Row:     rowNum,
				Message: fmt.Sprintf("ファイル内でメールアドレスが重複しています（%d行目と重複）", firstRow),
			})
			continue
		}
		seenEmails[email] = rowNum

		// DB 重複チェック
		_, err := h.Queries.GetUserByEmail(ctx, email)
		if err == nil {
			result.Errors = append(result.Errors, models.ImportRowError{Row: rowNum, Message: "このメールアドレスは既に登録されています"})
			continue
		}
		if err != sql.ErrNoRows {
			logger.Error("インポート中の DB エラー", "error", err, "email", email, "row", rowNum)
			result.Errors = append(result.Errors, models.ImportRowError{Row: rowNum, Message: "データベースエラーが発生しました"})
			continue
		}

		// ユーザー作成
		_, err = h.Queries.CreateUser(ctx, database.CreateUserParams{
			Email:    email,
			Name:     name,
			Role:     role,
			IsActive: true,
		})
		if err != nil {
			logger.Error("ユーザー作成に失敗", "error", err, "email", email, "row", rowNum)
			result.Errors = append(result.Errors, models.ImportRowError{Row: rowNum, Message: "ユーザーの作成に失敗しました"})
			continue
		}

		result.SuccessCount++
	}

	// エラーが多すぎる場合は切り詰め
	if len(result.Errors) > 50 {
		total := len(result.Errors)
		result.Errors = result.Errors[:50]
		result.Errors = append(result.Errors, models.ImportRowError{
			Row:     0,
			Message: fmt.Sprintf("他にも %d 件のエラーがあります", total-50),
		})
	}

	return renderPage(c, "ユーザー一括インポート", components.AdminUserImport(result))
}

// cellValue は行データから指定インデックスのセル値を取得する。
func cellValue(row []string, idx int) string {
	if idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

// isEmptyRow は行が空かどうか判定する。
func isEmptyRow(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}
