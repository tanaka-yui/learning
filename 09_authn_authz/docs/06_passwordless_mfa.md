# パスワードレス認証と MFA(TOTP / WebAuthn / Magic Link)

## 1. 概要

### パスワードレス / MFA とは

パスワードは「知識(本人だけが知っている情報)」という単一要素に依存する。漏洩・使い回し・フィッシングに弱く、これだけで本人確認を行うのはリスクが高い。そこで登場するのが次の2つの考え方である。

- **MFA(Multi-Factor Authentication, 多要素認証)**: パスワードに加えて、もう1つ以上の独立した要素で本人確認する。最も身近な例が「パスワード + ワンタイムコード(TOTP)」の 2要素認証(2FA)。
- **パスワードレス認証**: そもそもパスワードを使わず、別の要素だけで認証する。WebAuthn(パスキー)や Magic Link が代表例。

### 認証要素の3分類

| 分類 | 英語 | 例 |
|------|------|-----|
| 知識情報 | Something you know | パスワード、PIN、秘密の質問 |
| 所持情報 | Something you have | スマホ(TOTPアプリ)、ハードウェアキー、メール受信箱 |
| 生体情報 | Something you are | 指紋、顔、虹彩 |

**多要素** とは、これらの **異なる分類** を組み合わせること。「パスワード + 秘密の質問」はどちらも知識情報なので多要素とは呼べない点に注意。

本章では3つの方式をデモする。

| 方式 | 位置づけ | 主な要素 |
|------|---------|---------|
| TOTP | パスワードへの2要素目(2FA) | 所持情報(シークレットを持つアプリ) |
| WebAuthn / パスキー | パスワードレス | 所持情報(認証器) + 生体/PIN |
| Magic Link | パスワードレス | 所持情報(メール受信箱) |

> **注:** パスワードのハッシュ化・レート制限・タイミング攻撃などの基礎対策は
> `03_security_measures/docs/auth-bypass.md` を参照。本章は第2要素 / パスワードレスの
> 仕組みの理解に集中するため、第1要素(パスワード)の検証は最小限にとどめている。

## 2. 各方式の仕組み

### 2.1 TOTP(Time-based One-Time Password / RFC 6238)

サーバとアプリ(Google Authenticator 等)が **同じシークレット** を共有し、それと **現在時刻** から同じ6桁コードを独立に算出する。コードは30秒ごとに変わる。ネットワーク越しにコードそのものを保存・送信する必要がない(チャレンジ・レスポンス的)。

```
[エンロール時]
ユーザ                     サーバ                       認証アプリ
  |--- POST /totp/enroll --->|
  |                          | totp.Generate() でシークレット生成
  |<-- otpauth:// URI + secret|
  |--------- QRを読み込み / secret手入力 ------------------>|
  |                          |                  (アプリもシークレットを保持)

[検証時]
ユーザ                     サーバ                       認証アプリ
  |<--------- 6桁コードを表示(30秒ごとに更新) ------------|
  |--- POST /totp/verify --->|
  |    {code:"123456"}       | totp.Validate(code, secret)
  |                          |  = HMAC-SHA1(secret, floor(now/30)) を比較
  |<--- 200 ok / 401 fail    |
```

コード自体は時刻に紐づくため、盗まれても次の時間枠では無効。ただし **フィッシングサイトに6桁を入力させる中間者攻撃には弱い**(攻撃者がリアルタイムに本物へ転送できる)。

### 2.2 WebAuthn / パスキー(FIDO2)

公開鍵暗号を使う。認証器(端末のSecure Enclave、YubiKey 等)が **鍵ペア** を生成し、秘密鍵は端末から出さない。サーバには公開鍵だけを登録する。ログイン時はサーバが送る **チャレンジ(乱数)** に秘密鍵で署名し、サーバが公開鍵で検証する。

