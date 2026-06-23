package main

import "net/http"

// handleIndex は動作確認用の最小HTMLを返す。
// WebAuthn はブラウザでの操作が必要なため、JSで navigator.credentials を呼び出す。
func (a *app) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(indexHTML))
}

// indexHTML は3方式(TOTP / WebAuthn / Magic Link)を試せる学習用ページ。
// localhost はセキュアコンテキスト扱いのため HTTP でも WebAuthn が動作する。
const indexHTML = `<!doctype html>
<html lang="ja">
<head>
<meta charset="utf-8">
<title>MFA / パスワードレス デモ</title>
<style>
  body { font-family: system-ui, sans-serif; max-width: 760px; margin: 2rem auto; line-height: 1.6; padding: 0 1rem; }
  section { border: 1px solid #ccc; border-radius: 8px; padding: 1rem 1.25rem; margin-bottom: 1.25rem; }
  h2 { margin-top: 0; }
  button { padding: .4rem .9rem; margin-right: .5rem; cursor: pointer; }
  input { padding: .35rem; }
  pre { background: #f4f4f4; padding: .75rem; border-radius: 6px; white-space: pre-wrap; word-break: break-all; }
</style>
</head>
<body>
<h1>MFA / パスワードレス デモ</h1>
<p>シードユーザ: <code>alice</code> (email: <code>alice@example.com</code>)</p>

<section>
  <h2>1. TOTP (2要素認証)</h2>
  <p>エンロールして otpauth URI を取得し、認証アプリで6桁コードを生成して検証します。</p>
  <button onclick="totpEnroll()">enroll</button>
  <input id="totpCode" placeholder="6桁コード" size="8">
  <button onclick="totpVerify()">verify</button>
  <pre id="totpOut"></pre>
</section>

<section>
  <h2>2. WebAuthn / Passkeys</h2>
  <p>このブラウザの認証器(指紋・PIN・パスキー)で登録→ログインします。</p>
  <button onclick="register()">register</button>
  <button onclick="loginPasskey()">login</button>
  <pre id="waOut"></pre>
</section>

<section>
  <h2>3. Magic Link</h2>
  <p>リンクを要求すると Mailpit (<a href="http://localhost:8025" target="_blank">http://localhost:8025</a>) にメールが届きます。本文のリンクを開いてログインします。</p>
  <input id="magicId" value="alice@example.com" size="28">
  <button onclick="magicRequest()">request link</button>
  <pre id="magicOut"></pre>
</section>

<script>
// --- base64url <-> ArrayBuffer 変換ヘルパ ---
function b64urlToBuf(b64url) {
  const b64 = b64url.replace(/-/g, '+').replace(/_/g, '/').padEnd(Math.ceil(b64url.length / 4) * 4, '=');
  const bin = atob(b64);
  const buf = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) buf[i] = bin.charCodeAt(i);
  return buf.buffer;
}
function bufToB64url(buf) {
  const bytes = new Uint8Array(buf);
  let bin = '';
  for (const b of bytes) bin += String.fromCharCode(b);
  return btoa(bin).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}
function show(id, obj) { document.getElementById(id).textContent = typeof obj === 'string' ? obj : JSON.stringify(obj, null, 2); }

// --- TOTP ---
async function totpEnroll() {
  const r = await fetch('/totp/enroll', { method: 'POST' });
  show('totpOut', await r.json());
}
async function totpVerify() {
  const code = document.getElementById('totpCode').value;
  const r = await fetch('/totp/verify', {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ code })
  });
  show('totpOut', r.status + ' ' + (await r.text()));
}

// --- WebAuthn 登録 ---
async function register() {
  try {
    const r = await fetch('/webauthn/register/begin');
    const options = (await r.json()).publicKey;
    options.challenge = b64urlToBuf(options.challenge);
    options.user.id = b64urlToBuf(options.user.id);
    if (options.excludeCredentials) {
      options.excludeCredentials.forEach(c => c.id = b64urlToBuf(c.id));
    }
    const cred = await navigator.credentials.create({ publicKey: options });
    const body = {
      id: cred.id,
      rawId: bufToB64url(cred.rawId),
      type: cred.type,
      response: {
        attestationObject: bufToB64url(cred.response.attestationObject),
        clientDataJSON: bufToB64url(cred.response.clientDataJSON),
      },
    };
    const fin = await fetch('/webauthn/register/finish', {
      method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body)
    });
    show('waOut', 'register: ' + fin.status + ' ' + (await fin.text()));
  } catch (e) { show('waOut', 'error: ' + e); }
}

// --- WebAuthn ログイン ---
async function loginPasskey() {
  try {
    const r = await fetch('/webauthn/login/begin');
    if (!r.ok) { show('waOut', 'login begin: ' + r.status + ' ' + (await r.text())); return; }
    const options = (await r.json()).publicKey;
    options.challenge = b64urlToBuf(options.challenge);
    if (options.allowCredentials) {
      options.allowCredentials.forEach(c => c.id = b64urlToBuf(c.id));
    }
    const assertion = await navigator.credentials.get({ publicKey: options });
    const body = {
      id: assertion.id,
      rawId: bufToB64url(assertion.rawId),
      type: assertion.type,
      response: {
        authenticatorData: bufToB64url(assertion.response.authenticatorData),
        clientDataJSON: bufToB64url(assertion.response.clientDataJSON),
        signature: bufToB64url(assertion.response.signature),
        userHandle: assertion.response.userHandle ? bufToB64url(assertion.response.userHandle) : null,
      },
    };
    const fin = await fetch('/webauthn/login/finish', {
      method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body)
    });
    show('waOut', 'login: ' + fin.status + ' ' + (await fin.text()));
  } catch (e) { show('waOut', 'error: ' + e); }
}

// --- Magic Link ---
async function magicRequest() {
  const id = document.getElementById('magicId').value;
  const r = await fetch('/magic/request', {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email: id })
  });
  show('magicOut', await r.json());
}
</script>
</body>
</html>`
