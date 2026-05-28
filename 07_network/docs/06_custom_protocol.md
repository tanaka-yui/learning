# 06_custom_protocol: TCP 上のフレーミングと独自バイナリプロトコル

LPP（Length-Prefixed Protocol）は TCP のバイトストリームの上に「長さプレフィックス」でフレーム境界を定義した最小の独自バイナリプロトコルである。`servers/lpp/` の実装でフレーミングの本質を手で確認する。

---

## 1. TCP 上でなぜフレーミングが必要か

`02_tcp_basics.md` の実験で確認した通り、TCP は「メッセージ境界を持たないバイトストリーム」である。`conn.Read(buf)` を呼んでも、送信側が 1 回 `Write("hello")` した 5 バイトが確実に 1 回の `Read` で届くとは限らない。

```
送信側:  Write("hello")   → 5 バイト
受信側:  Read → "hel"     → 3 バイト
         Read → "lo"      → 2 バイト   ← 分割して届く可能性がある
```

逆に複数の `Write` がまとめて届くこともある（TCP の Nagle アルゴリズムによるバッファリング）。

```
送信側:  Write("hello")
         Write("world")
受信側:  Read → "helloworld"  ← まとめて届く可能性がある
```

アプリケーション層が「ここからここまでが 1 つのメッセージだ」を決めるための仕組みを**フレーミング**という。フレーミングなしでは、受信側はバイト列の切れ目を判断できない。

---

## 2. 長さプレフィックス / 区切り文字 / 固定長 の比較

フレーミングの主要な 3 方式を比べる。

| 方式 | 最大ペイロード | 解析の複雑さ | バイナリ安全性 | 実例 |
|------|--------------|------------|--------------|------|
| **長さプレフィックス** | フィールド幅次第（4 バイトなら ~4 GiB） | 簡単（ヘッダを固定長で読む → 本体を固定長で読む） | 安全（任意バイトを含むペイロードを扱える） | LPP・gRPC・Redis RESP3・SSH |
| **区切り文字** | 理論上無制限（ただし区切り文字を含むデータは転送不可） | やや複雑（ストリームをスキャンして区切り文字を探す） | 安全でない（ペイロードに区切り文字が含まれるとエラー） | HTTP/1.1 ヘッダ（`\r\n`）・SMTP・POP3 |
| **固定長** | フレームサイズに固定 | 最も簡単（常に N バイト読む） | 安全（任意バイトを含むペイロードを扱える） | イーサネットフレームの一部・一部の制御プロトコル |

LPP が長さプレフィックスを採用しているのは、バイナリペイロードを安全に扱えて、かつ解析が最もシンプルだからである。区切り文字方式（HTTP のヘッダに `\r\n` を使う方式）はテキストプロトコルには馴染みやすいが、バイナリペイロードにはそのまま使えない。

### 各方式の実装コスト比較

区切り文字方式の実装イメージ（`bufio.ReadString`）：

```
TCP ストリーム: "hello\nworld\n"
  → ReadString('\n') → "hello\n" ✓
  → ReadString('\n') → "world\n" ✓

ただし ペイロードに '\n' が含まれると:
TCP ストリーム: "hel\nlo\nworld\n"
  → ReadString('\n') → "hel\n"  ← 誤った分割!
```

長さプレフィックス方式の実装イメージ（`io.ReadFull`）：

```
TCP ストリーム: 00 00 00 05 02 68 65 6C 6C 6F
               ├─Len=5──┤ ├Cmd┤ ├──payload──┤
  → ReadFull(4) → n=5
  → ReadFull(5) → cmd=0x02, payload="hello" ✓
  ペイロードの中身に依存しない → バイナリ安全
```

---

## 3. フレームフォーマット図

LPP の 1 フレームは以下の構造を持つ。

```
オフセット(byte)   0     1     2     3     4     5   ... N+4
                 +-----+-----+-----+-----+-----+-----+-----+
フィールド        |         Len (4)         | Cmd | Payload |
                 +-----+-----+-----+-----+-----+-----+-----+
                 ├────────── 4 バイト ──────┤  1  ├── Len-1 バイト ─┤
```

- **Len (4 バイト、ビッグエンディアン uint32)**: `Cmd` + `Payload` のバイト数（Len 自体は含まない）。最小値は 1（Cmd のみ、Payload なし）。
- **Cmd (1 バイト)**: コマンド識別子。
  - `0x01` = PING（Payload なし、PING で応答）
  - `0x02` = ECHO（Payload をそのまま返す）
  - `0x03` = TIME（現在時刻を nanosec uint64 で返す）
  - `0xFF` = 不明コマンドへの応答
- **Payload (Len-1 バイト)**: コマンド依存の可変長データ。

実際のバイト列の例（`ECHO hello` の場合）：

