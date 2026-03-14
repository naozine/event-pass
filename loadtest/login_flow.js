// =============================================================================
// k6 負荷テスト: マジックリンクログインフロー（複数ユーザー版）
// =============================================================================
// 使い方:
//   k6 run loadtest/login_flow.js --env BASE_URL=https://your-app.fly.dev
//
// 前提条件:
//   - .bypass_emails に *@loadtest.example.com が登録済み
//   - ADMIN_EMAIL のユーザーが存在し、.bypass_emails にも登録済み
//   - setup() がテストユーザーを自動作成するため、事前準備不要
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
const ADMIN_EMAIL = __ENV.ADMIN_EMAIL || "test@example.com";
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
// setup: 管理者でログインし、テストユーザーを一括作成
// ---------------------------------------------------------------------------
export function setup() {
  console.log(`Setup: ${NUM_USERS} 件のテストユーザーを作成中...`);

  // 1. 管理者でログイン → セッション Cookie 取得
  const loginRes = http.post(
    `${BASE_URL}/auth/login`,
    JSON.stringify({ email: ADMIN_EMAIL }),
    { headers: { "Content-Type": "application/json" } }
  );

  if (loginRes.status !== 200) {
    throw new Error(
      `管理者ログインに失敗: status=${loginRes.status}, body=${loginRes.body}`
    );
  }

  const loginBody = JSON.parse(loginRes.body);
  if (!loginBody.magic_link) {
    throw new Error(
      `magic_link が返されませんでした（${ADMIN_EMAIL} は .bypass_emails に登録済みですか？）`
    );
  }

  const magicLinkUrl = new URL(loginBody.magic_link);
  const token = magicLinkUrl.searchParams.get("token");

  // Verify でセッション Cookie を取得
  const verifyRes = http.get(`${BASE_URL}/auth/verify?token=${token}`, {
    redirects: 0,
  });

  if (verifyRes.status !== 302) {
    throw new Error(`管理者 Verify に失敗: status=${verifyRes.status}`);
  }

  // セッション Cookie を取得（k6 は jar でクッキーを自動管理）
  const jar = http.cookieJar();
  const cookies = jar.cookiesForURL(BASE_URL);
  console.log(`管理者セッション取得完了（Cookie keys: ${Object.keys(cookies)}）`);

  // 2. テストユーザーをインポート API で一括作成
  //    Excel ファイルの代わりに、個別に POST /admin/users/new で作成
  const emails = [];
  let created = 0;
  let skipped = 0;

  for (let i = 1; i <= NUM_USERS; i++) {
    const email = `loadtest-${String(i).padStart(4, "0")}@loadtest.example.com`;
    const name = `LoadTest User ${i}`;

    const res = http.post(
      `${BASE_URL}/admin/users/new`,
      { name: name, email: email, role: "viewer" },
      { redirects: 0 }
    );

    if (res.status === 303) {
      created++;
    } else {
      // 既に存在する場合など
      skipped++;
    }

    emails.push(email);
  }

  console.log(
    `Setup 完了: ${created} 件作成, ${skipped} 件スキップ, 合計 ${emails.length} 件`
  );

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
  const magicLinkUrl = new URL(body.magic_link);
  const token = magicLinkUrl.searchParams.get("token");

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