```
[登録 (Registration)]
ブラウザ/認証器              サーバ(Relying Party)
  |--- GET /webauthn/register/begin -->|
  |                                    | BeginRegistration()
  |<-- CredentialCreationOptions ------|  (challenge, rp.id=localhost ...)
  | navigator.credentials.create()     |
  |  → 鍵ペア生成、秘密鍵は端末に保管 |
  |--- POST .../register/finish ------>|
  |    {attestation, 公開鍵}           | FinishRegistration() → 公開鍵を保存
  |<-- 200 registered -----------------|

[ログイン (Authentication)]
ブラウザ/認証器              サーバ
  |--- GET /webauthn/login/begin ----->|
  |                                    | BeginLogin()
  |<-- CredentialAssertion ------------|  (challenge, allowCredentials)
  | navigator.credentials.get()        |
  |  → 生体/PINで解錠し challengeに署名|
  |--- POST .../login/finish --------->|
  |    {署名, authenticatorData}       | FinishLogin() → 公開鍵で署名検証
  |<-- 200 authenticated --------------|
```

**フィッシング耐性が最も高い。** ブラウザが clientDataJSON に **実際のオリジン**(`http://localhost:9300` 等)を埋め込み、署名対象に含める。偽サイト上では正規オリジンと一致しないため署名が成立せず、たとえ騙されてもクレデンシャルは漏れない。秘密鍵が端末外に出ない点も大きい。

### 2.3 Magic Link

ログイン用の **使い捨て・短命なトークン** を含む URL をメールで送り、メール受信箱の所持を本人確認の根拠にする。パスワードを覚える必要がない。

```
ユーザ                サーバ                  メール(Mailpit)
  |-- POST /magic/request -->|
  |   {email}                | 使い捨てトークン発行(TTL付き)
  |                          | net/smtp でメール送信 ----->|
  |                          |   本文: .../magic/verify?token=xxxx
  |<-- 200 ok ---------------|
  |<----------- 受信箱でメールを開く -------------------- |
  |-- GET /magic/verify?token=xxxx -->|
  |                          | Consume(token): 取得と同時に削除(使い捨て)
  |                          | 有効ならセッション確立
  |<-- 200 ログイン成功 ------|
  |-- 同じリンクを再度開く -->|
  |<-- 401(消費済み) -------|
```

メール経路の安全性に依存する。**フィッシング耐性は中程度**: リンクをそのまま盗み見られると悪用されうるが、TTL と使い捨てで被害窓を絞る。

### フィッシング耐性の比較

| 観点 | TOTP | WebAuthn / パスキー | Magic Link |
|------|------|---------------------|-----------|
| 要素の種類 | 所持(2要素目) | 所持 + 生体/PIN | 所持(メール) |
| パスワードレス | ×(2FA前提) | ○ | ○ |
| フィッシング耐性 | 低(コード転送で突破) | 高(オリジン束縛・秘密鍵非開示) | 中(TTL/使い捨てで緩和) |
| サーバ保管物 | 共有シークレット | 公開鍵のみ | 使い捨てトークン |
| オフライン可否 | ○(時刻のみ) | ○(端末内署名) | ×(メール受信が必要) |
| 主なリスク | 中間者によるコード中継 | 端末紛失時の復旧 | メールアカウント侵害 |

## 3. デモ起動手順

```bash
# プロジェクトルート(09_authn_authz/)から実行
make mfa
```

起動後、ブラウザで `http://localhost:9300` を開くと3方式を試せる操作ページが表示される。Magic Link で送信されるメールは Mailpit が捕捉するため、`http://localhost:8025` で確認できる。

シードユーザ:

| ユーザ名 | メール | パスワード(参考) |
|----------|--------|------------------|
| alice | alice@example.com | password123 |

停止するには:

```bash
make down
```

> **WebAuthn が HTTP で動く理由:** WebAuthn はセキュアコンテキストを要求するが、
> ブラウザは `localhost` を例外的にセキュアと扱う。そのため `http://localhost:9300`
> でも `navigator.credentials` が利用できる。

## 4. 動作確認手順

### 4.1 TOTP

1. ページの「TOTP」セクションで **enroll** を押す。`otpauth_uri` と `secret` が表示される。
2. 認証アプリ(Google Authenticator 等)に `secret` を手入力するか、`otpauth_uri` を QR 化して読み込む。
3. アプリに表示された6桁コードを入力欄に入れ **verify** を押す。`200 ok` なら成功。30秒経つとコードが変わる点に注意。

