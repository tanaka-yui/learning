package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/pquerna/otp/totp"
)

const sessionCookieName = "session_id"

// app はハンドラ群が共有する依存をまとめる。
type app struct {
	users      *UserStore
	sessions   *SessionStore
	magic      *MagicStore
	waSessions *WebAuthnSessionStore
	wa         *webauthn.WebAuthn
	baseURL    string // マジックリンク生成に使う公開URL (例 http://localhost:9300)
	mailHost   string // SMTP 送信先 (Mailpit)
}

// setupRouter は依存を受け取り http.Handler を構築する。
func setupRouter(a *app) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", a.handleIndex)

	// --- TOTP (2FA) ---
	mux.HandleFunc("POST /totp/enroll", a.handleTOTPEnroll)
	mux.HandleFunc("POST /totp/verify", a.handleTOTPVerify)

	// --- WebAuthn / Passkeys ---
	mux.HandleFunc("GET /webauthn/register/begin", a.handleWebAuthnRegisterBegin)
	mux.HandleFunc("POST /webauthn/register/finish", a.handleWebAuthnRegisterFinish)
	mux.HandleFunc("GET /webauthn/login/begin", a.handleWebAuthnLoginBegin)
	mux.HandleFunc("POST /webauthn/login/finish", a.handleWebAuthnLoginFinish)

	// --- Magic Link ---
	mux.HandleFunc("POST /magic/request", a.handleMagicRequest)
	mux.HandleFunc("GET /magic/verify", a.handleMagicVerify)

	return mux
}

// writeJSON はJSONレスポンスを書き出す。
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// demoUser はデモ用の単一ユーザ alice を返す。
// 本デモはパスワード(第1要素)の検証を最小限にとどめ、第2要素/パスワードレスに集中する。
func (a *app) demoUser() (*User, bool) {
	return a.users.ByName("alice")
}

// ---------------- TOTP ----------------

// totpVerifyRequest は TOTP 検証の入力。
type totpVerifyRequest struct {
	Code string `json:"code"`
}

// handleTOTPEnroll はユーザに TOTP シークレットを発行し otpauth:// URI を返す。
func (a *app) handleTOTPEnroll(w http.ResponseWriter, r *http.Request) {
	u, ok := a.demoUser()
	if !ok {
		http.Error(w, "ユーザが見つかりません", http.StatusNotFound)
		return
	}
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "MFA Demo",
		AccountName: u.Email,
	})
	if err != nil {
		http.Error(w, "TOTPシークレットの生成に失敗しました", http.StatusInternalServerError)
		return
	}
	// 発行したシークレットをユーザに紐づけて保存する(検証で使う)。
	u.TOTPSecret = key.Secret()

	writeJSON(w, http.StatusOK, map[string]string{
		"secret":      key.Secret(),
		"otpauth_uri": key.URL(),
		"note":        "認証アプリ(Google Authenticator等)で otpauth_uri を読み込むか secret を手入力し、表示された6桁コードを POST /totp/verify に送信してください。",
	})
}

// handleTOTPVerify は入力された6桁コードを保存済みシークレットで検証する。
func (a *app) handleTOTPVerify(w http.ResponseWriter, r *http.Request) {
	u, ok := a.demoUser()
	if !ok {
		http.Error(w, "ユーザが見つかりません", http.StatusNotFound)
		return
	}
	if u.TOTPSecret == "" {
		http.Error(w, "先に POST /totp/enroll でエンロールしてください", http.StatusBadRequest)
		return
	}
	var req totpVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "不正なリクエスト", http.StatusBadRequest)
		return
	}
	if !totp.Validate(req.Code, u.TOTPSecret) {
		http.Error(w, "コードが一致しません", http.StatusUnauthorized)
		return
	}
	// 第2要素検証OK。学習用に認証済みセッションを発行する。
	sess := a.sessions.Create(u.Username)
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: sess.ID, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "username": u.Username})
}

// ---------------- WebAuthn ----------------

// handleWebAuthnRegisterBegin は登録セレモニーの CredentialCreationOptions を返す。
func (a *app) handleWebAuthnRegisterBegin(w http.ResponseWriter, r *http.Request) {
	u, ok := a.demoUser()
	if !ok {
		http.Error(w, "ユーザが見つかりません", http.StatusNotFound)
		return
	}
	creation, session, err := a.wa.BeginRegistration(u)
	if err != nil {
		http.Error(w, "登録開始に失敗しました: "+err.Error(), http.StatusBadRequest)
		return
	}
	// セレモニー完了時に必要な SessionData をサーバ側に保存する。
	a.waSessions.SaveRegister(u.Username, session)
	writeJSON(w, http.StatusOK, creation)
}

