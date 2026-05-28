# 07_network 設計書

ネットワーク学習章。Go の `net` パッケージで byte ストリームを直接扱い、TCP/UDP の上に HTTP/DNS/独自プロトコル/WebSocket を「足し算」で組み立てて見せる。第三者ライブラリは使わず stdlib only。

## 学習動線

「下から積み上げ」順。素の TCP/UDP の byte を先に体験し、その上に乗るプロトコルを順に重ねる。

```
            ┌─ HTTP ─────────── (servers/http)
            │
TCP ────────┼─ 独自プロトコル ── (servers/lpp)
            │
            └─ WebSocket ────── (servers/websocket)
                  ↑ HTTP Upgrade で始まり TCP に居座る

UDP ────────┬─ 素の byte ────── (servers/udp-echo)
            └─ DNS ──────────── (servers/dns)

(素の TCP: servers/tcp-echo)
```

## ディレクトリ構成

```
07_network/
├── README.md
├── docker-compose.yml
├── Makefile
├── go.work
├── docs/
│   ├── 01_concepts.md             # OSI/TCP-IP、L4〜L7の位置づけ
│   ├── 02_tcp_basics.md           # TCP=byteストリーム、3-way、観察
│   ├── 03_udp_basics.md           # UDP=データグラム、TCPとの対比
│   ├── 04_http_on_tcp.md          # HTTP/1.1 = TCP上のテキストプロトコル
│   ├── 05_dns_on_udp.md           # DNS = UDP上のバイナリプロトコル
│   ├── 06_custom_protocol.md      # 長さプレフィックス自作プロトコル
│   ├── 07_websocket.md            # HTTP Upgrade → 自前フレーミング + ハブ中継
│   └── 08_observability.md        # tcpdump/ss/lsof でレイヤーを観る
└── servers/
    ├── tcp-echo/      go.mod
    ├── udp-echo/      go.mod
    ├── http/          go.mod
    ├── dns/           go.mod
    ├── lpp/           go.mod  (cmd/client 同梱)
    └── websocket/     go.mod  (cmd/sender, cmd/receiver 同梱)
```

## 設計の柱

- **stdlib only**: `net` / `net/http` / `encoding/binary` / `crypto/sha1` / `encoding/base64` / `log/slog` で完結
- **Go バージョン**: 全モジュール `go 1.26`、Dockerfile は `golang:1.26-alpine` で統一
- **各サーバーは独立 Go モジュール**: `go.work` で束ねる（06章と同じ）
- **観察ツールは標準 UNIX ツール**: `tcpdump` / `ss` / `lsof` / `nc` / `curl` / `dig` / `xxd`
- **1 ポート 1 サーバー**: DNS は 53/udp 衝突回避のためホスト側 `5353/udp` にマップ
- **L5/L6/L7 を可視化**: コードコメント・slog ログ・章末マッピング表の3カ所で繰り返し示す

## サーバー設計

### 1. `servers/tcp-echo/` — 素の TCP

| 項目 | 内容 |
|---|---|
| 目的 | TCP=byte ストリーム。「メッセージ境界が無い」ことを体験 |
| 公開仕様 | `:9001/tcp`、受信 byte をそのまま返す |
| 主要 API | `net.Listen("tcp")` → `Accept` ループ → goroutine 1 本/conn → `conn.Read(buf)` + `conn.Write` |
| 設計上の工夫 | `bufio.Scanner` を**あえて使わず** `conn.Read` の戻り値を観察できる作り |
| 観察 | `nc localhost 9001` で長文送信 → 複数 `Read` 分割を log で確認、`tcpdump -i lo0 'tcp port 9001'` |
| L5/L6/L7 | **該当なし**（純粋に L4 のみ。対比として明示） |

### 2. `servers/udp-echo/` — 素の UDP

| 項目 | 内容 |
|---|---|
| 目的 | UDP=データグラム、「境界保持・配送無保証」を体験 |
| 公開仕様 | `:9002/udp`、受信データグラムをそのまま `WriteTo` で返す |
| 主要 API | `net.ListenPacket("udp")` → 単一 goroutine の `ReadFrom` ループ |
| 観察 | `nc -u localhost 9002`、`tcpdump -i lo0 'udp port 9002'`、1 メッセージ=1 パケット |
| L5/L6/L7 | **該当なし**（純粋に L4 のみ） |

