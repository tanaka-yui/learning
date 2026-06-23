package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

// newTestApp はテスト用の app を返す。
// SMTPはテストでは到達不能ホストを指す。メール送信はベストエフォートのため
// トークン発行自体は SMTP の可否に依存しない(マジックリンクのテストで利用)。
func newTestApp(t *testing.T) *app {
	t.Helper()
	t.Setenv("MAILPIT_HOST", "127.0.0.1:1") // 到達不能(送信失敗してもトークンは発行される)
	a, err := newApp()
	if err != nil {
		t.Fatalf("app初期化に失敗: %v", err)
	}
	return a
}

// newTestServer はテスト用サーバを起動する。
func newTestServer(t *testing.T) (*httptest.Server, *app) {
	t.Helper()
	a := newTestApp(t)
	return httptest.NewServer(setupRouter(a)), a
}

// ---------------- TOTP ----------------

// TestTOTPEnrollThenVerify は enroll 後、同じシークレットで生成した正しいコードが
// 受理され、誤ったコードが拒否されることを検証する。
func TestTOTPEnrollThenVerify(t *testing.T) {
	server, a := newTestServer(t)
	defer server.Close()

	// enroll: シークレットと otpauth URI を取得する
	resp, err := http.Post(server.URL+"/totp/enroll", "application/json", nil)
	if err != nil {
		t.Fatalf("enrollリクエスト失敗: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("enroll status = %d, want 200", resp.StatusCode)
	}
	var enroll map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&enroll); err != nil {
		t.Fatalf("enrollレスポンスのデコード失敗: %v", err)
	}
	secret := enroll["secret"]
	if secret == "" {
		t.Fatal("secret が空です")
	}
	if enroll["otpauth_uri"] == "" {
		t.Fatal("otpauth_uri が空です")
	}

	// サーバ側に保存されたシークレットとレスポンスのシークレットは一致するはず
	u, _ := a.users.ByName("alice")
	if u.TOTPSecret != secret {
		t.Fatalf("保存されたシークレットがレスポンスと不一致")
	}

	// 同じシークレットから現在時刻の正しいコードを生成する
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("コード生成失敗: %v", err)
	}

	// verify: 正しいコードは 200
	if got := totpVerify(t, server, code); got != http.StatusOK {
		t.Fatalf("正しいコードの verify status = %d, want 200", got)
	}

	// verify: 誤ったコードは 401
	if got := totpVerify(t, server, "000000"); got != http.StatusUnauthorized {
		t.Fatalf("誤ったコードの verify status = %d, want 401", got)
	}
}

