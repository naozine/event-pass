// =============================================================================
// k6 負荷テスト: マジックリンクログインフロー（複数ユーザー版）
// =============================================================================
// 使い方:
//   k6 run loadtest/login_flow.js --env BASE_URL=https://your-app.fly.dev
//
// 前提条件:
//   - .bypass_emails に *@loadtest.example.com が登録済み
//   - テストユーザー（loadtest-NNNN@loadtest.example.com）が事前に登録済み
//     （loadtest/loadtest_users.xlsx を管理画面からインポート）
//
// 段階的に同時ユーザー数を増やし、Login → Verify のフルフローを実行する。
// 各 VU は異なるユーザーでログインし、セッション蓄積の負荷も検証する。
// =============================================================================

import http from "k6/http";
import { check, sleep } from "k6";
import { Counter, Trend } from "k6/metrics";

// ---------------------------------------------------------------------------
// カスタムメトリクス
// ---------------------------------------------------------------------------
const loginDuration = new Trend("login_duration", true);
const verifyDuration = new Trend("verify_duration", true);
const loginErrors = new Counter("login_errors");
const verifyErrors = new Counter("verify_errors");

// ---------------------------------------------------------------------------
// テスト設定
// ---------------------------------------------------------------------------
const BASE_URL = __ENV.BASE_URL || "http://localhost:8080";
const NUM_USERS = parseInt(__ENV.NUM_USERS || "100");

// 段階的にユーザー数を増やす
export const options = {
  stages: [
    { duration: "10s", target: 10 }, // ウォームアップ
    { duration: "20s", target: 25 }, // 25 VU
    { duration: "20s", target: 50 }, // 50 VU
    { duration: "20s", target: 100 }, // 100 VU
    { duration: "10s", target: 0 }, // クールダウン
  ],
  thresholds: {
    http_req_failed: ["rate<0.1"],
    login_duration: ["p(95)<5000"],
    verify_duration: ["p(95)<5000"],
    http_req_duration: ["p(95)<5000"],
  },
};

// ---------------------------------------------------------------------------
// setup: テストユーザーのメールアドレス一覧を生成
// ---------------------------------------------------------------------------
export function setup() {
  const emails = [];
  for (let i = 1; i <= NUM_USERS; i++) {
    emails.push(`loadtest-${String(i).padStart(4, "0")}@loadtest.example.com`);
  }

  // ヘルスチェックで接続確認
  const healthRes = http.get(`${BASE_URL}/health`);
  if (healthRes.status !== 200) {
    throw new Error(`ヘルスチェック失敗: status=${healthRes.status}`);
  }

  // 最初のユーザーでログイン可能か確認
  const testRes = http.post(
    `${BASE_URL}/auth/login`,
    JSON.stringify({ email: emails[0] }),
    { headers: { "Content-Type": "application/json" } }
  );

  if (testRes.status !== 200) {
    throw new Error(
      `テストユーザーのログインに失敗（事前にインポート済みですか？）: status=${testRes.status}, body=${testRes.body}`
    );
  }

  console.log(`Setup 完了: ${emails.length} 件のテストユーザーを使用`);
  return { emails };
}

// ---------------------------------------------------------------------------
// メインシナリオ: Login → Verify（各 VU が異なるユーザーを使用）
// ---------------------------------------------------------------------------
export default function (data) {
  // VU ごとに異なるユーザーを割り当て（VU ID でローテーション）
  const vuIndex = (__VU - 1) % data.emails.length;
  const email = data.emails[vuIndex];

  // Phase 1: POST /auth/login
  const loginRes = http.post(
    `${BASE_URL}/auth/login`,
    JSON.stringify({ email: email }),
    {
      headers: { "Content-Type": "application/json" },
      tags: { name: "login" },
    }
  );

  loginDuration.add(loginRes.timings.duration);

  const loginOk = check(loginRes, {
    "login: status 200": (r) => r.status === 200,
    "login: has magic_link": (r) => {
      try {
        const body = JSON.parse(r.body);
        return body.magic_link && body.magic_link.length > 0;
      } catch {
        return false;
      }
    },
  });

  if (!loginOk) {
    loginErrors.add(1);
    console.error(
      `login failed: VU=${__VU}, email=${email}, status=${loginRes.status}, body=${loginRes.body}`
    );
    return;
  }

  // トークンを抽出
  const body = JSON.parse(loginRes.body);
  const tokenMatch = body.magic_link.match(/token=([^&]+)/);
  const token = tokenMatch ? tokenMatch[1] : null;

  if (!token) {
    loginErrors.add(1);
    console.error(`token extraction failed: VU=${__VU}`);
    return;
  }

  // Phase 2: GET /auth/verify?token=xxx
  const verifyRes = http.get(`${BASE_URL}/auth/verify?token=${token}`, {
    redirects: 0,
    tags: { name: "verify" },
  });

  verifyDuration.add(verifyRes.timings.duration);

  const verifyOk = check(verifyRes, {
    "verify: status 302": (r) => r.status === 302,
    "verify: redirect to /projects": (r) => {
      const location = r.headers["Location"];
      return location && location.includes("/projects");
    },
    "verify: has session cookie": (r) => {
      const cookies = r.cookies;
      return Object.keys(cookies).length > 0;
    },
  });

  if (!verifyOk) {
    verifyErrors.add(1);
    console.error(
      `verify failed: VU=${__VU}, status=${verifyRes.status}, location=${verifyRes.headers["Location"]}`
    );
  }

  // 実ユーザーの操作間隔をシミュレート（0.5〜1.5秒）
  sleep(Math.random() + 0.5);
}

// ---------------------------------------------------------------------------
// teardown: テスト結果のサマリーログ
// ---------------------------------------------------------------------------
export function teardown(data) {
  console.log(`Teardown: テストユーザー ${data.emails.length} 件を使用しました`);
}
