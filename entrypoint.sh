#!/bin/sh
# データディレクトリの所有権を appuser に修正（UID 変更への対応）
# ボリューム内ファイルが別 UID で作られていた場合に自動修復する
if [ -d /app/data ]; then
    chown -R appuser:appuser /app/data 2>/dev/null || true
fi

# バケット名のデフォルト: <プロジェクト名>-litestream
LITESTREAM_BUCKET="${LITESTREAM_BUCKET:-${APP_NAME}-litestream}"
export LITESTREAM_BUCKET

if [ -n "$LITESTREAM_ACCESS_KEY_ID" ]; then
    echo "Litestream: レプリケーション有効"
    if [ ! -f /app/data/app.db ]; then
        echo "Litestream: R2からリストアを試みます..."
        su-exec appuser litestream restore -config /etc/litestream.yml -if-replica-exists /app/data/app.db
    fi
    exec su-exec appuser litestream replicate -exec "/app/server $*" -config /etc/litestream.yml
else
    echo "Litestream: 無効（認証情報未設定）"
    exec su-exec appuser /app/server "$@"
fi
