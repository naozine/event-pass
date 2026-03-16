// Package models はハンドラとテンプレート間で共有する型定義を格納する。
package models

// ImportRowError はインポート時の行ごとのエラー。
type ImportRowError struct {
	Row     int
	Message string
}

// ImportResult はインポートの結果。
type ImportResult struct {
	SuccessCount int
	Errors       []ImportRowError
}
