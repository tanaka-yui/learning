# 01_concepts: ネットワーク層の全体像

本章では TCP/UDP という L4 トランスポート層の上に HTTP・DNS・独自バイナリプロトコル・WebSocket がどう積み重なっているかを 6 つのサーバー実装を通して学ぶ。

---

## 1. イントロ

インターネット上で動くアプリケーションは、どれも最終的には TCP か UDP のソケットを通してバイト列を送受信している。HTTP も DNS も WebSocket も、その実態は「TCP または UDP のストリーム／データグラムの上に、独自のルール（フォーマット・手順・意味）を載せたもの」に過ぎない。本章ではその「載せ方」を自前実装で手触りを持って理解することを目的とする。

Go の標準ライブラリには `net/http` パッケージや `net.Resolver` があり、これらを使えばひとまず HTTP サーバーも DNS 参照も書ける。しかし「HTTP リクエストとはどういうバイト列か」「DNS の TXID とは何を意味するか」は隠蔽されてしまう。本章のサーバー群はあえて生の `net.Conn` / `net.PacketConn` から組み上げることで、プロトコルの階層構造を手で確かめられるようにしている。コードを読む際は「このコードは OSI のどの層の仕事をしているか」という視点を持つと全体像が掴みやすい。

たとえば tcp-echo の `handle` 関数（[servers/tcp-echo/server.go:33](../servers/tcp-echo/server.go#L33)）はわずか 20 行で「受け取ったバイトを書き戻す」だけだ。同じ TCP の上に HTTP を載せた http サーバーは、同じループ構造を持ちながら「バイト列から HTTP リクエスト行を切り出す」「ヘッダを解析してルーティングする」「HTTP ステータス行とヘッダを組み立てて書き戻す」という処理が追加される。この差分こそが L6（バイト列の解析）と L7（ルーティングと意味付け）の実装コストを示している。

---

## 2. OSI 7 層 / TCP/IP 4 層 対応表

ネットワークプロトコルの教科書には OSI 7 層モデルと TCP/IP 4 層モデルという 2 つの整理方法が登場する。両者の対応と本章での扱いを以下にまとめる。

| OSI 層 | 名称 | TCP/IP 4 層 | 本章で扱う範囲 |
|--------|------|-------------|--------------|
| L7 | アプリケーション | Application | HTTP・DNS・LPP・WS のアプリ的な意味付け |
| L6 | プレゼンテーション | Application | HTTP ヘッダ解析・DNS ワイヤ符号化・LPP バイナリフレーム・WS フレーム |
| L5 | セッション | Application | HTTP keep-alive・DNS TXID・WS ハンドシェイク／ルーム管理 |
| L4 | トランスポート | Transport | **TCP**（tcp-echo / http / lpp / websocket）/ **UDP**（udp-echo / dns） |
| L3 | ネットワーク | Internet | IP（本章では直接触らない） |
| L2 | データリンク | Network Access | Ethernet 等（本章では直接触らない） |
| L1 | 物理 | Network Access | 物理媒体（本章では直接触らない） |

本章のコードが触るのは L4 〜 L7 の範囲である。L1〜L3 は OS と NIC が担っており、Go のソケット API を使う限り意識する必要はない。ただし L3（IP）のパケットサイズ上限（MTU）は UDP の設計に影響するため、`03_udp_basics.md` で補足する。

OSI 7 層モデルは 1984 年に策定された規格であり、TCP/IP 4 層モデルは ARPANET で実際に発展したインターネットの実装寄りの整理である。現実のプロトコル設計では OSI の 7 層に機械的に対応するのではなく、「セッション」「エンコーディング」「アプリ意味」という概念のいずれかがどこに実装されているかを考えると整理しやすい。本章ではこの 3 概念に注目してコードを読む。

---

## 3. L5/L6/L7 は TCP/IP では「Application」に束ねられている

TCP/IP 4 層モデルは OSI の L5（セッション）・L6（プレゼンテーション）・L7（アプリケーション）を「Application 層」という一枚の棚に押し込んでいる。これは実用上の単純化であって、実際のプロトコルにはこれら 3 層の役割がしっかり分かれて存在する。

3 層の仕事をそれぞれ一言で言うと：

- **L5 セッション**：「どの会話のどのやりとりか」を管理する。TCP の場合は接続（`net.Conn`）そのものがセッションに相当するが、UDP では接続がないため TXID のような識別子でリクエストとレスポンスを対応づける必要がある。
- **L6 プレゼンテーション**：「バイト列をどう解釈するか」を定める。テキスト（HTTP ヘッダ）、バイナリ（DNS ワイヤフォーマット・LPP フレーム・WS フレーム）、エンコーディング（Base64・圧縮）などがここに属する。
- **L7 アプリケーション**：「何をしたいか」の意味を扱う。HTTP なら「/users を GET する」、DNS なら「example.local の A レコードを引く」、LPP なら「現在時刻を問い合わせる」がこれにあたる。

DNS サーバー（`servers/dns/server.go`）はこの 3 層をログメッセージで明示している。`L5: parsed TXID`（[servers/dns/server.go:57](../servers/dns/server.go#L57)）はセッション層の仕事、`L6: decoded QNAME`（[servers/dns/server.go:66](../servers/dns/server.go#L66)）はプレゼンテーション層の仕事、`L7: answering A record`（[servers/dns/server.go:85](../servers/dns/server.go#L85)）はアプリケーション層の仕事である。ひとつのハンドラ関数がこれら 3 層を順番にこなしている様子がコードで追える。TCP/IP の「Application 層」はこれをまとめて呼んでいるだけで、設計上の区別は依然として有効である。

同様に WebSocket サーバーのコメントも層を明示している。`hub.go` の `L5: hub manages session membership per room`（[servers/websocket/hub.go:23](../servers/websocket/hub.go#L23)）、`server.go` の `L6: frame`（[servers/websocket/server.go:63](../servers/websocket/server.go#L63)）、`L7: broadcast`（[servers/websocket/server.go:66](../servers/websocket/server.go#L66)）も同じ構造になっている。コードを読む際にこのコメントを目印にすると、どの処理がどの層に属するかが整理しやすい。

さらに、「L5 を持たない」ことがどういう意味かを tcp-echo と lpp で比較するとわかりやすい。tcp-echo は TCP 接続が張られたら受け取ったバイトを返すだけであり、「この接続が何回目のやりとりをしているか」を記録しない。lpp も同様に接続単位で完結する。一方、WebSocket サーバーの Hub（[servers/websocket/hub.go:24](../servers/websocket/hub.go#L24)）はルームごとに `map[Sender]bool` を管理しており、「どのクライアントが同じルームにいるか」という状態を保持する。これが L5 セッション管理の具体的な実体である。HTTP の `Connection: keep-alive` も「同じ TCP 接続を複数リクエストに再利用する」というセッション的な仕組みであり、接続を使い捨てにする HTTP/1.0 との差がまさに L5 の有無に相当する。

---

## 4. 本章サーバーの位置づけ

6 つのサーバーを L4〜L7 の軸で整理する。

| サーバー | ポート | L4 | L5（セッション） | L6（プレゼンテーション） | L7（アプリ意味） |
|----------|--------|----|-----------------|------------------------|----------------|
| tcp-echo | 9001 | TCP | なし（接続単位で完結） | なし（生バイト折り返し） | なし（エコーのみ） |
| udp-echo | 9002 | UDP | なし（データグラム単位で完結） | なし（生バイト折り返し） | なし（エコーのみ） |
| http | 9003 | TCP | `Connection: keep-alive` による継続接続 | HTTP/1.1 テキストヘッダ解析 | GET/POST ルーティング・レスポンス生成 |
| dns | 5353 | UDP | TXID で要求と応答を 1 対 1 に対応づけ | QNAME ラベル符号化・ワイヤフォーマット | A レコード応答 / NXDOMAIN |
| lpp | 9004 | TCP | なし（接続単位で完結） | 4 バイト長プレフィックス＋コマンドバイト | PING / ECHO / TIME コマンド |
| websocket | 9005 | TCP | HTTP Upgrade ハンドシェイク・ルーム管理 | WS フレーム（opcode・マスク・長さ） | テキストメッセージのルーム内ブロードキャスト |

tcp-echo と udp-echo は L5/L6/L7 を何も持たない。L4 の動作だけを純粋に観察するための最小サーバーとして機能する。残りの 4 つはそれぞれ異なる設計で L5/L6/L7 を積み上げており、比べることで「Application 層の実装とは何をコードに足すことか」が見えてくる。

各サーバーが Go のどの型を中心に実装されているかも比較する上で重要である。

| サーバー | 中心となる Go の型 | 複数クライアントの扱い |
|----------|------------------|----------------------|
| tcp-echo | `net.Conn` | 接続ごとにゴルーチン（`go s.handle(conn)`） |
| udp-echo | `net.PacketConn` | 単一ゴルーチン（`ReadFrom` のループ） |
| http | `net.Conn` + 自前パーサ | 接続ごとにゴルーチン |
| dns | `net.PacketConn` | 単一ゴルーチン（`ReadFrom` のループ） |
| lpp | `net.Conn` | 接続ごとにゴルーチン |
| websocket | `net.Conn` + Hub | 接続ごとにゴルーチン＋Hub がブロードキャスト管理 |

TCP ベースのサーバーが「接続ごとにゴルーチン」を採用するのは、`net.Conn.Read` がブロッキング呼び出しだからである。UDP ベースのサーバーが「単一ゴルーチン」で済むのは、`net.PacketConn.ReadFrom` 一発で「誰から来たか」も分かり、ステートを持つ必要がないからである。この構造の違いが、TCP と UDP の本質的な差異を反映している。

---

## 5. 各サーバーが「自前実装」している箇所

本章のサーバーは Go 標準ライブラリの高レベル API を使わず、あえて低レイヤから実装している。以下にその一覧を示す。

| サーバー | 使わない標準 API | 自前で実装している箇所 |
|----------|----------------|----------------------|
| tcp-echo | — | そもそも実装が最小なので比較対象なし |
| udp-echo | — | 同上 |
| http | `net/http` | リクエスト行・ヘッダ・ボディの手動パース（`servers/http/http.go`） |
| dns | `net.Resolver` | DNS ワイヤフォーマットのエンコード・デコード（`servers/dns/wire.go`） |
| lpp | — | 独自プロトコルなので「標準 API がない」のが自然 |
| websocket | `golang.org/x/net/websocket` 等 | HTTP Upgrade、フレーム読み書き（`servers/websocket/handshake.go`, `frame.go`） |

「自前実装」の意図は、高レベル API が隠している部分を目に見える形にすることである。`net/http` を使えば HTTP リクエストの解析は意識しなくていい。しかし「`\r\n` でヘッダと本文を分ける」「`Content-Length` ヘッダの値に従ってボディを読む」という処理が必要なことは、自前で書いて初めて実感できる。WebSocket の HTTP Upgrade がなぜ HTTP ヘッダで行われるのか、DNS のラベルエンコードがなぜ長さプレフィックス方式なのかも、コードを読んで初めて腑に落ちる。

---

## 6. プロトコルスタックの積み重なりイメージ

各サーバーがどのようにプロトコルを積み重ねているかを図示する。

```
tcp-echo:
  +---------------------------+
  |   echo (L7 相当なし)       |
  |   raw bytes (L6 相当なし)  |
  |   TCP 接続 (L4)            |
  +---------------------------+

http:
  +---------------------------+
  |   GET/POST routing (L7)   |
  |   HTTP/1.1 headers (L6)   |
  |   keep-alive conn (L5)    |
  |   TCP 接続 (L4)            |
  +---------------------------+

dns:
  +---------------------------+
  |   A record / NXDOMAIN (L7)|
  |   QNAME wire format (L6)  |
  |   TXID matching (L5)      |
  |   UDP datagram (L4)       |
  +---------------------------+

websocket:
  +---------------------------+
  |   broadcast to room (L7)  |
  |   WS frame / opcode (L6)  |
  |   Upgrade + room (L5)     |
  |   TCP 接続 (L4)            |
  +---------------------------+
```

このように、L4 は変わらず TCP か UDP だが、その上に乗るものが増えるにつれてコードの責務が増えていく。本章のコードウォークスルーはこの「増え方」を追体験するものである。

---

## 7. 観察ツール早見表

サーバーを動かして観察するための CLI ツールをまとめる。

| ツール | 典型的な使い方 | 何が見えるか |
|--------|--------------|-------------|
| `ss -tnlp` | `ss -tnlp \| grep 900` | TCP リスニングソケット一覧・PID |
| `tcpdump` | `sudo tcpdump -i lo0 'tcp port 9001' -X` | パケット単位の送受信・3-way handshake |
| `lsof` | `lsof -i :9001` | ポートを掴んでいるプロセス名・PID |
| `nc` | `nc localhost 9001` | TCP サーバーへの手動接続・インタラクティブ送受信 |
| `curl` | `curl -v http://localhost:9003/` | HTTP サーバーへの接続（生ヘッダも表示） |
| `dig` | `dig @127.0.0.1 -p 5353 example.local A` | DNS クエリの送信と応答確認 |
| `xxd` | `echo -n '\x00\x00\x00\x01\x02' \| xxd` | バイナリデータの 16 進ダンプ（LPP フレーム確認等） |

`tcpdump` と `xxd` を組み合わせると、ネットワーク上を流れるバイト列を生で読める。LPP や DNS のワイヤフォーマットを学ぶ際に特に役に立つ。

なお macOS と Linux でインターフェイス名が異なる（macOS: `lo0`、Linux: `lo`）点に注意する。Docker コンテナ内で実行する場合は `docker-compose.yml` の各サービスに対して `tcpdump` を docker ホスト側から実行するか、コンテナ内に `tcpdump` をインストールする必要がある。本リポジトリは `Dockerfile` と `docker-compose.yml`（`07_network/docker-compose.yml`）を用意しているため、`docker compose up` で全サーバーを一括起動してから `nc` や `curl` で疎通確認するのが最も手軽な環境である。

各サーバーとポートの対応を再掲する：

```
ホスト:ポート  サーバー     プロトコル   観察コマンド例
localhost:9001  tcp-echo    TCP          nc localhost 9001
localhost:9002  udp-echo    UDP          nc -u -w1 localhost 9002
localhost:9003  http        TCP          curl -v http://localhost:9003/
localhost:5353  dns         UDP          dig @127.0.0.1 -p 5353 example.local A
localhost:9004  lpp         TCP          cd servers/lpp && go run ./cmd/client
localhost:9005  websocket   TCP          go run ./cmd/sender / ./cmd/receiver
```

DNS のみポートが 5353 で他と異なるのは、標準の DNS ポート（53）は root 権限が必要なためである。開発中は非特権ポート（1024 以上）を使い、本番環境では `authbind` やリバースプロキシで 53 に転送するのが一般的なパターンだ。

---

## 8. まとめ / 関連 doc / この先の話題

### まとめ

本章の 6 サーバーはすべて TCP か UDP という L4 の上に立っている。tcp-echo / udp-echo は L4 そのものを裸で観察する場所であり、http / dns / lpp / websocket は TCP/IP モデルの「Application 層」の中に OSI の L5・L6・L7 が存在することを示している。OSI モデルは古い概念のように見えて、実際のコードの中に今も生きている。コードのコメントに `L5:` `L6:` `L7:` と書かれているのは、その層の意識を持ってほしいという設計意図からである。

各サーバーのコードを読む順番として、tcp-echo → udp-echo → http → dns → lpp → websocket の順が推奨される。前半 2 つで「L4 だけの世界」を体験し、後半 4 つで「L5/L6/L7 が増えていく世界」を追う流れになっている。本ドキュメント群（`01_concepts.md` 〜 `07_websocket.md`）はその順番に沿って書かれているため、順に読むと各層の役割が積み重なって理解できる。

### 関連 doc

- `02_tcp_basics.md` — TCP byte ストリームの正体と tcp-echo コードウォークスルー
- `03_udp_basics.md` — UDP データグラムの正体と udp-echo コードウォークスルー
- `04_http.md` — HTTP/1.1 の自己パース実装（L5/L6/L7 が揃う最初の例）
- `05_dns.md` — DNS ワイヤフォーマットの自己パース実装（UDP 上の L5/L6/L7）
- `06_lpp.md` — 独自バイナリプロトコル（LPP）の設計と TCP フレーミング
- `07_websocket.md` — WebSocket ハンドシェイクとフレーミング（HTTP Upgrade の仕組み）

### この先の話題

- L3（IP）を直接触る Raw Socket プログラミングと ICMP の実装
- TLS によって L6 の役割（暗号化・証明書の検証）が `net.Conn` の上に追加される様子
- HTTP/2 による多重化（単一 TCP 接続上の複数ストリーム = L5 の進化）
- HTTP/3（QUIC）: UDP 上で信頼性・順序・暗号化を再実装したアーキテクチャ

