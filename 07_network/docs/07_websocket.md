# 07_websocket: HTTP をアップグレードしてバイナリフレームを流す

WebSocket は「HTTP で始まり、HTTP をやめる」プロトコルである。`servers/websocket/` の自前実装でハンドシェイクからフレームフォーマット、ブロードキャスト hub まで手で確認する。

---

## 1. WebSocket が必要だった経緯

HTTP/1.1 はリクエスト・レスポンスのペアが前提で、**サーバーからクライアントへ自発的にデータを送る手段がない**。2000 年代のチャットアプリはこの制約を「HTTP ポーリング」（数秒ごとにクライアントが `GET /updates` を繰り返す）で回避していたが、空振りリクエストの帯域浪費と遅延が問題だった。次に現れた「long-polling」はサーバーが応答を保留して新着データが来たら返す手法で遅延は改善したが、接続が切れるたびに再接続コストが発生した。2011 年に RFC 6455 として標準化された WebSocket はこの問題を根本解決する。HTTP/1.1 の `Upgrade: websocket` ヘッダで既存の TCP 接続をそのまま引き継ぎ、以降は**全二重のバイナリフレームストリーム**として使う。HTTP サーバーと同じポートを使えるため、ファイアウォールや NAT の問題も回避しやすい。

---

## 2. ハンドシェイク

### HTTP Upgrade の仕組み

WebSocket 接続は通常の HTTP GET リクエストとして始まる。クライアントは以下のヘッダを含むリクエストを送る。

```
GET /ws?room=demo HTTP/1.1
Host: localhost:9005
Upgrade: websocket
Connection: Upgrade
Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==
Sec-WebSocket-Version: 13
```

サーバーが WebSocket を受け入れる場合は `101 Switching Protocols` で応答する。

```
HTTP/1.1 101 Switching Protocols
Upgrade: websocket
Connection: Upgrade
Sec-WebSocket-Accept: s3pPLMBiTxaQ9kYGzzhZRbK+xOo=
```

この応答を受け取った瞬間に、両端ともに「この TCP 接続はもう HTTP ではない。WebSocket フレームを流す専用パイプになった」と認識する。

### Sec-WebSocket-Accept の計算（RFC 6455 §1.3）

`Sec-WebSocket-Accept` はクライアントが送った `Sec-WebSocket-Key` から計算する。手順は 3 ステップである。

```
1. clientKey + magic UUID を文字列結合する
   "dGhlIHNhbXBsZSBub25jZQ==" + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
   → "dGhlIHNhbXBsZSBub25jZQ==258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

2. SHA-1 ハッシュを計算する（20 バイト）
   SHA-1("dGhlIHNhbXBsZSBub25jZQ==258EAFA5-E914-47DA-95CA-C5AB0DC85B11")
   → b3 7a 4f 2c c0 62 4f 16 90 f6 46 06 cf 38 59 45 b2 be c4 ea

3. base64 エンコードする
   → "s3pPLMBiTxaQ9kYGzzhZRbK+xOo="
```

この magic UUID `258EAFA5-E914-47DA-95CA-C5AB0DC85B11` は RFC 6455 で固定されており、全ての WebSocket 実装がこの値を使う。計算が一致しない場合はハンドシェイク失敗（接続を閉じる）となる。

### コード参照: `AcceptKey` 関数