// handleWebAuthnRegisterFinish は authenticator の応答を検証し資格情報を保存する。
func (a *app) handleWebAuthnRegisterFinish(w http.ResponseWriter, r *http.Request) {
	u, ok := a.demoUser()
	if !ok {
		http.Error(w, "ユーザが見つかりません", http.StatusNotFound)
		return
	}
	session, ok := a.waSessions.LoadRegister(u.Username)
	if !ok {
		http.Error(w, "登録セッションがありません。先に begin を呼んでください", http.StatusBadRequest)
		return
	}
	cred, err := a.wa.FinishRegistration(u, *session, r)
	if err != nil {
		http.Error(w, "登録検証に失敗しました: "+err.Error(), http.StatusBadRequest)
		return
	}
	u.AddCredential(*cred)
	writeJSON(w, http.StatusOK, map[string]string{"status": "registered"})
}

// handleWebAuthnLoginBegin はログインセレモニーの CredentialAssertion を返す。
func (a *app) handleWebAuthnLoginBegin(w http.ResponseWriter, r *http.Request) {
	u, ok := a.demoUser()
	if !ok {
		http.Error(w, "ユーザが見つかりません", http.StatusNotFound)
		return
	}
	// 資格情報が未登録なら BeginLogin はエラーになる。明示的に弾く。
	if !u.HasCredential() {
		http.Error(w, "登録済みのパスキーがありません。先に register を実行してください", http.StatusBadRequest)
		return
	}
	assertion, session, err := a.wa.BeginLogin(u)
	if err != nil {
		http.Error(w, "ログイン開始に失敗しました: "+err.Error(), http.StatusBadRequest)
		return
	}
	a.waSessions.SaveLogin(u.Username, session)
	writeJSON(w, http.StatusOK, assertion)
}

// handleWebAuthnLoginFinish は assertion 応答を検証しセッションを発行する。
func (a *app) handleWebAuthnLoginFinish(w http.ResponseWriter, r *http.Request) {
	u, ok := a.demoUser()
	if !ok {
		http.Error(w, "ユーザが見つかりません", http.StatusNotFound)
		return
	}
	session, ok := a.waSessions.LoadLogin(u.Username)
	if !ok {
		http.Error(w, "ログインセッションがありません。先に begin を呼んでください", http.StatusBadRequest)
		return
	}
	if _, err := a.wa.FinishLogin(u, *session, r); err != nil {
		http.Error(w, "ログイン検証に失敗しました: "+err.Error(), http.StatusBadRequest)
		return
	}
	sess := a.sessions.Create(u.Username)
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: sess.ID, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "authenticated", "username": u.Username})
}

// ---------------- Magic Link ----------------

// magicRequest はマジックリンク要求の入力。email または username のいずれかで指定する。
type magicRequest struct {
	Email    string `json:"email"`
	Username string `json:"username"`
}

// handleMagicRequest は使い捨てトークンを発行し、リンクをメール送信する。
func (a *app) handleMagicRequest(w http.ResponseWriter, r *http.Request) {
	var req magicRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "不正なリクエスト", http.StatusBadRequest)
		return
	}
	var u *User
	var ok bool
	switch {
	case req.Email != "":
		u, ok = a.users.ByEmail(req.Email)
	case req.Username != "":
		u, ok = a.users.ByName(req.Username)
	default:
		http.Error(w, "email または username を指定してください", http.StatusBadRequest)
		return
	}

	// ユーザ列挙を避けるため、存在有無にかかわらず常に同じ応答を返す。
	// トークン発行は同期的に行い、メール送信のみ非同期(ゴルーチン)にすることで
	// ・ユーザ存在/非存在でレスポンス時間が変わらない(タイミングチャネルを閉じる)
	// ・SMTP への同期ブロックがリクエスト処理をブロックしない
	if ok {
		t := a.magic.Issue(u.Username)
		link := fmt.Sprintf("%s/magic/verify?token=%s", a.baseURL, t.Token)
		mailHost, email := a.mailHost, u.Email
		go func() {
			// メール送信はベストエフォート。SMTPが届かなくてもトークン発行は成立済み。
			if err := sendMagicLinkMail(mailHost, email, link); err != nil {
				log.Printf("マジックリンクのメール送信に失敗(トークンは発行済み): %v", err)
			}
		}()
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
		"note":   "該当アカウントが存在する場合、ログインリンクをメールで送信しました。Mailpit (http://localhost:8025) で確認してください。",
	})
}

// handleMagicVerify はトークンを1回だけ消費し、セッションを確立する。
func (a *app) handleMagicVerify(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "token がありません", http.StatusBadRequest)
		return
	}
	username, ok := a.magic.Consume(token)
	if !ok {
		http.Error(w, "トークンが無効か、期限切れか、既に使用済みです", http.StatusUnauthorized)
		return
	}
	sess := a.sessions.Create(username)
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: sess.ID, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!doctype html><html lang="ja"><head><meta charset="utf-8"><title>ログイン成功</title></head>
<body><h1>ログイン成功</h1><p>%s としてログインしました(マジックリンク)。</p>
<p>このトークンは使い捨てのため、同じリンクを再度開くと失敗します。</p></body></html>`, username)
}
