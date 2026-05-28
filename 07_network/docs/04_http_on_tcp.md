# 04_http_on_tcp: TCP の上に HTTP を載せる

HTTP/1.1 は TCP のバイトストリームの上に「テキスト行の積み重ね」という構造を定義したプロトコルである。`servers/http/` の自前実装でその骨格を手で確認する。

---

## 1. HTTP/1.1 の最小要素

HTTP/1.1 メッセージは 4 つの要素で構成される。

```
<リクエストライン>  ← 例: GET /path HTTP/1.1
<ヘッダー行>        ← 例: Host: localhost:9003
<ヘッダー行>        ← 例: Connection: close
<空行>              ← CRLF のみ（ヘッダ終端の合図）
<本文>              ← Content-Length が 0 なら空
```

### ワイヤ上のバイト列（リクエスト）

```
G  E  T     /     H  T  T  P  /  1  .  1  \r \n
47 45 54 20 2F 20 48 54 54 50 2F 31 2E 31 0D 0A

H  o  s  t  :     l  o  c  a  l  h  o  s  t  \r \n
48 6F 73 74 3A 20 6C 6F 63 61 6C 68 6F 73 74 0D 0A

\r \n          ← 空行（ヘッダ終端）
0D 0A
```

### ワイヤ上のバイト列（レスポンス）

```
H  T  T  P  /  1  .  1     2  0  0     O  K  \r \n
48 54 54 50 2F 31 2E 31 20 32 30 30 20 4F 4B 0D 0A

C  o  n  t  e  n  t  -  T  y  p  e  :  ...  \r \n
C  o  n  t  e  n  t  -  L  e  n  g  t  h  :  ...  \r \n

\r \n          ← 空行（ヘッダ終端）

H  e  l  l  o  ,     w  o  r  l  d  !  \n
```

区切りは常に `\r\n`（CRLF, 0x0D 0x0A）。`\n` だけでは RFC 7230 非準拠になる。

### TCP ストリーム上でのメッセージの流れ

HTTP は TCP のストリームを「テキスト行の連続」として使う。TCP 側からは単なるバイト列に見えるが、アプリ（HTTP）がそのバイト列に意味を与える。

```
クライアント (curl)                     サーバー (servers/http/)
     │                                        │
     │  TCP ストリーム上に流れるバイト列         │
     │ ──────────────────────────────────────>│
     │                                        │
     │  47 45 54 20 2F 20 48 54 54 50 2F 31  │  "GET / HTTP/1.
     │  2E 31 0D 0A 48 6F 73 74 3A 20 6C 6F  │  1\r\nHost: lo
     │  63 61 6C 68 6F 73 74 3A 39 30 30 33  │  calhost:9003
     │  0D 0A 0D 0A                           │  \r\n\r\n
     │                                        │
     │                                        │ bufio.Reader がバッファリング
     │                                        │ ReadString('\n') で 1 行ずつ切り出す
     │                                        │
     │  <──────────────────────────────────── │
     │  48 54 54 50 2F 31 2E 31 20 32 30 30  │  "HTTP/1.1 200
     │  20 4F 4B 0D 0A ...                    │   OK\r\n..."
     │                                        │
```

TCP 自身は「どこがリクエストラインの終わりか」を知らない。知っているのは HTTP 層だけであり、それが L6（プレゼンテーション）の仕事である。

---

## 2. `curl -v` の生 byte = サーバーが受け取る byte

`curl -v` を使うとクライアントが送受信する生のヘッダ行を確認できる。

```
$ curl -v http://localhost:9003/
*   Trying 127.0.0.1:9003...
* Connected to localhost (127.0.0.1) port 9003 (#0)
> GET / HTTP/1.1                   ←── ① リクエストライン
> Host: localhost:9003             ←── ② L5: どのホスト宛か
> User-Agent: curl/7.88.1
> Accept: */*
>                                  ←── ③ 空行（ヘッダ終端）
< HTTP/1.1 200 OK                  ←── ④ ステータスライン
< Content-Type: text/plain         ←── ⑤ L6: MIME タイプ
< Content-Length: 14               ←── ⑤ L6: 本文の長さ
< Connection: close                ←── ② L5: この接続を閉じる
<
Hello, world!                      ←── ⑥ L7: アプリの応答本文
```