### 3. `servers/http/` — TCP 上の HTTP/1.1（自前パース）

| 項目 | 内容 |
|---|---|
| 目的 | 「HTTP は結局 TCP 上のテキスト」を体感。`net/http` は使わない |
| 公開仕様 | `:9003/tcp`、`GET /` のみ、`200 OK` + `Content-Type: text/plain` で `Hello, world!\n` を返す。それ以外のパス/メソッドは `404` / `405` |
| 主要 API | `net.Listen("tcp")` → `bufio.Reader.ReadString('\n')` で行読み → `Content-Length` 解釈 → 手書きレスポンス |
| 比較 | `docs/04` 内に `net/http` 等価実装（30 行ほど）を並べる |
| 観察 | `curl -v localhost:9003`、`nc localhost 9003` で生 HTTP 手打ち |
| L5（セッション） | `Host` ヘッダの解釈、`Connection: close` の扱い |
| L6（プレゼンテーション） | `Content-Type: text/plain`、`Content-Length` |
| L7（アプリケーション） | メソッド / URL / ステータスコード / ボディの意味 |

### 4. `servers/dns/` — UDP 上の DNS（自前パース）

| 項目 | 内容 |
|---|---|
| 目的 | バイナリプロトコルの「ビット単位の解釈」と「名前圧縮」。`net.LookupHost` は使わない |
| 公開仕様 | `:5353/udp`、A/IN のクエリのみ、固定マップから応答 |
| 主要 API | `net.ListenPacket("udp")` → 12 byte ヘッダ + QNAME（ラベル長+ラベル、末尾 0）を `encoding/binary` でパース |
| 対応範囲（明示） | 単一クエリ / A/IN 固定 / 再帰なし / 名前圧縮は**書き込み側のみ実装**、読み込み側は非圧縮 QNAME 前提でエラー扱い |
| 観察 | `dig @localhost -p 5353 example.local`、`tcpdump -X -i lo0 'udp port 5353'` |
| L5（セッション） | TXID によるリクエスト〜レスポンス対応、QR ビット |
| L6（プレゼンテーション） | 12 byte ヘッダのビット配置、QNAME のラベル長プレフィックス、名前圧縮、バイトオーダー |
| L7（アプリケーション） | 「example.com の A レコードが欲しい」というクエリ意味 |

### 5. `servers/lpp/` — TCP 上の長さプレフィックス独自プロトコル

- **目的**: TCP にアプリ側で境界を引く方法。バイナリプロトコル設計の基本
- **公開仕様**: `:9004/tcp`、`BigEndian` 統一、フレーム形式:

```
+--------+----------+-----------------+
| Len(4) | Cmd(1)   | Payload(Len-1)  |
+--------+----------+-----------------+
```

- **コマンド**:
  - `0x01 PING` — リクエスト: payload 空。レスポンス: `0x01` + 空 payload（pong 相当）
  - `0x02 ECHO` — リクエスト: 任意 byte。レスポンス: 同 cmd + 同 payload
  - `0x03 TIME` — リクエスト: payload 空。レスポンス: `0x03` + 8 byte（`time.Now().UnixNano()` の BigEndian）
  - 未知 cmd: `0xFF` + 空 payload を返す
- **主要 API**: `io.ReadFull` で 4 byte 長 → 本体 → コマンド dispatch → 同形式で応答
- **付属**: `servers/lpp/cmd/client/main.go` で `go run ./cmd/client PING|ECHO <msg>|TIME`
- **観察**: `nc localhost 9004 | xxd` で生 byte、Go クライアントで会話
- **L5（セッション）**: **意図的になし**（毎リクエスト独立）。「L5 を持たない設計の代償」を考察
- **L6（プレゼンテーション）**: Len/Cmd の byte 表現、エンディアン
- **L7（アプリケーション）**: PING/ECHO/TIME のコマンド意味

