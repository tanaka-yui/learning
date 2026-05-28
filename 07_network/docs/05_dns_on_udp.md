# 05_dns_on_udp: UDP の上に DNS を載せる

DNS は UDP データグラムの上にバイナリワイヤフォーマットを定義した名前解決プロトコルである。`servers/dns/` の自前実装でその構造を手で確認する。

---

## 1. DNS メッセージフォーマット

DNS メッセージはリクエスト・レスポンスともに同じ構造を持つ。

```
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
|                      ID (16)                    |  ← TXID: トランザクション識別子
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
|QR|   Opcode  |AA|TC|RD|RA|   Z    |   RCODE    |  ← フラグ (16)
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
|                    QDCOUNT (16)                 |  ← Question 数
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
|                    ANCOUNT (16)                 |  ← Answer 数
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
|                    NSCOUNT (16)                 |  ← Authority 数
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
|                    ARCOUNT (16)                 |  ← Additional 数
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
```

ヘッダは固定 12 バイト。その後に QDCOUNT 個の Question セクションが続く。

```
Question セクション（1 クエリ分）:
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
|                     QNAME                       |  ← ラベル長エンコード（可変長）
|                      ...                        |
|           0x00 (終端)                            |
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
|                     QTYPE (16)                  |  ← 1 = A レコード
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
|                     QCLASS (16)                 |  ← 1 = IN (インターネット)
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
```

Answer セクション（レスポンスのみ）:
```
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
|                     NAME                        |  ← QNAME と同じ（またはポインタ）
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
|                     TYPE (16)                   |  ← 1 = A
|                     CLASS (16)                  |  ← 1 = IN
|                     TTL (32)                    |  ← キャッシュ有効秒数
|                     RDLENGTH (16)               |  ← RDATA のバイト数 (A なら 4)
|                     RDATA                       |  ← IPv4 アドレス 4 バイト
+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
```

すべてのマルチバイトフィールドはビッグエンディアン（ネットワークバイトオーダー）で格納される。

### フラグフィールドのビットレイアウト

Flags（2 バイト = 16 ビット）は複数のフィールドをビットパックしている：

```
ビット  15  14  13  12  11  10   9   8   7   6   5   4   3   2   1   0
       +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
       |QR|  Opcode   |AA|TC|RD|RA|   Z      |   RCODE  |
       +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
```

- **QR (1)**: 0 = クエリ、1 = レスポンス
- **Opcode (4)**: クエリ種別（0 = 標準クエリ）
- **AA (1)**: Authoritative Answer（権威応答）。本実装は常に 1 を立てる
- **TC (1)**: Truncated（512 バイト超えで切り詰め）
- **RD (1)**: Recursion Desired（クライアントが再帰を要求）
- **RA (1)**: Recursion Available（サーバーが再帰対応）
- **RCODE (4)**: 応答コード。0 = NOERROR、3 = NXDOMAIN

