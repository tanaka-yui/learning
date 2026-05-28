# 02_tcp_basics: TCP の正体

TCP は「バイトストリームを確実に届ける」ための L4 プロトコルである。tcp-echo サーバーは TCP の最低限の動作を裸で観察できる教材として設計されている。

---

## 1. TCP の正体

### バイトストリーム

TCP はメッセージ境界を持たない。送信側が `Write("hello")` → `Write(" world")` と 2 回呼んでも、受信側では `Read` で `"hello world"` がまとめて届いたり `"hel"` と `"lo world"` に分割されたりする。これは「ストリーム」という言葉の意味そのものであり、アプリケーション層がメッセージ境界を自前で管理しなければならない根本的な理由である。

### 接続志向

TCP は通信を始める前に必ず「接続」を確立する。クライアントとサーバーが SYN / SYN-ACK / ACK の 3 ウェイハンドシェイクを完了して初めてデータを送れる状態になる。接続は `net.Listener.Accept()` が返す `net.Conn` として Go では表現される（[servers/tcp-echo/server.go:21](../servers/tcp-echo/server.go#L21)）。

### 順序保証

TCP はネットワーク上でパケットが順序逆転しても、受信側の TCP スタックが正しい順序に並べ直してアプリに渡す。アプリケーション層からは「常に送った順番通りのバイト列が届く」ように見える。これはシーケンス番号と ACK 番号によって実現されている。

### 再送

パケットが失われた場合、送信側 TCP は一定時間 ACK が来なければ自動で再送する。アプリケーションは再送を意識する必要がない。ただしこの再送はレイテンシの増大と head-of-line blocking（先頭パケットが詰まると後続も止まる）という代償を伴う。

---

## 2. 3-way handshake / FIN・RST

```
    Client                          Server
      |                               |
      |------- SYN (seq=x) --------->|  接続要求
      |                               |
      |<----- SYN-ACK (seq=y,       |  受け入れ可能
      |         ack=x+1) ------------|
      |                               |
      |------- ACK (ack=y+1) ------->|  接続確立
      |                               |
      |======= DATA =================|  データ転送
      |                               |
      |------- FIN ----------------->|  能動的クローズ
      |<------ ACK ------------------|
      |<------ FIN ------------------|
      |------- ACK ----------------->|  TIME_WAIT → CLOSED
      |                               |
```

FIN は「これ以上送るデータはない」という通知であり、相手からの FIN を受け取るまでは受信を継続できる（half-close）。RST はエラー時に接続を即座に切断する手段で、FIN の 4 ウェイ交換を省略する。`tcpdump` で観察すると `[S]`（SYN）、`[S.]`（SYN-ACK）、`[.]`（ACK）、`[F.]`（FIN-ACK）というフラグ表記で確認できる。

---

## 3. servers/tcp-echo/ コードウォークスルー

### NewServer

[servers/tcp-echo/server.go:15](../servers/tcp-echo/server.go#L15)

```go
func NewServer(ln net.Listener) *Server {
    return &Server{ln: ln, log: slog.Default()}
}
```

`net.Listener` を受け取るだけのシンプルなコンストラクタ。リスナーの作成（`net.Listen("tcp", ":9001")`）は [servers/tcp-echo/main.go:17](../servers/tcp-echo/main.go#L17) で行われており、サーバー本体はリスナーを注入してもらう設計になっている。これによってテスト時に `net.Listen("tcp", "127.0.0.1:0")` でランダムポートを使えるようになる。

### Serve

[servers/tcp-echo/server.go:19](../servers/tcp-echo/server.go#L19)

```go
func (s *Server) Serve() error {
    for {
        conn, err := s.ln.Accept()
        if err != nil {
            if errors.Is(err, net.ErrClosed) {
                return nil
            }
            return err
        }
        s.log.Info("accept", "remote", conn.RemoteAddr().String())
        go s.handle(conn)
    }
}
```

`Accept()` はクライアントからの接続が来るまでブロックする。接続が来るたびに `go s.handle(conn)` でゴルーチンを起動するため、複数クライアントを同時に扱える。`net.ErrClosed` は `ln.Close()` を呼んだときに返るエラーで、グレースフルシャットダウンの判定に使う（[servers/tcp-echo/main.go:29](../servers/tcp-echo/main.go#L29) でシグナル受信時に `ln.Close()` を呼んでいる）。

### handle

[servers/tcp-echo/server.go:33](../servers/tcp-echo/server.go#L33)

```go
func (s *Server) handle(conn net.Conn) {
    defer conn.Close()
    buf := make([]byte, 4096)
    for {
        n, err := conn.Read(buf)
        if n > 0 {
            s.log.Info("read", "n", n)
            if _, werr := conn.Write(buf[:n]); werr != nil {
                s.log.Error("write_err", "err", werr.Error())
                return
            }
        }
        if err != nil {
            if !errors.Is(err, io.EOF) {
                s.log.Error("read_err", "err", err.Error())
            }
            return
        }
    }
}
```

**`conn.Read(buf)` を直接使う理由**：tcp-echo は「受け取ったバイトをそのまま返す」だけが仕事であり、メッセージ境界を気にする必要がない。`bufio.Scanner` や `bufio.ReadString` は「改行まで読む」「N バイト読む」といった境界認識を前提とするが、純粋なエコーサーバーにはその概念が不要である。むしろ `Read` の戻り値 `n` が「今回受け取れたバイト数」であることを利用して、そのまま `buf[:n]` を返すのが最も素直な実装になる。

注意点として、`n > 0` と `err != nil` の両方が同時に成立する場合があることが重要だ。`io.EOF` と同時に最後のデータが届くことがある。このコードは `n > 0` を先にチェックしてデータを書いてから `err` を処理するため、そのケースを正しく扱っている。

---

## 4. `Read` の戻り値が送信バイト数と一致しない実験

TCP のストリーム性を実際に観察する実験手順：

```bash
# ターミナル 1: サーバー起動
make demo-tcp
# または
cd servers/tcp-echo && go run .

# ターミナル 2: 長い文字列を送信
python3 -c "print('A' * 5000)" | nc localhost 9001
```

サーバー側のログに `read n=4096` と `read n=904` のように複数行の read ログが出ることがある。これは：

1. `nc` が 5000 バイトを一度に送信しようとしても
2. OS の TCP スタックが複数のセグメントに分割して送信し
3. サーバー側の `conn.Read(buf)` が 4096 バイトのバッファサイズ上限で複数回に分かれて読む

という TCP ストリームの性質を示している。逆に短い文字列であれば 1 回の `Read` で届くことが多い。この「何バイト届くかは `Read` してみないとわからない」という事実が、HTTP や LPP が独自のメッセージ境界（改行や長さプレフィックス）を必要とする根本的な理由である。

---

## 5. 観察コマンド

### ss でリスニング確認

```bash
ss -tnlp | grep 9001
```

`LISTEN` 状態のソケットと PID が表示される。サーバーが起動していれば `0.0.0.0:9001` または `:::9001` が見える。

### tcpdump でパケット観察

```bash
# macOS
sudo tcpdump -i lo0 'tcp port 9001' -X

# Linux
sudo tcpdump -i lo 'tcp port 9001' -X
```

`-X` オプションで 16 進とASCII の両方が表示される。SYN・SYN-ACK・ACK の 3-way handshake、データ転送、FIN のシーケンスがパケット単位で見える。

### lsof でプロセス確認

```bash
lsof -i :9001
```

ポート 9001 を使っているプロセス名・PID・ファイルディスクリプタが表示される。サーバーが二重起動してポートが被っているときの診断に使う。

---

## 6. 落とし穴

### 部分読み（partial read）

`conn.Read(buf)` は `buf` を全部埋めてから返すわけではない。1 バイトしか読めなくても返ってくる。メッセージ境界が必要なプロトコルでは `io.ReadFull` や独自のフレーミングが必須である。tcp-echo はエコーするだけなので部分読みでも正しく動くが、上位プロトコルでは必ず問題になる。

### `Close` の half-close

`conn.Close()` は送受信の両方向を同時に閉じる。RFC 的には FIN を送ってから相手の FIN を待つ半二重クローズ（`CloseWrite`）が正しい場合もあるが、Go の標準的なサーバーコードでは `defer conn.Close()` で両方向を同時に閉じるのが一般的である。相手がまだ書き込んでいる最中に `Close` すると RST が送られることがある。

### goroutine リーク

`go s.handle(conn)` で起動したゴルーチンは、`conn.Read` がエラーを返すか `conn.Close` が呼ばれるまで終了しない。サーバーをシャットダウンするとき `ln.Close()` だけではすでに `Accept` 済みの接続が残る。本実装では `defer conn.Close()` があるため、シグナルを受けて `ln.Close()` → `Serve` が返った後も、既存の接続ハンドラは自然にエラーを受け取って終了する。大規模なサーバーでは `context.Context` を `handle` に渡して明示的にキャンセルすることも多い。

### SO_REUSEADDR

サーバーをクラッシュ再起動すると「address already in use」になることがある。これは前の接続が `TIME_WAIT` 状態で残っているためだ。Go の `net.Listen` はデフォルトで `SO_REUSEADDR` を設定するため通常は問題にならないが、`TIME_WAIT` が何であるかを知っておくと `tcpdump` の解析が楽になる。

---

## 7. L4 のみであることを明示

素の TCP echo サーバーは L4 だけの世界である。L5（セッション管理）・L6（データ符号化）・L7（アプリケーション意味）を一切持たない。受け取ったバイトをそのまま返すだけで、「何を送ったか」にサーバーは無関心だ。後続章では、この同じ TCP 接続の上に HTTP（`04_http.md`）・LPP（`06_lpp.md`）・WebSocket（`07_websocket.md`）がどう積み上がるかを見ていく。それぞれの実装と tcp-echo を比べることで、「L5/L6/L7 を追加するとコードにどんな責務が増えるか」が明確になる。

---

## 8. まとめ / 関連 doc / この先の話題

### まとめ

TCP はバイトストリーム・接続志向・順序保証・再送という 4 つの性質を持つ L4 プロトコルである。tcp-echo はその最小実装であり、`conn.Read(buf)` が任意のバイト数を返すというストリームの現実を直接観察できる。メッセージ境界はアプリ層の責務であり、それが HTTP や LPP のフレーミング設計の出発点になっている。

### 関連 doc

- `01_concepts.md` — OSI/TCP-IP 対応表と全サーバーの位置づけ
- `03_udp_basics.md` — TCP と対比しながら UDP の性質を理解する
- `06_lpp.md` — TCP 上に長さプレフィックスフレームを載せた実装

### この先の話題

- TCP の輻輳制御（Reno / CUBIC / BBR）とパフォーマンス特性
- `SO_KEEPALIVE` によるアイドル接続の死活監視
- `net.Conn` に TLS を重ねる `tls.Server` / `tls.Client` の使い方