### 6. `servers/websocket/` — HTTP Upgrade + 自前フレーミング + ハブ中継

| 項目 | 内容 |
|---|---|
| 目的 | 「HTTP で始まり TCP に居座る」プロトコルの構造。フレーミングの再登場 |
| 公開仕様 | `:9005/tcp`、`GET /ws?room=<name>` で部屋参加、同部屋への**テキストメッセージ broadcast** |
| 対応範囲 | テキストフレームのみ、クライアント→サーバーのマスキング解除を仕様通り処理、ping 受信→pong 応答、close 受信→同 close コードで応答後 TCP クローズ |
| スコープ外 | バイナリフレーム / 拡張 / サブプロトコル / permessage-deflate（docs/07 で言及のみ） |
| 主要 API | `net.Listen("tcp")` → HTTP 自前パース → `Upgrade` 検知 → `Sec-WebSocket-Accept` 計算（`crypto/sha1`+マジック UUID+base64）→ 101 応答 → フレームループ |
| Hub | `map[room]map[*conn]chan []byte` + 中央 broadcast goroutine、conn 書き込みは専用 goroutine（バックプレッシャ用） |
| 付属 | `cmd/sender/main.go`（stdin→送信）、`cmd/receiver/main.go`（受信→stdout） |
| 観察 | 2 receiver + 1 sender で broadcast、`tcpdump -X 'tcp port 9005'` で「HTTP テキスト→バイナリフレーム」の切替 |
| L5（セッション） | Upgrade ハンドシェイク、ping/pong、close handshake、room の概念 |
| L6（プレゼンテーション） | フレームヘッダ（FIN/opcode/MASK/length）、マスキング、text vs binary |
| L7（アプリケーション） | チャットメッセージ、broadcast 対象の選択 |

### 各サーバー共通

- `func main()` は薄く、`Server` 構造体で起動・停止。テストから `net.Pipe` や `127.0.0.1:0` で叩ける
- `slog` で `accept / read / write / close` を構造化出力
- **層タグ付きログ**: 主要処理に `slog.Info("L6: decoded QNAME", "name", name)` のように層を明示（学習目的の特例、本番では普通やらない旨を注記）
- グレースフルシャットダウン: `signal.NotifyContext` で SIGTERM 受領 → `Listener.Close()`

### 各サーバー内ファイル構成

```
servers/<name>/
├── go.mod
├── main.go               # flag/signal/起動だけ
├── server.go             # Server構造体 + Start/Shutdown
├── <protocol>.go         # http.go, dns_wire.go, frame.go など
├── <protocol>_test.go    # table-driven テスト
├── server_test.go        # 統合テスト（127.0.0.1:0）
└── Dockerfile
```

lpp / websocket は `cmd/client`、`cmd/sender`、`cmd/receiver` を同モジュール内に持つ。

## ドキュメント章立て

### `docs/01_concepts.md` — レイヤーの位置づけ（〜300 行）

- OSI 7 層 / TCP/IP 4 層 の対応表
- 「TCP/IP 4 層モデルでは L5/L6/L7 が `Application` に潰されているが、実際のプロトコルにはこの 3 層の役割がちゃんと分かれて入っている」を明示
- 本章 6 サーバーがどの層のどの仕事をしているかの一覧表
- 観察ツール早見表（tcpdump/ss/lsof/nc/curl/dig/xxd）

### `docs/02_tcp_basics.md` — TCP の正体（〜400 行）

- TCP=byte ストリーム / 接続志向 / 順序保証 / 再送
- 3-way handshake と FIN/RST を ASCII シーケンス図で
- `servers/tcp-echo/` コードウォークスルー
- 「`Read` の戻り値が送信 byte 数と一致しない」実験手順
- 観察: `ss -tnl`、`tcpdump 'tcp port 9001'`、`lsof -i :9001`
- 落とし穴: 部分読み、`Close` の half-close、goroutine リーク、`SO_REUSEADDR`

### `docs/03_udp_basics.md` — UDP の正体（〜300 行）