`>` 行がクライアントの送信、`<` 行がサーバーの応答である。`>` に見える文字列がそのまま TCP ストリームに流れる。サーバーはこのバイト列を `bufio.Reader` で 1 行ずつ読んで解析する。

---

## 3. `servers/http/` コードウォークスルー

### ParseRequest — リクエストを解析する

[servers/http/http.go:20](../servers/http/http.go#L20)

```go
func ParseRequest(r *bufio.Reader) (*Request, error) {
    line, err := r.ReadString('\n')   // ① リクエストライン 1 行を読む
    ...
    line = strings.TrimRight(line, "\r\n")
    parts := strings.SplitN(line, " ", 3)   // ② "GET", "/", "HTTP/1.1" に分割
    if len(parts) != 3 {
        return nil, errors.New("malformed request line")
    }
    req := &Request{Method: parts[0], Path: parts[1], Proto: parts[2], ...}

    for {
        l, err := r.ReadString('\n')   // ③ ヘッダを 1 行ずつ読む
        ...
        l = strings.TrimRight(l, "\r\n")
        if l == "" { break }           // ④ 空行 → ヘッダ終端
        k, v, ok := strings.Cut(l, ":")
        k = strings.ToLower(strings.TrimSpace(k))
        v = strings.TrimSpace(v)
        req.Headers[k] = v
        switch k {
        case "host":       req.Host = v      // ⑤ L5: ホスト記録
        case "connection": ...               // ⑤ L5: close フラグ記録
        }
    }
    return req, nil
}
```

解析フローの概要：

```
TCP ストリーム
    │
    ▼
r.ReadString('\n')        ← 改行まで読む（① リクエストライン）
    │
    ▼
strings.SplitN(line, " ", 3)  ← "GET" / "/" / "HTTP/1.1"
    │
    ▼
loop: r.ReadString('\n')   ← ヘッダ行を 1 行ずつ
    │   空行 → break        ← ④ ヘッダ終端
    ▼
req.Headers に格納
```

`bufio.Reader` が内部でバッファリングしているため、`conn.Read` が細切れに返してきても `ReadString` は改行が来るまで自動的に継続読み込みする。

`Request` 構造体（[servers/http/http.go:9](../servers/http/http.go#L9)）は解析結果を保持する：

```go
type Request struct {
    Method  string            // "GET", "POST" 等
    Path    string            // "/", "/about" 等
    Proto   string            // "HTTP/1.1"
    Host    string            // L5: "localhost:9003"
    Headers map[string]string // すべてのヘッダ（小文字キー）
    Close   bool              // L5: Connection: close だったか
}
```

`Headers` マップのキーが小文字に正規化される（[http.go:47](../servers/http/http.go#L47)）ため、`req.Headers["content-type"]` と `req.Headers["Content-Type"]` は同じ値を返す。

### handle — ルーティング

[servers/http/server.go:33](../servers/http/server.go#L33)

```go
func (s *Server) handle(conn net.Conn) {
    defer conn.Close()
    r := bufio.NewReader(conn)
    req, err := ParseRequest(r)
    ...
    s.log.Info("L5: parsed request", "host", req.Host, "close", req.Close)
    s.log.Info("L7: route", "method", req.Method, "path", req.Path)

    switch {
    case req.Method != "GET":
        writeResponse(conn, 405, "text/plain", nil, true)
    case req.Path == "/":
        writeResponse(conn, 200, "text/plain", []byte("Hello, world!\n"), req.Close)
    default:
        writeResponse(conn, 404, "text/plain", nil, true)
    }
}
```

ログコメントが層を明示している。`L5: parsed request` はホストと接続管理の解釈（セッション層）、`L7: route` はメソッドとパスによる意味解釈（アプリケーション層）である。

### writeResponse — レスポンスを組み立てる

[servers/http/server.go:57](../servers/http/server.go#L57)

```go
func writeResponse(conn net.Conn, code int, contentType string, body []byte, closeIt bool) {
    reason := map[int]string{200: "OK", 400: "Bad Request", ...}[code]
    hdr := fmt.Sprintf("HTTP/1.1 %d %s\r\nContent-Type: %s\r\nContent-Length: %d\r\n",
        code, reason, contentType, len(body))   // ← L6: ヘッダ組み立て
    if closeIt {
        hdr += "Connection: close\r\n"           // ← L5: 接続継続方針
    }
    hdr += "\r\n"                                // ← 空行（ヘッダ終端）
    _, _ = conn.Write([]byte(hdr))
    if len(body) > 0 {
        _, _ = conn.Write(body)
    }
}
```

レスポンス構築の流れ：

```
ステータスライン   "HTTP/1.1 200 OK\r\n"
Content-Type      "Content-Type: text/plain\r\n"    ← L6
Content-Length    "Content-Length: 14\r\n"           ← L6
Connection        "Connection: close\r\n"            ← L5（closeIt=true のとき）
空行              "\r\n"                              ← ヘッダ終端
本文              "Hello, world!\n"                  ← L7
```

`Content-Length` に `len(body)` を正確に設定することが重要である。クライアントはこの値を信頼して「何バイト読めばボディを全部受け取ったか」を判断するためだ。

---

## 4. 比較: `net/http` で書いた等価実装

標準ライブラリを使えば同じ動作を以下のように書ける。

```go
package main

import (
    "fmt"
    "net/http"
)

func main() {
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodGet {
            http.Error(w, "", http.StatusMethodNotAllowed)
            return
        }
        w.Header().Set("Content-Type", "text/plain")
        fmt.Fprintln(w, "Hello, world!")
    })
    http.ListenAndServe(":9003", nil)
}
```

`net/http` が隠している処理を列挙する：

| 処理 | net/http が担う場所 | servers/http/ での対応箇所 |
|------|--------------------|-----------------------------|
| `\r\n` でヘッダを区切る | `net/http` 内部パーサ | [http.go:39](../servers/http/http.go#L39) `TrimRight` |
| ヘッダキーを小文字正規化 | `textproto.CanonicalMIMEHeaderKey` | [http.go:47](../servers/http/http.go#L47) `ToLower` |
| `Content-Length` の自動付与 | `ResponseWriter.Write` が計算 | [server.go:60](../servers/http/server.go#L60) 手動 `len(body)` |
| 複数リクエストのループ処理 | `http.Server` が管理 | `handle` が 1 リクエストで返る（本実装は close 前提） |
| `Connection: keep-alive` の実装 | `http.Server` が管理 | 本実装では `req.Close` が true のとき close |

本実装が「keep-alive を完全実装していない」点は意図的である。`handle` は 1 リクエストを処理して `conn.Close()` するため、HTTP/1.1 の持続接続を省いた HTTP/1.0 相当の動作になっている。

### 実装コストの差

`net/http` が抽象化している処理を追うと、プロダクション品質の HTTP サーバーには本実装のコードに加えて以下が必要なことがわかる。

```
本実装が持つ処理:
  ParseRequest (http.go)
    - リクエストライン解析
    - ヘッダ解析（キー正規化・Host・Connection 抽出）
  handle (server.go)
    - 1 リクエストを処理して閉じる
  writeResponse (server.go)
    - ステータス行・Content-Type・Content-Length・Connection ヘッダ

net/http が追加する処理:
  - keep-alive（同一接続で複数リクエストのループ）
  - リクエストボディの読み取り（Content-Length / chunked）
  - Transfer-Encoding: chunked のエンコード・デコード
  - タイムアウト管理（ReadHeaderTimeout / WriteTimeout）
  - TLS ハンドシェイク（ListenAndServeTLS）
  - HTTP/2 ネゴシエーション（ALPN）
  - 最大ヘッダサイズ制限（DoS 対策）
  - 100 Continue 応答
  - Trailer ヘッダ
```

これだけの差分があってもコードの見た目は `http.HandleFunc` 数行になる。「標準ライブラリが何を隠しているか」を一度手で実装することで、この隠蔽の価値と代償が体感できる。

---

## 5. 落とし穴

### `\r\n` と `\n` の混同

HTTP/1.1 の仕様（RFC 7230）はヘッダ区切りに必ず `\r\n`（CRLF）を要求する。サーバーが `\n` だけで終端判定していると、`\r` が次のヘッダキーの先頭に残ってパース失敗する。[http.go:25](../servers/http/http.go#L25) の `TrimRight(line, "\r\n")` は両方を取り除くため安全だが、レスポンス生成側（[server.go:60](../servers/http/server.go#L60)）が `\r\n` を手書きするコードは、`\n` だけにすると curl 等の厳格なクライアントで問題が出る。実際の `net/http` は受信側で `\n` 単独も受け付ける寛容モードを持つが、送信側は常に `\r\n` を使う。

### `Content-Length` を無視するとどうなるか

クライアントは `Content-Length` ヘッダの値だけを信頼して「ボディの読み終わり」を判断する。このヘッダを省略したり実際のバイト数と異なる値を送ると、クライアントは読み過ぎ（次のレスポンスのバイトを読む）または読み足り（ボディが途中で切れる）が起きる。chunked 転送エンコーディングを使う場合は `Content-Length` の代わりにチャンクサイズを埋め込む別の機構が必要になる。

### ヘッダ名の大小文字

HTTP はヘッダ名を大文字小文字区別なしと定めている（RFC 7230 §3.2）。`Content-Type` も `content-type` も同義である。[http.go:47](../servers/http/http.go#L47) で `strings.ToLower` を使って正規化しているのはそのためだ。逆にレスポンスを生成するとき大文字小文字どちらで送っても仕様上は正しいが、慣例として `Content-Type` のようにタイトルケースが使われる。

### HTTP/0.9 との違い

HTTP/0.9（1991 年）はリクエストが `GET /path\r\n` 1 行だけで、レスポンスもステータス行なしの生 HTML 本文だった。HTTP/1.0 でステータス行とヘッダが追加され、HTTP/1.1 で `Host` ヘッダ必須化・`Connection: keep-alive` による持続接続・チャンク転送が加わった。本実装の `ParseRequest` が `HTTP/1.1` という文字列をリクエストラインから取り出して検証できるのは HTTP/1.0 以降の構造があるからであり、HTTP/0.9 のリクエストは `SplitN(line, " ", 3)` で 3 要素に分割できずパース失敗する。

### ヘッダ順序に依存してはいけない

HTTP 仕様はヘッダの出現順序を定めていない（一部例外を除く）。本実装の `ParseRequest` は `for` ループでヘッダを 1 行ずつ読んでマップに格納するため、順序には依存しない設計になっている。実際の `curl` は `Host` を最初のヘッダに置くが、クライアントが `Connection` を `Host` より前に送っても正しく動く。パース後に `req.Headers["host"]` でアクセスすれば、何番目に届いたヘッダかを意識せずに値を取り出せる。

### ボディ読み取りを実装していない理由

本実装の `ParseRequest` はヘッダの空行（`\r\n`）で解析を終了し、ボディを読まない。これは実装を最小に保つための省略であり、`POST` リクエストのボディ処理は実装していない。実際に `POST` を受け取ると `method != "GET"` で 405 が返る（[server.go:46](../servers/http/server.go#L46)）。完全な HTTP サーバーにするには `Content-Length` の値を読んで `io.ReadFull(r, make([]byte, contentLength))` でボディを読む処理が必要になる。

---

## 6. 観察: `tcpdump` で HTTP の生バイトを見る

サーバーを起動した状態で `tcpdump` を使うと、TCP ストリーム上を流れる HTTP のバイト列をそのまま観察できる。

```bash
# ターミナル 1: サーバー起動
cd 07_network/servers/http && go run .

# ターミナル 2: tcpdump で観察（macOS）
sudo tcpdump -i lo0 'tcp port 9003' -A

# ターミナル 3: リクエスト送信
curl http://localhost:9003/
```

`tcpdump` の `-A` オプションは ASCII でペイロードを表示する。出力例：

```
(リクエスト)
GET / HTTP/1.1
Host: localhost:9003
User-Agent: curl/7.88.1
Accept: */*

(レスポンス)
HTTP/1.1 200 OK
Content-Type: text/plain
Content-Length: 14
Connection: close

Hello, world!
```

このテキストがそのままバイト列として TCP ストリームに流れている。`\r` は tcpdump の表示では見えないが、`-X` オプション（16 進表示）に切り替えると `0D 0A` として確認できる。

```bash
sudo tcpdump -i lo0 'tcp port 9003' -X
```

16 進ダンプの中で `0D 0A` を探すと、リクエストライン・各ヘッダ行・空行の区切りが目視で確認できる。これが「HTTP とは何か」を最も直接的に体験する方法だ。

---

## 7. 章末「L5/L6/L7 マッピング表」

| 層 | 役割 | HTTP/1.1 での実装 | コード参照 |
|---|---|---|---|
| L7 アプリケーション | 何をしたいか（メソッド・パス・ステータス・本文の意味） | `GET /`・`200 OK`・`"Hello, world!"` | [server.go:43](../servers/http/server.go#L43) `L7: route` |
| L6 プレゼンテーション | バイト列の表現形式（MIME・文字コード・本文サイズ） | `Content-Type: text/plain`・`Content-Length: 14`・CRLF 区切りテキスト行 | [server.go:56](../servers/http/server.go#L56) `writeResponse` |
| L5 セッション | 会話の管理（どのホスト宛か・接続を継続するか） | `Host: localhost:9003`・`Connection: close` | [http.go:51](../servers/http/http.go#L51) `case "host"` |

`Host` ヘッダが L5 に属する理由は、同一 IP で複数の仮想ホストを運用する際に「どのサービスへのリクエストか」を識別するセッション的な情報だからである。`Connection: close` は「この TCP 接続をこのリクエストの後に閉じる」という接続ライフサイクルの宣言であり、これもセッション管理の一部に相当する。

---

## 8. この先の話題

- **keep-alive と pipelining** — `Connection: keep-alive` で同一 TCP 接続上に複数リクエストを送る仕組みと、その実装コスト（`handle` をループ化・タイムアウト管理）
- **chunked 転送エンコーディング** — `Content-Length` 不明のときにチャンク単位でボディを送る方法（`Transfer-Encoding: chunked`）
- **HTTPS（TLS）** — `net.Conn` を `tls.Conn` に差し替えるだけで HTTP over TLS になる；L6 に暗号化・証明書検証が追加される
- **HTTP/2** — バイナリフレーム・多重化・ヘッダ圧縮（HPACK）；単一 TCP 接続で複数ストリームを並列処理する
- **HTTP/3 (QUIC)** — UDP 上で信頼性・多重化・暗号化を再実装；head-of-line blocking を根本から排除

---

## 9. まとめ / 関連 doc

### まとめ

HTTP/1.1 は TCP バイトストリームの上にテキスト行の構造（リクエストライン・ヘッダ・空行・本文）を定義したプロトコルである。`servers/http/` の実装は `bufio.Reader.ReadString('\n')` でこの構造を手でパースし、`fmt.Sprintf` と `conn.Write` でレスポンスを組み立てる。`net/http` が隠しているのはこのパース処理と `Content-Length` の自動計算、そして持続接続の管理である。L5（Host・Connection）・L6（Content-Type・Content-Length）・L7（メソッド・パス・ステータス）が 1 つのハンドラ関数の中に積み重なっている様子をコードで追えたことが本章の核心である。

### 関連 doc

- `01_concepts.md` — OSI/TCP-IP 対応表と全サーバーの位置づけ
- `02_tcp_basics.md` — TCP バイトストリームの正体（本章の前提知識）
- `05_dns_on_udp.md` — UDP 上で同様の L5/L6/L7 積み重ねを DNS で見る
- `06_custom_protocol.md` — 独自バイナリプロトコル（LPP）での同じ構造
