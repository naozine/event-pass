// =============================================================================
// k6 負荷テスト: マジックリンクログインフロー
// =============================================================================
// 使い方:
//   k6 run loadtest/login_flow.js --env BASE_URL=https://your-app.fly.dev
//
// 前提条件:
//   - .bypass_emails にテスト用メールアドレスが登録済み
//   - テスト用ユーザーが users テーブルに存在
//
// 段階的に同時ユーザー数を増やし、Login → Verify のフルフローを実行する。
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
// BASE_URL は --env で指定（例: --env BASE_URL=https://your-app.fly.dev）
const BASE_URL = __ENV.BASE_URL || "http://localhost:8080";

// テスト用メールアドレス（.bypass_emails に登録済みであること）
const TEST_EMAIL = __ENV.TEST_EMAIL || "test@example.com";

// 段階的にユーザー数を増やす
export const options = {
  stages: [
    { duration: "10s", target: 10 },  // ウォームアップ: 10秒で10ユーザーまで増加
    { duration: "20s", target: 25 },  // 25ユーザーで20秒間
    { duration: "20s", target: 50 },  // 50ユーザーで20秒間
    { duration: "20s", target: 100 }, // 100ユーザーで20秒間
    { duration: "10s", target: 0 },   // クールダウン
  ],
  thresholds: {
    http_req_failed: ["rate<0.1"],           // エラー率 10% 未満
    login_duration: ["p(95)<5000"],           // Login の 95%ile が 5秒未満
    verify_duration: ["p(95)<5000"],          // Verify の 95%ile が 5秒未満
    http_req_duration: ["p(95)<5000"],        // 全体の 95%ile が 5秒未満
  },
};

// ---------------------------------------------------------------------------
// メインシナリオ: Login → Verify
// ---------------------------------------------------------------------------
export default function () {
  // Phase 1: POST /auth/login
  const loginRes = http.post(
    `${BASE_URL}/auth/login`,
    JSON.stringify({ email: TEST_EMAIL }),
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
      `login failed: status=${loginRes.status}, body=${loginRes.body}`
    );
    return;
  }

  // トークンを抽出
  const body = JSON.parse(loginRes.body);
  const magicLinkUrl = new URL(body.magic_link);
  const token = magicLinkUrl.searchParams.get("token");

  if (!token) {
    loginErrors.add(1);
    console.error("token extraction failed");
    return;
  }

  // Phase 2: GET /auth/verify?token=xxx
  // リダイレクト（302）を自動追従しない設定
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
      // クッキー名はプロジェクトにより異なる
      return Object.keys(cookies).length > 0;
    },
  });

  if (!verifyOk) {
    verifyErrors.add(1);
    console.error(
      `verify failed: status=${verifyRes.status}, location=${verifyRes.headers["Location"]}`
    );
  }

  // 実ユーザーの操作間隔をシミュレート（0.5〜1.5秒）
  sleep(Math.random() + 0.5);
}