[servers/websocket/handshake.go:17](../servers/websocket/handshake.go#L17) の `AcceptKey` が上記の 3 ステップをそのまま実装している。

```go
func AcceptKey(clientKey string) string {
    h := sha1.New()
    _, _ = h.Write([]byte(clientKey + wsMagicGUID))
    return base64.StdEncoding.EncodeToString(h.Sum(nil))
}
```

`wsMagicGUID` は [handshake.go:13](../servers/websocket/handshake.go#L13) で定数定義されている。`Upgrade` 関数（[handshake.go:25](../servers/websocket/handshake.go#L25)）がこれを呼び出し、`101 Switching Protocols` レスポンスに埋め込んで送信する。

---

## 3. フレームフォーマット図

WebSocket の 1 フレームはビットレベルで定義されたヘッダを持つ。RFC 6455 §5.2 のレイアウトを ASCII で示す。

```
Bit:   0               1               2               3
       0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7
      +-+-+-+-+-------+-+-------------+-------------------------------+
      |F|R|R|R| opcode|M| Payload len |    Extended payload length    |
      |I|S|S|S|  (4)  |A|    (7)      |           (16/64)             |
      |N|V|V|V|       |S|             |   (if payload len==126/127)   |
      | |1|2|3|       |K|             |                               |
      +-+-+-+-+-------+-+-------------+-------------------------------+
      |   Extended payload length (cont)  |  Masking-key (if MASK=1)  |
      +-----------------------------------+---------------------------+
      | Masking-key (cont) 4 bytes total  |         Payload data      |
      +-----------------------------------+ - - - - - - - - - - - - -+
      :                     Payload data (cont)                       :
      + - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - +
```

フィールドの意味を表にまとめる。

| フィールド | ビット幅 | 説明 |
|-----------|---------|------|
| FIN | 1 | 1 = このフレームでメッセージが完結（fragmentationなし） |
| RSV1-3 | 各 1 | 拡張用予約ビット（permessage-deflate 等が使う）。通常 000 |
| opcode | 4 | フレーム種別（後述） |
| MASK | 1 | 1 = マスキングキーあり（クライアント→サーバーは必須） |
| Payload len | 7 | 0-125: そのままペイロード長。126: 次の 2 バイトが長さ。127: 次の 8 バイトが長さ |
| Extended payload length | 0/16/64 | payload len が 126/127 のときのみ存在 |
| Masking-key | 32 | MASK=1 のとき存在。XOR マスキングに使う 4 バイト乱数 |
| Payload data | 可変 | 実データ。MASK=1 ならマスキング済み |

### opcode 一覧

| 値 | 定数名 | 意味 |
|----|--------|------|
| 0x0 | OpCont | 継続フレーム（フラグメント） |
| 0x1 | OpText | UTF-8 テキスト |
| 0x2 | OpBin | バイナリ |
| 0x8 | OpClose | 接続クローズ要求 |
| 0x9 | OpPing | ping（keepalive） |
| 0xA | OpPong | pong（ping への応答） |

コード参照: [frame.go:9-18](../servers/websocket/frame.go#L9)

---

## 4. マスキング仕様

### クライアント→サーバー: 必ずマスクする

RFC 6455 §5.1 は「**クライアントから送るフレームは必ずマスキングしなければならない (MUST)**」と定める。マスキングの手順は単純な XOR である。

```
masking-key = [k0, k1, k2, k3]  (4 バイト乱数)
masked[i]   = payload[i] XOR masking-key[i % 4]
```

復元は同じ XOR を再度適用するだけだ（XOR は自己逆演算）。

```go
// frame.go:56-58 の復元処理
for i := range payload {
    payload[i] ^= mask[i%4]
}
```

### サーバー→クライアント: マスクしない（MUST NOT）

サーバーが送るフレームはマスキングしてはならない。[frame.go:63-85](../servers/websocket/frame.go#L63) の `WriteFrame` は `hdr` の MASK ビットを立てておらず、masking-key フィールドもない。

### なぜマスキングが必要か

マスキングは**透過 HTTP プロキシによるキャッシュポイズニング攻撃を防ぐため**に導入された。マスキングなしでは、悪意のあるサーバーが WebSocket ペイロードに細工した HTTP レスポンスを埋め込み、途中のキャッシュサーバーが「これは HTTP レスポンスだ」と誤認識してキャッシュに保存してしまう可能性がある。クライアントがランダムな 4 バイトマスクで XOR することで、攻撃者はペイロードの内容を予測制御できなくなる。

[frame.go:45-47](../servers/websocket/frame.go#L45) でサーバー側は非マスクフレームを拒否する：

```go
if !masked {
    return 0, nil, errors.New("client frame must be masked")
}
```

---

## 5. `servers/websocket/` コードウォークスルー

### handshake.go: AcceptKey / Upgrade

**AcceptKey** [handshake.go:17](../servers/websocket/handshake.go#L17) — Sec-WebSocket-Accept を計算する純粋関数。引数はクライアントキー文字列、戻り値は base64 文字列。

**Upgrade** [handshake.go:25](../servers/websocket/handshake.go#L25) — `bufio.Reader` から HTTP リクエストを行単位で読み出し、`Sec-WebSocket-Key` ヘッダを抽出する。ヘッダ終端（空行）を検出したら `AcceptKey` を呼び出して `101 Switching Protocols` レスポンスを書き込む。クエリ文字列 `?room=` からルーム名を取り出して返す（[handshake.go:35](../servers/websocket/handshake.go#L35)）。

### frame.go: ReadFrame / WriteFrame

**ReadFrame** [frame.go:23](../servers/websocket/frame.go#L23) — サーバー側フレーム読み取り。

```
1. io.ReadFull(r, hdr[2])  → FIN/opcode/MASK/plen を一括取得
2. plen が 126/127 なら追加バイトを読んで実際の長さを確定
3. MASK ビットが 0 なら即エラー（§5.1 違反）
4. masking-key 4 バイトを読む
5. payload を読んで XOR 復元
```

**WriteFrame** [frame.go:63](../servers/websocket/frame.go#L63) — サーバー側フレーム書き込み。ペイロード長に応じてヘッダを組み立て（FIN=1 固定、MASK=0）、先にヘッダを書いてからペイロードを書く。

### hub.go: Broadcast

**Hub** [hub.go:9](../servers/websocket/hub.go#L9) — ルームごとのメンバーを管理するメモリ内 pub/sub。`rooms` フィールドは `map[string]map[Sender]bool`（ルーム名 → 参加者集合）。

**Join / Leave** [hub.go:24/33](../servers/websocket/hub.go#L24) — 参加・退出時に `sync.Mutex` でロックして `rooms` を更新する。

**Broadcast** [hub.go:42](../servers/websocket/hub.go#L42) — ロックして peers を一時スライスにコピーし、ロックを解放してから各 `Sender.Send` を呼ぶ。「ロック中に Send を呼ばない」設計が重要で、Send がブロックすることでデッドロックが起きる危険を避けている。

```go
func (h *Hub) Broadcast(room string, from Sender, msg []byte) {
    h.mu.Lock()
    peers := make([]Sender, 0, len(h.rooms[room]))
    for c := range h.rooms[room] {
        if c != from {
            peers = append(peers, c)
        }
    }
    h.mu.Unlock()          // ← ロックを先に解放
    for _, c := range peers {
        c.Send(msg)        // ← ロック外で Send
    }
}
```

### server.go: handle

**handle** [server.go:36](../servers/websocket/server.go#L36) は 1 接続の全ライフサイクルを担う。

```
1. Upgrade(conn, r)         → L5: ハンドシェイク + ルーム決定
2. hub.Join(room, c)        → L5: セッション登録
3. go writer goroutine      → c.send チャネルを drain してフレームを書く
4. for ループ (ReadFrame)   → L6: フレーム受信
   - OpText  → hub.Broadcast  L7: チャットメッセージ配信
   - OpPing  → WriteFrame(OpPong)
   - OpClose → WriteFrame(OpClose) + return
5. defer hub.Leave(room, c) → L5: セッション解除
```

`wsConn.Send` [server.go:88](../servers/websocket/server.go#L88) はバックプレッシャを `select default` で実装する。チャネルが満杯のとき（クライアントが遅い）、メッセージをドロップして他のクライアントへの送信をブロックしない。

### cmd/sender/main.go

[servers/websocket/cmd/sender/main.go:14](../servers/websocket/cmd/sender/main.go#L14) — 標準入力から 1 行読んで `writeMaskedText` でマスク済みフレームを手組みして送る。

`writeMaskedText` [cmd/sender/main.go:61](../servers/websocket/cmd/sender/main.go#L61) が教育的なコードで、マスキングの実装を生でみせる。固定マスク `{0xAA, 0xBB, 0xCC, 0xDD}` を使い、フレームヘッダ `0x81`（FIN=1, opcode=Text）+ `0x80|len`（MASK=1）を手作りしている。

### cmd/receiver/main.go

[servers/websocket/cmd/receiver/main.go:16](../servers/websocket/cmd/receiver/main.go#L16) — サーバーからの非マスクフレームを `readUnmaskedFrame` で受け取り、OpText なら標準出力に印字する。

---

## 6. broadcast hub パターンの構造

1 接続に対して「reader goroutine」と「writer goroutine」の 2 goroutine を立てる設計が肝心だ。

```
              ┌─────────────────────────────────────────────┐
              │              WebSocket Server                │
              │                                             │
  Client A    │  reader goroutine A                         │
 ─────────────┼──────────────────────────►                 │
              │   ReadFrame()     │                         │
              │                   │ hub.Broadcast()         │
              │                   ▼                         │
              │              ┌─────────┐                    │
              │              │   Hub   │                    │
              │              │ rooms = │                    │
              │              │ {demo:  │                    │
              │              │  [A,B,C]│                    │
              │              │ }       │                    │
              │              └─────────┘                    │
              │                   │ c.Send(msg)             │
              │                   ▼                         │
              │  writer goroutine B  chan []byte (cap=16)   │
 ─────────────┼◄─────────────────────────────────────────   │
  Client B    │   WriteFrame()                             │
              │                                             │
              │  writer goroutine C  chan []byte (cap=16)   │
 ─────────────┼◄─────────────────────────────────────────   │
  Client C    │   WriteFrame()                             │
              └─────────────────────────────────────────────┘
```

**writer goroutine を分離する理由**は**バックプレッシャの隔離**だ。もし reader goroutine が直接 `WriteFrame` を呼ぶ設計にすると、Client B が遅い（受信バッファが詰まっている）ときに Client A の reader goroutine がブロックし、Client C への配信も止まってしまう。writer goroutine と `send chan []byte` の組み合わせで「Client B が遅くても Client C への送信は続く」を実現している。

`send` チャネルの容量は 16 [server.go:46](../servers/websocket/server.go#L46)。容量を超えた場合は `Send` の `select default` でドロップする。これは「低速クライアントに引きずられてメッセージが際限なく積み上がる」状況を防ぐトレードオフだ。

---

## 7. 実験

### セットアップ: 3 ターミナル

```bash
# ターミナル 0: サーバー起動
cd 07_network/servers/websocket
go run .

# ターミナル 1: receiver その 1
go run ./cmd/receiver -room demo

# ターミナル 2: receiver その 2
go run ./cmd/receiver -room demo

# ターミナル 3: sender
go run ./cmd/sender -room demo
# → "connected, type messages and press enter:" と表示される
# → テキストを入力して Enter → ターミナル 1, 2 に同じテキストが届く
```

sender は自分自身には届かない（`hub.Broadcast` が `from Sender` を除外する: [hub.go:45](../servers/websocket/hub.go#L45)）。

### 別ルームでの分離

```bash
go run ./cmd/receiver -room roomA  # ターミナル 1
go run ./cmd/receiver -room roomB  # ターミナル 2
go run ./cmd/sender   -room roomA  # ターミナル 3 → roomB には届かない
```

### tcpdump でハンドシェイクとフレームの境目を見る

```bash
# macOS
sudo tcpdump -X -i lo0 'tcp port 9005'

# Linux
sudo tcpdump -X -i lo 'tcp port 9005'
```

サーバーを起動して sender を接続すると、tcpdump の出力がテキスト（HTTP Upgrade）からバイナリに切り替わる境界が見える。

```
# ハンドシェイク部分 (ASCII テキスト)
GET /ws?room=demo HTTP/1.1
Upgrade: websocket
...
HTTP/1.1 101 Switching Protocols
...

# 以降はバイナリフレーム
# 0x81 = FIN=1, opcode=Text
# 0x85 = MASK=1, paylen=5
# 0xAA 0xBB 0xCC 0xDD = masking-key
# XOR されたペイロード
```

---

## 8. 落とし穴

### マスキング忘れ

クライアント実装でマスキングを省略すると、サーバーの `ReadFrame` が [frame.go:46](../servers/websocket/frame.go#L46) でエラーを返して接続が即座に切れる。「なぜか接続が切れる」バグとして現れやすい。接続直後に切れる場合はフレームの MASK ビットを疑う。

### ping/pong の未実装

RFC 6455 §5.5.2 は ping/pong を keepalive として規定している。サーバーまたはクライアントが定期的に ping を送り、相手が pong を返すことで「接続が生きている」を確認する。本実装は受信した ping に pong を返す（[server.go:69](../servers/websocket/server.go#L69)）が、自発的に ping を送る機能はない。プロダクション実装では `time.Ticker` で定期 ping を送り、一定時間 pong が来なければ接続を切る処理が必要だ。

### close handshake の省略

WebSocket の正常終了は「close フレームを双方向に交換する」2 フェーズハンドシェイクである。本実装は OpClose を受け取ったら OpClose を返して接続を閉じる（[server.go:71-73](../servers/websocket/server.go#L71)）。しかし TCP 接続が突然切れた場合（クライアントがクラッシュ・ネットワーク障害）は OpClose が届かないため、サーバー側は `ReadFrame` が `io.EOF` または `net.ErrClosed` を返すことで初めて気づく。`defer hub.Leave` があるので孤立したセッションが残ることはないが、タイムアウトを組み合わせないとゾンビ接続が長時間残る可能性がある。

### バックプレッシャとドロップ

[server.go:94-96](../servers/websocket/server.go#L94) の `select default` はチャネルが満杯のときメッセージをサイレントにドロップする。低速クライアントへの配信が滞ってもサーバー全体は動き続けるが、そのクライアントはメッセージを取りこぼす。チャット用途ではこれで十分なことが多いが、「全員に確実に届ける」用途（例: ゲームの状態同期）では接続切断に切り替えるか、メッセージキューを外部化する必要がある。

---

## 9. 章末「L5/L6/L7 マッピング表」

| 層 | 役割 | WebSocket での実装 | コード参照 |
|---|---|---|---|
| L7 アプリケーション | 何をしたいか（メッセージの意味） | チャットメッセージのブロードキャスト、ルーム選択 | [hub.go:42](../servers/websocket/hub.go#L42) `Broadcast`、[server.go:66](../servers/websocket/server.go#L66) |
| L6 プレゼンテーション | データ表現（フレームのビットエンコード） | フレームヘッダのビットパッキング（FIN/opcode/MASK/plen）、XOR マスキング、テキスト(0x1) vs バイナリ(0x2) opcode | [frame.go:23](../servers/websocket/frame.go#L23) `ReadFrame`、[frame.go:63](../servers/websocket/frame.go#L63) `WriteFrame` |
| L5 セッション | 接続の確立と管理 | `Upgrade` ハンドシェイク（101 応答）、ping/pong keepalive、OpClose クローズハンドシェイク、Hub によるルーム単位のセッション管理 | [handshake.go:25](../servers/websocket/handshake.go#L25) `Upgrade`、[hub.go:24](../servers/websocket/hub.go#L24) `Join/Leave`、[server.go:69](../servers/websocket/server.go#L69) pong |

### LPP（前章）との比較

| 観点 | LPP（06_custom_protocol） | WebSocket（本章） |
|------|--------------------------|------------------|
| L5 セッション | 意図的に持たない | Upgrade ハンドシェイク + Hub によるルーム管理 |
| L6 フレーミング | 4 バイト長さ + 1 バイト Cmd（テキスト） | ビットフィールドヘッダ + マスキング（バイナリ） |
| 接続形態 | 1 対 1（request/response ループ） | 1 対多（broadcast hub） |
| フレーム境界 | 固定長プレフィックス | 可変長（plen の値で決まる） |

---

## 10. この先

本実装は WebSocket の骨格（UTF-8 テキストフレームのマスキングなし中継）に絞っている。実際のプロダクションでは以下の拡張が現れる。

- **バイナリフレーム (opcode 0x2)**: 画像・音声・Protocol Buffers などのバイナリデータを扱う。現在の hub は `msg []byte` を素通しするため、opcode の区別が必要になる。
- **permessage-deflate** (RFC 7692): RSV1 ビットを使ってフレームを zlib 圧縮する。大量テキストの帯域を大幅削減できる。gorilla/websocket がサポート例。
- **サブプロトコル** (`Sec-WebSocket-Protocol`): WebSocket の上に乗るアプリ層プロトコルを交渉する仕組み。STOMP、MQTT over WS、GraphQL subscriptions などに使われる。
- **フラグメンテーション**: 大きなメッセージを複数の continuation フレーム（opcode 0x0）に分割する仕組み。本実装は FIN=1 フレームのみを想定している。
- **WebTransport** (IETF draft): HTTP/3 (QUIC) の上に全二重ストリームを実現する次世代プロトコル。低遅延でフレームレベルの再送制御ができる。

---

## 11. まとめ / 関連 doc

**まとめ**

WebSocket は HTTP で始まり HTTP をやめるプロトコルである。`Upgrade: websocket` ハンドシェイクで TCP 接続を引き継ぎ、以降は FIN/opcode/MASK/plen のビットフィールドを持つバイナリフレームで全二重通信を行う。クライアントから送るフレームは必ず 4 バイト masking-key で XOR マスクしなければならない（MUST）。サーバーが送るフレームはマスクしない（MUST NOT）。`servers/websocket/` の実装では hub が L5（ルーム単位のセッション管理）、`ReadFrame`/`WriteFrame` が L6（フレームのビットエンコード）、`hub.Broadcast` が L7（チャットメッセージ配信）を担い、OSI モデルの責務分離がコードに直接現れている。

**関連 doc**

- [02_tcp_basics.md](./02_tcp_basics.md) — TCP バイトストリーム、`io.ReadFull` の必要性
- [04_http_on_tcp.md](./04_http_on_tcp.md) — HTTP/1.1 ヘッダ構造（Upgrade の起点となる HTTP ヘッダ形式）
- [06_custom_protocol.md](./06_custom_protocol.md) — バイナリフレーミングの原理（LPP との対比）
- [08_observability.md](./08_observability.md) — tcpdump でハンドシェイクとバイナリフレームを観察する手順