curl で確認する場合:

```bash
# エンロール(secret を控える)
curl -s -X POST http://localhost:9300/totp/enroll

# アプリで生成した6桁コードを検証
curl -s -X POST http://localhost:9300/totp/verify \
  -H "Content-Type: application/json" \
  -d '{"code":"123456"}'
```

### 4.2 WebAuthn(ブラウザ必須)

1. 「WebAuthn」セクションで **register** を押す。OS の認証ダイアログ(指紋・顔・PIN・パスキー)が出るので承認する。`register: 200 registered` が出れば公開鍵が登録された。
2. 続けて **login** を押す。再び認証ダイアログが出て、解錠すると `login: 200 authenticated` が返り、セッション Cookie が発行される。

> 完全な WebAuthn セレモニーは実機の認証器が必要なため、curl では再現できない。
> ブラウザでの操作で動作確認する。

### 4.3 Magic Link

1. 「Magic Link」セクションでメールアドレス(既定 `alice@example.com`)を確認し **request link** を押す。
2. `http://localhost:8025`(Mailpit)を開く。「ログイン用マジックリンク」というメールが届いているので開く。
3. 本文の `http://localhost:9300/magic/verify?token=...` を開くと「ログイン成功」ページが表示される。
4. **同じリンクをもう一度開く** と、トークンが使い捨てのため `401`(消費済み)になる。これが単回使用の確認になる。

curl で確認する場合:

```bash
# リンク要求(Mailpit にメールが届く)
curl -s -X POST http://localhost:9300/magic/request \
  -H "Content-Type: application/json" \
  -d '{"email":"alice@example.com"}'

# Mailpit でトークンを確認して verify(2回目は 401)
curl -s -o /dev/null -w "%{http_code}\n" "http://localhost:9300/magic/verify?token=<TOKEN>"
```

## 5. コード解説

### 5.1 TOTP — `handlers.go`

```go
// handleTOTPEnroll はユーザに TOTP シークレットを発行し otpauth:// URI を返す。
func (a *app) handleTOTPEnroll(w http.ResponseWriter, r *http.Request) {
    u, _ := a.demoUser()
    key, err := totp.Generate(totp.GenerateOpts{
        Issuer:      "MFA Demo",
        AccountName: u.Email,
    })
    // ...
    u.TOTPSecret = key.Secret()          // サーバ側に保存(検証で使う)
    writeJSON(w, http.StatusOK, map[string]string{
        "secret":      key.Secret(),     // Base32 シークレット
        "otpauth_uri": key.URL(),        // otpauth://totp/... (QR化用)
    })
}
```

`totp.Generate` がシークレットと `otpauth://` URI を持つ Key を返す。`key.URL()` をそのまま QR にすれば認証アプリに取り込める。検証側は次の通り。

```go
// handleTOTPVerify は入力された6桁コードを保存済みシークレットで検証する。
func (a *app) handleTOTPVerify(w http.ResponseWriter, r *http.Request) {
    // ...
    if !totp.Validate(req.Code, u.TOTPSecret) {
        http.Error(w, "コードが一致しません", http.StatusUnauthorized)
        return
    }
    // 検証OK → 認証済みセッションを発行
}
```

`totp.Validate` は Google Authenticator 互換の既定(30秒周期・6桁・SHA1)で、時計のズレを吸収するため前後1周期(Skew=1)も許容する。テストでは **同じシークレット** から `totp.GenerateCode(secret, time.Now())` で正しいコードを生成し、受理されることを確認している。

### 5.2 WebAuthn — `main.go` / `users.go` / `handlers.go`

Relying Party の設定:

```go
wa, err := webauthn.New(&webauthn.Config{
    RPID:          "localhost",                          // 認証を束縛するドメイン
    RPDisplayName: "MFA Demo",
    RPOrigins:     []string{"http://localhost:9300"},    // 許可するオリジン
})
```

`RPID` と `RPOrigins` がフィッシング耐性の要。ブラウザは署名対象に実オリジンを含めるので、別ドメインからのアクセスは検証で弾かれる。

アプリのユーザ型は `webauthn.User` インターフェースを実装する必要がある(`users.go`)。