```
リクエスト:
00 00 00 06   02   68 65 6C 6C 6F
└────────┘    └─   └────────────┘
Len=6 (Cmd1+Payload5)  Cmd=ECHO  "hello"

レスポンス:
00 00 00 06   02   68 65 6C 6C 6F
└────────┘    └─   └────────────┘
Len=6          Cmd=ECHO  "hello"
```

---

## 4. `servers/lpp/` コードウォークスルー

### WriteFrame — フレームを書く

[servers/lpp/frame.go:20](../servers/lpp/frame.go#L20)

```go
func WriteFrame(w io.Writer, cmd byte, payload []byte) error {
    body := make([]byte, 1+len(payload))
    body[0] = cmd                    // ← Cmd バイトを先頭に
    copy(body[1:], payload)          // ← Payload を続ける
    hdr := make([]byte, 4)
    binary.BigEndian.PutUint32(hdr, uint32(len(body)))  // ← Len をビッグエンディアンで
    if _, err := w.Write(hdr); err != nil {
        return err
    }
    _, err := w.Write(body)
    return err
}
```

2 回の `Write` に分けているが（ヘッダ 4 バイト + ボディ N バイト）、`io.Writer` が `net.Conn` の場合は OS がバッファリングするため、通常は 1 つの TCP セグメントに収まる。`bufio.Writer` でラップして `Flush` するとより確実に 1 回の送信にまとめられる。

`WriteFrame` への入力と出力の関係を図示する：

```
WriteFrame(conn, CmdEcho, []byte("hello"))
  ↓
body = [0x02, 'h', 'e', 'l', 'l', 'o']    (len=6)
hdr  = [0x00, 0x00, 0x00, 0x06]            (BigEndian uint32(6))

TCP ストリームへの書き込み:
  Write(hdr):  00 00 00 06
  Write(body): 02 68 65 6C 6C 6F
```

### ReadFrame — フレームを読む

[servers/lpp/frame.go:33](../servers/lpp/frame.go#L33)

```go
func ReadFrame(r io.Reader) (byte, []byte, error) {
    hdr := make([]byte, 4)
    if _, err := io.ReadFull(r, hdr); err != nil {   // ← 必ず 4 バイト読む
        return 0, nil, err
    }
    n := binary.BigEndian.Uint32(hdr)
    if n == 0 {
        return 0, nil, errors.New("empty frame")
    }
    if n > maxFrame {                                 // ← DoS 防止（1 MiB 上限）
        return 0, nil, errors.New("frame too large")
    }
    body := make([]byte, n)
    if _, err := io.ReadFull(r, body); err != nil {  // ← 必ず n バイト読む
        return 0, nil, err
    }
    return body[0], body[1:], nil
}
```

`io.ReadFull` を 2 回使う構造になっている（ヘッダ 4 バイト → ボディ n バイト）。`maxFrame` は 1 MiB に設定されており（[frame.go:16](../servers/lpp/frame.go#L16) `const maxFrame = 1 << 20`）、巨大な Len を送りつける DoS 攻撃からサーバーを守る。

### dispatch — コマンドに応じてレスポンスを生成

[servers/lpp/server.go:49](../servers/lpp/server.go#L49)

```go
func dispatch(cmd byte, payload []byte) (byte, []byte) {
    switch cmd {
    case CmdPing:
        return CmdPing, nil
    case CmdEcho:
        return CmdEcho, payload
    case CmdTime:
        out := make([]byte, 8)
        binary.BigEndian.PutUint64(out, uint64(time.Now().UnixNano()))
        return CmdTime, out
    default:
        return CmdUnknown, nil   // ← 未知コマンドには 0xFF で応答
    }
}
```

`handle` から呼ばれ（[server.go:41](../servers/lpp/server.go#L41)）、コマンドのロジックを単純な switch に集約している。L7 の意味（「何をしたいか」）がこの関数に閉じており、フレームの読み書き（L6）とは分離されている。

### handle — フレームループ

[servers/lpp/server.go:33](../servers/lpp/server.go#L33)

```go
func (s *Server) handle(conn net.Conn) {
    defer conn.Close()
    for {
        cmd, payload, err := ReadFrame(conn)   // ← L6: フレーム読み取り
        if err != nil { return }
        s.log.Info("L6: decoded frame", "cmd", cmd, "payload_len", len(payload))
        respCmd, respPayload := dispatch(cmd, payload)   // ← L7: 意味処理
        s.log.Info("L7: dispatch", "in", cmd, "out", respCmd)
        if err := WriteFrame(conn, respCmd, respPayload); err != nil {
            return
        }
    }
}
```

`for` ループで同一接続上のフレームを繰り返し処理する。TCP 接続が切れると `ReadFrame` が `io.EOF` を返してループを抜ける。HTTP の `handle` が「1 リクエストで return する」のに対し、LPP の `handle` は「接続が切れるまでループする」設計だ。これはフレーム境界を自前で定義しているため、1 本の TCP 接続で複数フレームを連続処理できるからである。

コード内の `L6:` / `L7:` ログコメントは `01_concepts.md` の設計方針を踏まえたものだ。フレームの読み書きが L6 であり、コマンドの解釈が L7 であることをログラインで可視化している。

### cmd/client/main.go:writeFrame — クライアント側の意図的な複製

[servers/lpp/cmd/client/main.go:1](../servers/lpp/cmd/client/main.go#L1)

```go
// writeFrame/readFrame are intentionally copied from ../../frame.go;
// both binaries are package main and cannot import each other.
```

クライアント（`cmd/client/main.go`）のコメント 1 行目がこの複製の理由を説明している。Go では 2 つの `package main` は互いをインポートできないため、`frame.go` の `WriteFrame`/`ReadFrame` をクライアント側に手でコピーした。実際の運用では共通パッケージ（例: `lpp/wire`）に切り出して両方からインポートするのが正しい設計だが、本章では「動くコードで学ぶ」ことを優先して意図的に複製した構造になっている。

クライアントからの実行例：

```bash
cd 07_network/servers/lpp
go run ./cmd/client PING
# 0x01 

go run ./cmd/client ECHO hello
# 0x02 hello

go run ./cmd/client TIME
# TIME 2026-05-28T02:08:00.123456789Z
```

---

## 5. `io.ReadFull` を使う理由

`conn.Read(buf)` は要求したバイト数を必ずしも返さない（`02_tcp_basics.md` 参照）。

```
ReadFrame の仮実装（間違い）:
buf := make([]byte, 4)
n, err := conn.Read(buf)   // n が 4 より小さい可能性がある
```

この間違いを犯すと「Len フィールドの一部しか読めていない」状態で `binary.BigEndian.Uint32` を呼ぶことになり、誤った長さを読み取ってしまう。`io.ReadFull` は内部でループしながら指定バイト数が揃うまで `Read` を繰り返す。

```go
// io.ReadFull の疑似実装（概念）
func ReadFull(r Reader, buf []byte) (int, error) {
    total := 0
    for total < len(buf) {
        n, err := r.Read(buf[total:])
        total += n
        if err != nil {
            return total, err
        }
    }
    return total, nil
}
```

`ReadFrame` が `io.ReadFull` を 2 回呼ぶ（ヘッダ 4 バイト・ボディ n バイト）のは、この「部分読み」問題に対する最もシンプルな解決策である。`io.EOF` が途中で返った場合は `io.ErrUnexpectedEOF` に変換されるため、エラー処理も明確になる。

---

## 6. 落とし穴

### エンディアン（バイトオーダー）

Len フィールドは 4 バイトの uint32 だが、[frame.go:25](../servers/lpp/frame.go#L25) では `binary.BigEndian.PutUint32` を使っている。クライアント側も同じく `binary.BigEndian.PutUint32`（[cmd/client/main.go:28](../servers/lpp/cmd/client/main.go#L28)）を使っており、両側が一致していれば問題ない。しかしプロトコル仕様に「ビッグエンディアン」と明記しないまま実装すると、別言語のクライアントが誤ってリトルエンディアンで実装するリスクがある。実際のプロトコル設計ではバイトオーダーをドキュメントの冒頭に明示するのが慣例だ。

### サイズ上限と DoS 耐性

`ReadFrame` は `n > maxFrame` のチェックで 1 MiB を超えるフレームを拒否する（[frame.go:42](../servers/lpp/frame.go#L42)）。このチェックがなければ、悪意のあるクライアントが `Len = 0x7FFFFFFF`（2 GiB）を送りつけることで `make([]byte, n)` が巨大なメモリを確保し、サーバーを OOM（Out of Memory）で落とせる。実際のサービスでは：
- 最大フレームサイズを設定に外出しして調整可能にする
- 接続ごとに受信量を累積カウントしてタイムアウトと組み合わせる
- `sync.Pool` でバッファを再利用してアロケーション圧力を下げる

### 未知 Cmd の扱い

`dispatch` は未知コマンドに対して `CmdUnknown (0xFF)` で応答する（[server.go:60](../servers/lpp/server.go#L60)）。これはサーバーが「わからなかった」とクライアントに通知する最低限の実装だ。実際のプロトコルでは：
- エラーコードを Payload に含める（`0xFF <error_code> <message>`）
- 接続を切る（不正な Cmd はプロトコル違反として扱う）
- 複数バージョンの Cmd を定義してネゴシエーションする（拡張性のある設計）

のどれかを選ぶ。本実装は `0xFF` を返すだけなので、クライアントはエラーの種類を知ることができない。

### バージョニング

LPP のフレームフォーマットにはバージョンフィールドがない。コマンドの意味を後から変えると既存クライアントが壊れる。実際のプロトコル設計では：
- 先頭にプロトコルバージョン（1 バイト）を設ける
- または新しい Cmd バイトを追加して古い Cmd との後方互換を維持する
- または上位層でネゴシエーション（HTTP の `Upgrade` ヘッダのように）する

本章では学習を優先してバージョニングを省いているが、本番で使うプロトコルには必ず何らかのバージョン管理が必要だ。

---

## 7. 章末「L5/L6/L7 マッピング表」

| 層 | 役割 | LPP での実装 | コード参照 |
|---|---|---|---|
| L7 アプリケーション | 何をしたいか（コマンドの意味） | `PING` = 生存確認、`ECHO` = ペイロードをそのまま返す、`TIME` = 現在時刻取得 | [server.go:49](../servers/lpp/server.go#L49) `dispatch` |
| L6 プレゼンテーション | データ表現（フレームのバイナリエンコード） | `Len(4 バイト, BigEndian uint32)` + `Cmd(1 バイト)` + `Payload` の固定レイアウト | [frame.go:20](../servers/lpp/frame.go#L20) `WriteFrame`、[frame.go:33](../servers/lpp/frame.go#L33) `ReadFrame` |
| L5 セッション | **意図的に持たない**（各リクエストが独立完結） | なし | — |

### L5 を持たない設計の代償

LPP は「1 リクエスト・1 レスポンス」を TCP 接続の上で繰り返すだけで、接続をまたいだ状態を持たない。これは実装をシンプルに保つ設計選択だが、いくつかの機能を実現できなくなる代償がある。

- **多重化の不可能性**: 複数の非同期リクエストを 1 本の接続に乗せるには、どのレスポンスがどのリクエストへの応答かを識別する識別子（DNS の TXID 相当）が必要だ。LPP にはそれがないため、クライアントはリクエストを順番に送って応答を待つしかない（head-of-line blocking）。
- **重複リクエストの検知不能**: 同じ `ECHO hello` が 2 回来ても、サーバーには「これは別々のリクエストか、ネットワーク上の重複か」を区別する手段がない。冪等でない操作（例: 銀行振込）には危険な性質だ。
- **再認証の欠如**: 接続が長寿命になれば「誰が接続しているか」をセッション単位で確認したくなる。L5 がないため、再認証や接続ごとの権限管理の仕組みを別に設ける必要がある。
- **接続の意味の喪失**: TCP 接続が切れて再接続しても、サーバーには「同じクライアントが戻ってきた」と認識する情報がない。HTTP の Cookie・SSH の session ID・TLS のセッション再開（session ticket）はいずれも L5 的な仕組みである。

L5 を持たないことはシンプルさのトレードオフであり、PING/ECHO/TIME のような stateless なコマンドには十分だ。しかし「状態を持つ会話」が必要になった瞬間に、L5 の設計コストが避けられない。

---

## 8. まとめ / 関連 doc / この先の話題

### まとめ

LPP は TCP バイトストリームの上に「4 バイト長さ + 1 バイトコマンド + ペイロード」という最小限のフレーミングを定義した独自バイナリプロトコルである。`frame.go` の `WriteFrame`/`ReadFrame` がフレームの組み立てと解析（L6）を担い、`server.go` の `dispatch` がコマンドの意味（L7）を扱う。フレーミングに `io.ReadFull` を使う理由は TCP の部分読みという L4 の性質に起因する。L5（セッション管理）を意図的に持たない設計はシンプルさを得る代わりに、多重化・重複検知・再認証などの機能を諦めている。

### 関連 doc

- `01_concepts.md` — OSI/TCP-IP 対応表と全サーバーの位置づけ
- `02_tcp_basics.md` — TCP バイトストリームの部分読み実験（本章の前提）
- `04_http_on_tcp.md` — HTTP が区切り文字方式（`\r\n`）でフレーミングする例
- `05_dns_on_udp.md` — UDP ではフレーミング不要・L5 を TXID で実現する例

### この先の話題

- **共有パッケージへの抽出** — `cmd/client/main.go` の複製を避けるために `lpp/wire` パッケージに切り出す設計
- **多重化の追加** — フレームに RequestID フィールドを追加して複数の非同期リクエストを 1 接続で処理する（gRPC が HTTP/2 でやっていることの本質）
- **ストリーミングレスポンス** — 1 リクエストに対して複数フレームを返す設計（サーバーサイドストリーミング）
- **バージョンネゴシエーション** — 接続開始時にサポートするコマンドセットを交渉する仕組み（SSH の version string exchange に似た構造）
