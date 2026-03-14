#!/bin/sh
# データディレクトリの所有権を appuser に修正（UID 変更への対応）
# ボリューム内ファイルが別 UID で作られていた場合に自動修復する
if [ -d /app/data ]; then
    chown -R appuser:appuser /app/data 2>/dev/null || true
fi

# appuser に降格して実行
exec su-exec appuser /app/server "$@"
