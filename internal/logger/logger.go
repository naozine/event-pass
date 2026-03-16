package logger

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	// App はアプリケーションログ用のロガー
	App *slog.Logger
	// accessWriter はアクセスログの出力先
	accessWriter io.Writer
	// appWriter はアプリログの出力先
	appWriter io.Writer
	// lumberjack ロガー（クローズ用）
	accessLogger *lumberjack.Logger
	appLogger    *lumberjack.Logger
)

// Config はロガーの設定
type Config struct {
	// LogDir はログファイルの出力ディレクトリ（空の場合は標準出力）
	LogDir string
	// AccessLogFile はアクセスログのファイル名
	AccessLogFile string
	// AppLogFile はアプリログのファイル名
	AppLogFile string
	// MaxSize はローテーション前の最大ファイルサイズ（MB）
	MaxSize int
	// MaxBackups は保持する古いログファイルの最大数
	MaxBackups int
	// MaxAge は古いログファイルを保持する最大日数
	MaxAge int
	// Compress は古いログファイルを圧縮するかどうか
	Compress bool
}

// DefaultConfig はデフォルトの設定を返す
func DefaultConfig() Config {
	return Config{
		LogDir:        "", // 空 = 標準出力
		AccessLogFile: "access.log",
		AppLogFile:    "app.log",
		MaxSize:       100, // 100MB
		MaxBackups:    3,
		MaxAge:        28, // 28日
		Compress:      true,
	}
}

// Init はロガーを初期化する
func Init(cfg Config) error {
	if cfg.LogDir != "" {
		// ディレクトリが存在しなければ作成
		if err := os.MkdirAll(cfg.LogDir, 0755); err != nil {
			return err
		}

		// アクセスログ（lumberjack でローテーション）
		accessLogger = &lumberjack.Logger{
			Filename:   filepath.Join(cfg.LogDir, cfg.AccessLogFile),
			MaxSize:    cfg.MaxSize,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAge,
			Compress:   cfg.Compress,
		}
		accessWriter = accessLogger

		// アプリログ（lumberjack でローテーション）
		appLogger = &lumberjack.Logger{
			Filename:   filepath.Join(cfg.LogDir, cfg.AppLogFile),
			MaxSize:    cfg.MaxSize,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAge,
			Compress:   cfg.Compress,
		}
		appWriter = appLogger
	} else {
		// 標準出力に出力
		accessWriter = os.Stdout
		appWriter = os.Stderr
	}

	// アプリログ用の slog ロガーを作成（JSON形式）
	App = slog.New(slog.NewJSONHandler(appWriter, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	return nil
}

// AccessWriter はアクセスログの出力先を返す
func AccessWriter() io.Writer {
	if accessWriter == nil {
		return os.Stdout
	}
	return accessWriter
}

// Close はログファイルをクローズする
func Close() {
	if accessLogger != nil {
		accessLogger.Close()
	}
	if appLogger != nil {
		appLogger.Close()
	}
}

// Info はINFOレベルのログを出力する
func Info(msg string, args ...any) {
	if App != nil {
		App.Info(msg, args...)
	}
}

// Warn はWARNレベルのログを出力する
func Warn(msg string, args ...any) {
	if App != nil {
		App.Warn(msg, args...)
	}
}

// Error はERRORレベルのログを出力する
func Error(msg string, args ...any) {
	if App != nil {
		App.Error(msg, args...)
	}
}