// totpVerify は /totp/verify にコードを送りステータスコードを返す。
func totpVerify(t *testing.T, server *httptest.Server, code string) int {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"code": code})
	resp, err := http.Post(server.URL+"/totp/verify", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("verifyリクエスト失敗: %v", err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

// ---------------- Magic Link ----------------

// TestMagicLinkSingleUse は request でトークンが発行され、verify で1回だけ消費でき、
// 同じトークンの2回目の使用が失敗する(使い捨て)ことを検証する。
func TestMagicLinkSingleUse(t *testing.T) {
	server, a := newTestServer(t)
	defer server.Close()

	// request: マジックリンクを要求(SMTPは到達不能だがトークンは発行される)
	reqBody, _ := json.Marshal(map[string]string{"email": "alice@example.com"})
	resp, err := http.Post(server.URL+"/magic/request", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("requestリクエスト失敗: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("request status = %d, want 200", resp.StatusCode)
	}

	// テストではトークンをインプロセスで直接取得する(SMTP不要)
	token := a.magic.peekToken(t)

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	// verify 1回目: 成功(200) + セッションCookie
	r1, err := client.Get(server.URL + "/magic/verify?token=" + token)
	if err != nil {
		t.Fatalf("verify 1回目失敗: %v", err)
	}
	if r1.StatusCode != http.StatusOK {
		t.Fatalf("verify 1回目 status = %d, want 200", r1.StatusCode)
	}
	if sessionCookie(r1) == nil {
		t.Error("verify 1回目でセッションCookieが設定されていません")
	}

	// verify 2回目(同じトークン): 使い捨てのため失敗(401)
	r2, err := client.Get(server.URL + "/magic/verify?token=" + token)
	if err != nil {
		t.Fatalf("verify 2回目失敗: %v", err)
	}
	if r2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("verify 2回目 status = %d, want 401 (使い捨て)", r2.StatusCode)
	}
}

// TestMagicLinkInvalidToken は不正なトークンが拒否されることを検証する。
func TestMagicLinkInvalidToken(t *testing.T) {
	server, _ := newTestServer(t)
	defer server.Close()

	resp, err := http.Get(server.URL + "/magic/verify?token=does-not-exist")
	if err != nil {
		t.Fatalf("verifyリクエスト失敗: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("不正トークンの status = %d, want 401", resp.StatusCode)
	}
}

// sessionCookie はレスポンスから session_id Cookie を取り出す。
func sessionCookie(resp *http.Response) *http.Cookie {
	for _, c := range resp.Cookies() {
		if c.Name == sessionCookieName {
			return c
		}
	}
	return nil
}

// ---------------- WebAuthn ----------------
// 完全なWebAuthnセレモニーはブラウザ/認証器が必要なため、ここでは
// /webauthn/register/begin が妥当なオプションJSONを返し SessionData を保存すること、
// および /webauthn/login/begin が未登録時にクリーンにエラーを返すことのみ検証する。
// 完全な登録/ログインはブラウザでの動作確認に委ねる(docs/06参照)。

// TestWebAuthnRegisterBeginReturnsOptions は register/begin が有効なオプションを返し
// SessionData を保存することを検証する。
func TestWebAuthnRegisterBeginReturnsOptions(t *testing.T) {
	server, a := newTestServer(t)
	defer server.Close()

	resp, err := http.Get(server.URL + "/webauthn/register/begin")
	if err != nil {
		t.Fatalf("register/beginリクエスト失敗: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("register/begin status = %d, want 200", resp.StatusCode)
	}

	// CredentialCreationOptions JSON は publicKey.challenge / publicKey.rp.id を含む
	var got struct {
		PublicKey struct {
			Challenge string `json:"challenge"`
			RP        struct {
				ID string `json:"id"`
			} `json:"rp"`
			User struct {
				ID string `json:"id"`
			} `json:"user"`
		} `json:"publicKey"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("オプションJSONのデコード失敗: %v", err)
	}
	if got.PublicKey.Challenge == "" {
		t.Error("challenge が空です")
	}
	if got.PublicKey.RP.ID != "localhost" {
		t.Errorf("rp.id = %q, want localhost", got.PublicKey.RP.ID)
	}
	if got.PublicKey.User.ID == "" {
		t.Error("user.id が空です")
	}

	// SessionData がサーバ側に保存されているはず
	if _, ok := a.waSessions.LoadRegister("alice"); !ok {
		t.Error("register の SessionData が保存されていません")
	}
}

// TestWebAuthnLoginBeginNoCredential は未登録時に login/begin がクリーンに
// エラー(400)を返すことを検証する。
func TestWebAuthnLoginBeginNoCredential(t *testing.T) {
	server, _ := newTestServer(t)
	defer server.Close()

	resp, err := http.Get(server.URL + "/webauthn/login/begin")
	if err != nil {
		t.Fatalf("login/beginリクエスト失敗: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("未登録時の login/begin status = %d, want 400", resp.StatusCode)
	}
}

// 環境変数の未設定パスでも mailpitHost が既定値を返すことを確認する。
func TestMailpitHostDefault(t *testing.T) {
	os.Unsetenv("MAILPIT_HOST")
	if got := mailpitHost(); got != "localhost:1025" {
		t.Errorf("mailpitHost() = %q, want localhost:1025", got)
	}
}