本実装の定数（[server.go:16](../servers/dns/server.go#L16)）：

```go
const (
    flagsNoError  uint16 = 0x8400 // QR=1 AA=1 RCODE=0 (NOERROR)
    flagsNXDomain uint16 = 0x8403 // QR=1 AA=1 RCODE=3 (NXDOMAIN)
)
```

`0x8400` を 2 進数で見ると `1000 0100 0000 0000`：QR=1（bit 15）・AA=1（bit 10）・それ以外 0。

---

## 2. QNAME のラベル長プレフィックスと名前圧縮（ポインタ）

### ラベル長エンコード

`example.com` をワイヤ上に表現すると：

```
07 65 78 61 6D 70 6C 65   03 63 6F 6D   00
│  └───── "example" ───┘  │  └─"com"┘  └─ 終端 (0x00)
└─ 長さ 7                  └─ 長さ 3
```

ドットで区切られた各ラベルの前に 1 バイトの長さを置き、最後に `0x00` で終端する。ラベルの最大長は 63 バイト（上位 2 ビットが `0b00`）。

### 名前圧縮（ポインタ）

同一メッセージ内に同じドメイン名が複数回登場する場合、2 バイトのポインタで先行箇所を参照できる。

```
0xC0 XX
│    └─ メッセージ先頭からのオフセット（バイト）
└─ 上位 2 ビット = 0b11 でポインタと識別
```

例: `0xC0 0C` → メッセージ先頭 +12 バイト目を参照（Question セクションの QNAME と同じ位置）。本実装はデコード側でポインタを拒否し（エンコード側では Answer の NAME を完全展開で書く）、動作を単純化している（[wire.go:64](../servers/dns/wire.go#L64) `if n&0xC0 == 0xC0 → error`）。

---

## 3. `servers/dns/` コードウォークスルー

### ParseHeader — 12 バイトのヘッダを読む

[servers/dns/wire.go:22](../servers/dns/wire.go#L22)

```go
func ParseHeader(b []byte) (Header, error) {
    if len(b) < 12 {
        return Header{}, errors.New("short header")
    }
    return Header{
        ID:      binary.BigEndian.Uint16(b[0:2]),   // ← TXID
        Flags:   binary.BigEndian.Uint16(b[2:4]),   // ← QR/Opcode/AA/TC/RD/RA/RCODE
        QDCount: binary.BigEndian.Uint16(b[4:6]),
        ANCount: binary.BigEndian.Uint16(b[6:8]),
        NSCount: binary.BigEndian.Uint16(b[8:10]),
        ARCount: binary.BigEndian.Uint16(b[10:12]),
    }, nil
}
```

`binary.BigEndian.Uint16` でビッグエンディアン 2 バイトを uint16 に変換する。フィールド位置はバイトオフセットで固定されており、DNS ヘッダがなぜ 12 バイトなのかはこの 6 × uint16 の構造から明らかだ。

### DecodeQNAME — ラベル長エンコードを文字列に戻す

[servers/dns/wire.go:50](../servers/dns/wire.go#L50)

```go
func DecodeQNAME(r io.ByteReader) (string, error) {
    var parts []string
    for {
        n, err := r.ReadByte()   // ① 1 バイト読む（ラベル長）
        if err != nil { return "", err }
        if n == 0 { return strings.Join(parts, "."), nil }   // ② 終端 0x00
        if n&0xC0 == 0xC0 {
            return "", errors.New("compressed pointer not supported in decoder")
        }
        if n > 63 { return "", errors.New("invalid label length") }
        buf := make([]byte, n)
        for i := range buf {
            b, err := r.ReadByte()   // ③ ラベル本体を n バイト読む
            ...
            buf[i] = b
        }
        parts = append(parts, string(buf))   // ④ ラベルを追加
    }
}
```

ラベル長 → ラベル本体 → ラベル長 → … → 0x00 という繰り返しを `io.ByteReader` で 1 バイトずつ読む。`io.ByteReader` を引数に取ることで `bytes.Reader` や `bufio.Reader` どちらでも使えるインターフェイスになっている。

`handle` では `bytes.NewReader(msg[12:])` を渡している（[server.go:61](../servers/dns/server.go#L61)）。ヘッダ 12 バイトをスキップして Question セクションから読み始めるためのスライスである。`bytes.Reader` は `io.ByteReader` インターフェイスを満たすため、そのまま `DecodeQNAME` に渡せる。

### EncodeQNAME — 文字列をラベル長エンコードに変換

[servers/dns/wire.go:82](../servers/dns/wire.go#L82)

```go
func EncodeQNAME(name string) []byte {
    var out []byte
    for _, label := range strings.Split(name, ".") {
        if label == "" { continue }
        out = append(out, byte(len(label)))      // ← ラベル長
        out = append(out, []byte(label)...)       // ← ラベル本体
    }
    out = append(out, 0)   // ← 終端 0x00
    return out
}
```

`"example.local"` に適用すると：

```
"example.local"
  → ["example", "local"]
  → 07 65 78 61 6D 70 6C 65  05 6C 6F 63 61 6C  00
```

### EncodeAnswer — Answer レコードをバイナリで組み立てる

[servers/dns/wire.go:97](../servers/dns/wire.go#L97)

```go
func EncodeAnswer(name string, ttl uint32, ip net.IP) []byte {
    var out []byte
    out = append(out, EncodeQNAME(name)...)   // NAME（ラベル長エンコード）
    t := make([]byte, 10)
    binary.BigEndian.PutUint16(t[0:2], 1)    // TYPE = A
    binary.BigEndian.PutUint16(t[2:4], 1)    // CLASS = IN
    binary.BigEndian.PutUint32(t[4:8], ttl)  // TTL
    binary.BigEndian.PutUint16(t[8:10], 4)   // RDLENGTH = 4 バイト
    out = append(out, t...)
    out = append(out, ip.To4()...)           // RDATA = 4 バイト IPv4
    return out
}
```

### handle — リクエストを受けてレスポンスを返す

[servers/dns/server.go:49](../servers/dns/server.go#L49)

```go
func (s *Server) handle(msg []byte) ([]byte, error) {
    h, err := ParseHeader(msg[:12])
    ...
    s.log.Info("L5: parsed TXID", "txid", h.ID)   // ← L5: セッション識別
    ...
    name, err := DecodeQNAME(r)
    s.log.Info("L6: decoded QNAME", "name", name) // ← L6: プレゼンテーション
    ...
    qtype := binary.BigEndian.Uint16(tc[0:2])
    if qtype != 1 || qclass != 1 {
        return nil, errors.New("only A/IN supported")
    }
    ip, ok := s.zone[name]
    if !ok {
        s.log.Info("L7: NXDOMAIN", "name", name)  // ← L7: アプリ判定
        ...
    }
    s.log.Info("L7: answering A record", "name", name, "ip", ip.String())
    ...
}
```

ログのコメントが 3 層の仕事を明示している。`ParseHeader` で TXID を取り出すのがセッション層（L5）、`DecodeQNAME` でバイナリを文字列に変換するのがプレゼンテーション層（L6）、ゾーン辞書で A レコードを探すのがアプリケーション層（L7）である。

### ゾーン辞書の定義

[servers/dns/main.go](../servers/dns/main.go) でサーバーの静的ゾーンを定義している（実際の行番号は main.go を参照）。`zone` は `map[string]net.IP` 型で、ホスト名 → IPv4 アドレスの直引きテーブルだ：

```go
zone := map[string]net.IP{
    "example.local": net.ParseIP("10.0.0.1").To4(),
    // 必要に応じてエントリを追加できる
}
```

`s.zone[name]` でマップを引き（[server.go:77](../servers/dns/server.go#L77)）、存在しなければ NXDOMAIN、存在すれば A レコードを返す。この「ゾーン辞書引き」が L7 の処理であり、udp-echo の「受け取ったバイトを返す」との本質的な差分である。

---

## 4. 実験: `dig` と `tcpdump` で観察する

### dig で A レコードを問い合わせる

```bash
# ターミナル 1: サーバー起動
cd 07_network/servers/dns && go run .

# ターミナル 2: A レコードを問い合わせる
dig @127.0.0.1 -p 5353 example.local A +short
```

期待される出力：

```
10.0.0.1
```

NXDOMAIN を確認する場合：

```bash
dig @127.0.0.1 -p 5353 unknown.local A
# ;; ->>HEADER<<- opcode: QUERY, status: NXDOMAIN, id: ...
```

### tcpdump でワイヤを覗く

```bash
# macOS
sudo tcpdump -X -i lo0 'udp port 5353'

# Linux
sudo tcpdump -X -i lo 'udp port 5353'
```

`dig @127.0.0.1 -p 5353 example.local A` を実行したときの tcpdump 出力イメージ：

```
(クエリ)
0x0000:  xx xx 01 00 00 01 00 00  00 00 00 00
         ├─ID─┤ ├Flags─┤ QD=1    AN=0    NS=0
0x000c:  07 65 78 61 6d 70 6c 65  ← "example" (長さ7 + 7バイト)
0x0014:  05 6c 6f 63 61 6c 00     ← "local" (長さ5) + 0x00 終端
0x001b:  00 01 00 01              ← QTYPE=A  QCLASS=IN

(レスポンス)
0x0000:  xx xx 84 00 00 01 00 01  00 00 00 00
         ├─ID─┤ ├Flags─┤ QD=1    AN=1    NS=0
...
0x0024:  00 00 00 3c 00 04        ← TTL=60  RDLENGTH=4
0x002a:  0a 00 00 01              ← 10.0.0.1
```

TXID（ID フィールド）がクエリとレスポンスで一致していることを確認する。これが L5 の「会話対応」の実体である。

---

## 5. 対応範囲を明示

本実装は意図的に最小限のサブセットしか実装していない。

| 機能 | 本実装 | 本格的な DNS サーバー |
|------|--------|----------------------|
| レコードタイプ | A（IN クラスのみ） | A/AAAA/MX/CNAME/NS 等 |
| 再帰解決 | なし（権威専用） | `RD` ビット対応・上位リゾルバへの転送 |
| 名前圧縮デコード | 拒否（エラー） | RFC 1035 §4.1.4 準拠 |
| 名前圧縮エンコード | なし（完全展開） | 長いレスポンスでは必要 |
| EDNS0 / 大パケット | なし | RFC 6891 で UDP 4096 バイトまで拡張 |
| TCP フォールバック | なし | 512 バイト超過時は TC ビットを立てて再送要求 |
| DNSSEC | なし | RRSIG/DNSKEY 等 |

これらの制限は学習目的として意図的なものである。A/IN 固定・再帰なし・圧縮はエンコード側のみ、という制約の中で「DNS のワイヤフォーマット解析が何をしているか」を最もクリアに理解できる実装になっている。

---

## 6. 落とし穴

### バイトオーダー（エンディアン）

DNS のすべてのマルチバイト整数フィールドはビッグエンディアン（ネットワークバイトオーダー）である。x86 系の CPU はリトルエンディアンのため、`uint16` を直接メモリにコピーすると上位バイトと下位バイトが入れ替わって壊れる。[wire.go:27](../servers/dns/wire.go#L27) の `binary.BigEndian.Uint16` は常に上位バイト先読みでフィールドを解釈する。`encoding/binary` を使わずに `*(*uint16)(unsafe.Pointer(&b[0]))` のような cast を書くとアーキテクチャ依存の問題が生じる。

### TTL の使いどころ

Answer セクションの TTL（[wire.go:103](../servers/dns/wire.go#L103)）はクライアントが「このレコードを何秒キャッシュしてよいか」を示す値である。本実装は 60 秒で固定しているが（`EncodeAnswer(name, 60, ip)`）、実際の DNS サーバーではゾーンファイルの TTL 値を使う。TTL=0 はキャッシュ禁止を意味し、ヘルスチェックや動的 IP の用途で使われる。

### Serve ループ — UDP と TCP で異なるゴルーチン構造

[servers/dns/server.go:30](../servers/dns/server.go#L30)

```go
func (s *Server) Serve() error {
    buf := make([]byte, 512)
    for {
        n, addr, err := s.pc.ReadFrom(buf)   // ← 1 データグラムを受信
        ...
        resp, err := s.handle(buf[:n])       // ← 同一ゴルーチンで処理
        ...
        _, _ = s.pc.WriteTo(resp, addr)      // ← 送信元に返す
    }
}
```

udp-echo（`03_udp_basics.md`）と同じ「単一ゴルーチン・`ReadFrom` ループ」構造だが、`buf[:n]` を `handle` に渡して解析する点が異なる。TCP ベースの http/lpp が「接続ごとにゴルーチン」を使うのとの対比が重要だ：

| | TCP (http/lpp) | UDP (dns) |
|---|---|---|
| 接続概念 | `net.Conn`（`Accept` で得る） | なし（`net.PacketConn` + `ReadFrom`） |
| ゴルーチン | 接続ごとに `go handle(conn)` | 単一ゴルーチンでループ |
| 同時処理 | 各ゴルーチンが並列 | 同時に 1 データグラムのみ |
| ステート | 接続ごとのバッファを持てる | バッファは共有（1 本のループ） |

DNS の単一ゴルーチンが成立するのは `handle` が純粋関数（同一入力に同一出力・副作用なし）だからである。ゾーン辞書は読み取り専用のため並行アクセスが安全だ。

### TXID マッチの重要性

[server.go:57](../servers/dns/server.go#L57) で `h.ID` を取り出しているのは、レスポンスの ID フィールドをリクエストと一致させるためだ（[server.go:80](../servers/dns/server.go#L80) `Header{ID: h.ID, ...}`）。UDP は接続がないため、クライアントは TXID が一致するレスポンスだけを自分のクエリへの回答と認識する。TXID を使わない（常に 0 にする等）と、複数の同時クエリで混乱が生じる。TXID はランダム化することが推奨されており（Kaminsky 攻撃への対策）、本実装はクライアントから受け取った TXID をそのまま返すだけで、ランダム化は行わない。

### UDP サイズ上限と TCP フォールバック

UDP ペイロードの古典的上限は 512 バイトである（RFC 1035）。レスポンスが 512 バイトを超える場合、サーバーは TC（Truncated）ビットを立てて切り詰めたレスポンスを返し、クライアントは同じクエリを TCP で再試行することになっている。本実装はバッファを 512 バイトに固定しており（[server.go:31](../servers/dns/server.go#L31)）、大きなレスポンスは返せない。EDNS0（RFC 6891）では `OPT` レコードで UDP バッファサイズを最大 4096 バイトまで交渉できるが、本実装ではサポートしていない。

---

## 7. 章末「L5/L6/L7 マッピング表」

| 層 | 役割 | DNS での実装 | コード参照 |
|---|---|---|---|
| L7 アプリケーション | 何をしたいか（名前解決の要求・A レコード応答・NXDOMAIN 返却） | `example.local → 10.0.0.1` の A レコード検索、ゾーン辞書引き | [server.go:85](../servers/dns/server.go#L85) `L7: answering A record` |
| L6 プレゼンテーション | データ表現（12 バイトヘッダのビットパッキング・ラベル長エンコード・ビッグエンディアン整数） | `ParseHeader` の `binary.BigEndian.Uint16`、`DecodeQNAME` のラベル長読み取り、`EncodeAnswer` のバイナリ組み立て | [wire.go:22](../servers/dns/wire.go#L22) `ParseHeader`、[wire.go:50](../servers/dns/wire.go#L50) `DecodeQNAME` |
| L5 セッション | 会話の管理（どのクエリへの応答かの対応づけ） | TXID（`h.ID`）をリクエストからコピーしてレスポンスに埋め込む、QR ビットで「これは応答だ」と示す | [server.go:57](../servers/dns/server.go#L57) `L5: parsed TXID` |

DNS が UDP を使うにもかかわらず L5（セッション）を持てるのは、TXID という 16 ビットの識別子がアプリケーション層でセッション的な対応づけを実現しているからである。TCP の接続（`net.Conn`）がセッションを保証するのとは対照的に、UDP では「TXID が一致する応答は自分のクエリへの回答である」という約束をアプリが実装している。

---

## 8. この先の話題

- **再帰解決** — NXDOMAIN の代わりに上位 DNS サーバー（ルートサーバー → TLD → 権威サーバー）に問い合わせをリレーする仕組み
- **TCP フォールバック** — TC ビットを立てて 512 バイト超えを通知し、クライアントに TCP 再試行を促す実装
- **DNSSEC** — RRSIG・DNSKEY・DS レコードで応答の真正性を暗号署名で保証する仕組み

---

## 9. まとめ / 関連 doc

### まとめ

DNS は 12 バイトの固定ヘッダ・ラベル長エンコードされた QNAME・Type/Class/TTL/RDATA のバイナリワイヤフォーマットを UDP データグラムの上で送受信するプロトコルである。`servers/dns/wire.go` はこのフォーマットを `binary.BigEndian` で手書きエンコード・デコードしており、`server.go` は TXID を L5 として、QNAME デコードを L6 として、A レコード応答を L7 として明示的にログに残している。UDP に接続がないにもかかわらず L5（セッション）を実現できるのは TXID という識別子を使っているからであり、これが udp-echo との本質的な差分である。

### 関連 doc

- `01_concepts.md` — OSI/TCP-IP 対応表と全サーバーの位置づけ
- `03_udp_basics.md` — UDP データグラムの正体（本章の前提知識）
- `04_http_on_tcp.md` — TCP 上での L5/L6/L7 の実装（比較対象）
- `06_custom_protocol.md` — 独自バイナリプロトコル（LPP）での同じ構造