- UDP=データグラム / 接続なし / 順序なし / ロス可 / 境界保持 の対比表
- `servers/udp-echo/` コードウォークスルー
- TCP と同じ長文を送って「1 回の `ReadFrom` で全部届く」実験
- MTU / フラグメンテーション / 65507 byte 上限
- 落とし穴: ロス検知不能、順序入れ替わり、再送はアプリ責務

### `docs/04_http_on_tcp.md` — HTTP の正体（〜500 行）

- HTTP/1.1 最小要素: リクエストライン、ヘッダー、空行、本文
- 「`curl -v` のテキスト = 生 byte」の対応
- `servers/http/` コードウォークスルー
- 並列比較: 同等機能の `net/http` 実装
- 落とし穴: `\r\n` 取り違え、Content-Length 無視、ヘッダー大小無関心、HTTP/0.9 との違い
- **章末: L5/L6/L7 マッピング表** + 各層対応のコード行リンク
- この先: keep-alive、chunked、HTTPS、HTTP/2、HTTP/3 への一行ずつ言及

### `docs/05_dns_on_udp.md` — DNS の正体（〜500 行）

- DNS メッセージフォーマット: 12 byte ヘッダ、Question、Answer、Authority、Additional
- QNAME のラベル長プレフィックスと「名前圧縮」（ポインタ）
- `servers/dns/` コードウォークスルー
- 実験: `dig @localhost -p 5353 example.local +short`、`tcpdump -X` の生 byte
- 対応範囲を明示
- 落とし穴: バイトオーダー、TTL、TXID マッチ、UDP サイズ上限と TCP フォールバック
- **章末: L5/L6/L7 マッピング表** + コード行リンク

### `docs/06_custom_protocol.md` — 自作バイナリプロトコル（〜400 行）

- 「TCP 上で何故フレーミングが必要か」（02 章の `Read` 実験への回答）
- 長さプレフィックス vs 区切り文字 vs 固定長 の比較表
- フレームフォーマット図（Len/Cmd/Payload）
- `servers/lpp/` のサーバー & クライアントコードウォークスルー
- `io.ReadFull` を使う理由
- 落とし穴: エンディアン、サイズ上限/DoS 耐性、未知 Cmd の扱い、バージョニング
- **章末: L5/L6/L7 マッピング表**（L5 が意図的に無い設計の考察）

### `docs/07_websocket.md` — HTTP で始まり TCP に居座る（〜600 行）

- WebSocket が必要だった経緯（ポーリング → long-polling → WS）
- ハンドシェイク: `Upgrade` ヘッダ → `Sec-WebSocket-Accept` 計算手順
- フレームフォーマット図（FIN/opcode/MASK/Payload len/extended/masking key/payload）
- マスキング仕様（クライアント→サーバーは必須）と理由
- `servers/websocket/` の server・sender・receiver・hub コードウォークスルー
- broadcast hub パターンの構造（channel + goroutine 分離）
- 実験: 2 receiver + 1 sender、`tcpdump -X` で切り替わり
- 落とし穴: マスキング忘れ、ping/pong、close handshake、バックプレッシャ
- **章末: L5/L6/L7 マッピング表** + コード行リンク
- この先: バイナリフレーム、permessage-deflate、サブプロトコル、WebTransport

### `docs/08_observability.md` — レイヤーを観るツール集（〜300 行）

- ツール早見表（02〜07 章で散らばっている観察コマンドを集約）
- `tcpdump` 基本: `-i lo0`、`-X`、`-A`、フィルタ式
- `ss -tnl` / `ss -unl` / `lsof -i :PORT`
- `nc` / `curl -v` / `dig +short +trace` / `xxd`
- macOS と Linux の `tcpdump` インターフェイス名差分（`lo0` vs `lo`）
- Docker 環境でホスト側からどう観るか

### 横断方針

- コード参照は `servers/.../main.go:42` 形式（06章 `10_patterns_in_code.md` 準拠）
- 絵は ASCII 図、HTTP リクエストや DNS パケットの **16 進ダンプ**は実物を貼る
- 各 doc 末尾に「**まとめ**」「**関連 doc**」「**この先の話題**」を統一配置

## テスト戦略

### パーサ / シリアライザ単体テスト

