// Authentication and WebAuthn functions

// Conditional UI (Passkey autofill) initialization
// ページロード時にパスキーの自動補完を開始
document.addEventListener('DOMContentLoaded', async function() {
    // セットアップページではConditional UIを開始しない（ユーザーが存在しないため）
    if (window.location.pathname === '/setup') return;
    if (!window.MagicLink) return;

    try {
        await MagicLink.conditionalLogin();
    } catch (e) {
        if (e.name !== 'AbortError' && e.name !== 'NotAllowedError') {
            showAuthMessage("パスキー認証に失敗しました。別の方法でログインしてください。", true);
        }
    }
});

// Login Form Handler (fetch)
document.addEventListener('DOMContentLoaded', function() {
    const form = document.getElementById('login-form');
    if (!form) return;

    form.addEventListener('submit', async function(e) {
        e.preventDefault();
        const email = document.getElementById('email').value;

        try {
            const res = await fetch('/auth/login', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ email })
            });
            const resp = await res.json();
            const msgDiv = document.getElementById('auth-messages');
            if (!msgDiv) return;

            msgDiv.classList.remove('hidden', 'bg-red-50', 'text-red-700', 'bg-green-50', 'text-green-700');

            if (res.ok) {
                msgDiv.classList.add('bg-green-50', 'text-green-700');
                if (resp.magic_link) {
                    msgDiv.innerHTML = '<p class="font-bold mb-2">' + resp.message + '</p>' +
                        '<a href="' + resp.magic_link + '" class="underline break-all hover:text-green-900">ログインリンクをクリックして続行</a>';
                } else {
                    msgDiv.textContent = resp.message;
                }
            } else {
                msgDiv.classList.add('bg-red-50', 'text-red-700');
                msgDiv.textContent = resp.error || "エラーが発生しました。";
            }
            msgDiv.classList.remove('hidden');
        } catch (e) {
            showAuthMessage("エラーが発生しました。", true);
        }
    });
});

// Helper to show messages (on login page)
function showAuthMessage(text, isError = false) {
    const el = document.getElementById('auth-messages');
    if (!el) {
        // Fallback for pages without the message container (e.g. dashboard)
        alert(text);
        return;
    }
    el.textContent = text;
    el.className = isError
        ? 'p-4 rounded-md bg-red-50 text-red-700'
        : 'p-4 rounded-md bg-green-50 text-green-700';
    el.classList.remove('hidden');
}

// WebAuthn Functions

async function registerPasskey(email) {
    if (!email) {
        showAuthMessage("メールアドレスが見つかりません。", true);
        return;
    }

    if (!window.MagicLink) {
        console.error("MagicLink JS library not loaded");
        return;
    }

    try {
        // Conditional UI が実行中の場合は中止する（WebAuthn は同時に1つの操作のみ許可）
        MagicLink.abortConditionalLogin();

        // If on dashboard, we might use alert for progress
        const isDashboard = !document.getElementById('auth-messages');
        if(isDashboard) {
            if(!confirm("この端末をパスキーとして登録しますか？")) return;
        } else {
            showAuthMessage("パスキーを登録中...", false);
        }

        const res = await MagicLink.register(email);

        if (res.success) {
            // Always reload to reflect the new passkey status from server
            location.reload();
        }
    } catch (e) {
        if (!document.getElementById('auth-messages')) alert("エラー: " + e.message);
        else showAuthMessage("エラー: " + e.message, true);
    }
}

async function loginPasskey() {
    const emailInput = document.getElementById('email');
    if (!emailInput) return;
    const email = emailInput.value;

    if (!email) {
        showAuthMessage("メールアドレスを入力してください。", true);
        return;
    }

    try {
        // Conditional UI が実行中の場合は中止する
        MagicLink.abortConditionalLogin();

        showAuthMessage("パスキーでログイン中...", false);
        const res = await MagicLink.login(email);
        if (res.success) {
            // Redirection is now handled by the MagicLink library itself via webauthn.js
        }
    } catch (e) {
        showAuthMessage("ログインエラー: " + e.message, true);
    }
}

async function loginDiscoverable() {
    try {
        // Conditional UI が実行中の場合は中止する
        MagicLink.abortConditionalLogin();

        showAuthMessage("端末認証でログイン中...", false);
        const res = await MagicLink.loginDiscoverable();
        if (res.success) {
            // Redirection is now handled by the MagicLink library itself via webauthn.js
        }
    } catch (e) {
        showAuthMessage("ログインエラー: " + e.message, true);
    }
}