```go
func (u *User) WebAuthnID() []byte                      { return u.id }          // 安定したハンドル
func (u *User) WebAuthnName() string                    { return u.Username }
func (u *User) WebAuthnDisplayName() string             { return u.Username }
func (u *User) WebAuthnCredentials() []webauthn.Credential { return u.credentials } // 登録済み公開鍵群
```

ハンドラは2段階(begin / finish)で、中間状態(チャレンジ等)を `SessionData` としてサーバ側に保持する。

```go
// 登録開始: オプションJSONを返し、SessionData を保存
creation, session, err := a.wa.BeginRegistration(u)
a.waSessions.SaveRegister(u.Username, session)
writeJSON(w, http.StatusOK, creation)

// 登録完了: authenticator の応答を検証し公開鍵を保存
cred, err := a.wa.FinishRegistration(u, *session, r)
u.AddCredential(*cred)
```

ログインも同じ begin / finish 構造(`BeginLogin` / `FinishLogin`)。未登録の状態で `BeginLogin` を呼ぶとライブラリがエラーを返すため、ハンドラ側でも `HasCredential()` を見て明示的に `400` を返している。

### 5.3 Magic Link — `store.go` / `email.go` / `handlers.go`

使い捨ての肝は `Consume` の実装にある。

```go
// Consume はトークンを1回だけ消費する。
func (m *MagicStore) Consume(token string) (string, bool) {
    m.mu.Lock()
    defer m.mu.Unlock()
    t, ok := m.tokens[token]
    if !ok {
        return "", false
    }
    delete(m.tokens, token)              // 取得した時点で削除 → 2回目は必ず失敗
    if time.Now().After(t.ExpiresAt) {   // TTL チェック
        return "", false
    }
    return t.Username, true
}
```

「取得と同時に削除」することで単回使用を保証し、`ExpiresAt` で有効期間も絞る。トークン自体は `crypto/rand` 由来の32バイト乱数(`newToken`)で推測不可能にしている。

メール送信は `net/smtp` で Mailpit(認証なしの開発用SMTP)へ送る。送信先は環境変数で差し替え可能。

```go
func mailpitHost() string {
    if h := os.Getenv("MAILPIT_HOST"); h != "" {
        return h
    }
    return "localhost:1025"
}

func sendMagicLinkMail(host, to, link string) error {
    from := "no-reply@mfa-demo.local"
    // 開発用SMTPは認証不要のため auth は nil
    return smtp.SendMail(host, nil, from, []string{to}, []byte(buildMail(from, to, link)))
}
```

リクエストハンドラでは、SMTP が届かなくても **トークン発行自体は成立** させる(ベストエフォート)設計にしている。これによりライブSMTPなしでもトークン発行ロジックを単体テストできる。

```go
if err := sendMagicLinkMail(a.mailHost, u.Email, link); err != nil {
    log.Printf("マジックリンクのメール送信に失敗(トークンは発行済み): %v", err)
}
```

加えて、`/magic/request` はユーザの存在有無にかかわらず常に同じ `200` 応答を返し、ユーザ列挙(アカウントの有無の推測)を防いでいる。

## 6. まとめ

- **MFA は「異なる分類の要素」を組み合わせる** こと。同じ知識情報を2つ重ねても多要素にはならない。
- **TOTP** は共有シークレット + 時刻で6桁を独立算出する。手軽だが、コードを偽サイトに入力させる中間者攻撃には弱い。
- **WebAuthn / パスキー** は公開鍵暗号 + オリジン束縛で **フィッシング耐性が最も高い**。秘密鍵は端末から出ず、サーバは公開鍵だけを保管する。begin / finish の2段階で `SessionData` にチャレンジを保持するのが実装パターン。
- **Magic Link** は使い捨て・短命トークンを `Consume` で「取得と同時に削除」して単回使用を保証する。安全性はメール経路に依存する。
- **`crypto/rand`・TTL・使い捨て・ユーザ列挙対策** といった基礎が、どの方式でも共通して効いてくる。
- 実務では「パスワード + WebAuthn」「パスキーのみ」へ移行する流れが主流で、TOTP や Magic Link は補完・フォールバックとして併用されることが多い。