- table-driven、正常系 + 不正入力（破損 byte / 長さ不一致 / 未知 opcode）
- 例: `dns_wire_test.go` で QNAME のエンコード/デコード round-trip
- 例: `frame_test.go` で `io.ReadFull` が境界をまたいでも正しく読めるか

### サーバー統合テスト

- `net.Listen("tcp", "127.0.0.1:0")` でランダムポート起動 → クライアントから叩く
- UDP も同様に `net.ListenPacket("udp", "127.0.0.1:0")`
- WebSocket hub: 「sender→hub→receiver1+receiver2 に broadcast」を 1 テストで検証
- DNS は `dig` 不要、テスト内で自前 DNS クエリ byte を組み立てて検証

### Race detector

- `go test -race ./servers/...` を CI 想定で前提
- WebSocket hub は特に重要

## Docker Compose

```yaml
services:
  tcp-echo:
    build: ./servers/tcp-echo
    ports: ["9001:9001"]
  udp-echo:
    build: ./servers/udp-echo
    ports: ["9002:9002/udp"]
  http:
    build: ./servers/http
    ports: ["9003:9003"]
  dns:
    build: ./servers/dns
    ports: ["5353:5353/udp"]
  lpp:
    build: ./servers/lpp
    ports: ["9004:9004"]
  websocket:
    build: ./servers/websocket
    ports: ["9005:9005"]
```

- Dockerfile はサーバーごとに最小（multi-stage `golang:1.26-alpine` → `gcr.io/distroless/static`）
- lpp / websocket のクライアント側は**ホストで `go run`**（外からサーバーを叩く体験を優先、コンテナ内 exec はしない）

## Makefile

```makefile
.PHONY: up down logs test test-race demo-tcp demo-udp demo-http demo-dns demo-lpp demo-ws

up:        ; docker compose up -d --build
down:      ; docker compose down
logs:      ; docker compose logs -f
test:      ; go test ./servers/...
test-race: ; go test -race ./servers/...

demo-tcp:  ; echo "hello" | nc -w1 localhost 9001
demo-udp:  ; echo "hello" | nc -u -w1 localhost 9002
demo-http: ; curl -v http://localhost:9003/
demo-dns:  ; dig @localhost -p 5353 example.local +short
demo-lpp:  ; go run ./servers/lpp/cmd/client PING
demo-ws:   ; @echo "run sender/receiver in separate terminals:" && \
             echo "  go run ./servers/websocket/cmd/receiver -room demo" && \
             echo "  go run ./servers/websocket/cmd/sender   -room demo"
```

## README.md（章トップ）

- 学習動線（docs 01〜08 へのリンク）
- クイックスタート（`make up` → `make demo-http` など）
- ポート一覧表
- 環境注意（macOS と Linux の `tcpdump` インターフェイス名差分など）
- 06 章 README の構成に揃える

## 受け入れ条件

このスペックが「実装完了」と言える基準:

1. `make up` で 6 サービスが listening 状態
2. `make demo-tcp` 〜 `make demo-ws` がすべて期待出力を返す
3. `make test-race` が pass
4. `docs/01〜08` が完成し、`04`/`05`/`06`/`07` の章末に「L5/L6/L7 マッピング表」が入っている（`02`/`03` は「該当なし」を明示）
5. `docker compose logs <service>` で L5/L6/L7 を持つサービス（http/dns/lpp/websocket）について各リクエストの層遷移がログから読める
6. README が動線・クイックスタート・ポート一覧を網羅

## スコープ外（明示的にやらないこと）

- **TLS / HTTPS / HTTP/2 / HTTP/3**: `docs/04_http_on_tcp.md` 末尾で「この先」として言及のみ
- **DNS の再帰解決、TCP fallback、DNSSEC**: `docs/05` で言及のみ
- **WebSocket の permessage-deflate、バイナリフレーム、サブプロトコル**: `docs/07` で言及のみ
- **raw socket / IP 層自作**: 学習者に root 権限と OS 依存を強いるため対象外
- **第三者ライブラリ**: stdlib only を貫く（`miekg/dns` や `gorilla/websocket` は使わない）
