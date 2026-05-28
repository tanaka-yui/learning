# 08_observability: ネットワークを観察するツール早見表

本章では 02〜07 の各 doc で登場した観察コマンドをまとめて整理する。「どのツールがどの層を見せるか」を把握すると、問題発生時に迷わずツールを選べる。

---

## 1. ツール早見表

| ツール | 見えるもの | 主な利用シーン | 参照 doc |
|--------|-----------|--------------|---------|
| `tcpdump` | TCP/UDP のパケット（ヘッダ + ペイロード） | フレームバイト列の確認、接続の確立・切断、L4-L6 の観察 | 02, 04, 05, 07 |
| `ss -tnl` | TCP の LISTEN/ESTABLISHED ソケット一覧 | サーバーが正しいポートでリスニングしているか確認 | 02 |
| `lsof -i :PORT` | ポートを使っているプロセス | ポート衝突・二重起動の診断 | 02 |
| `nc` | 生 TCP/UDP の送受信（手動クライアント） | サーバーへの最小限の接続テスト | 02, 03 |
| `curl -v` | HTTP リクエスト・レスポンスのヘッダ | HTTP サーバーの動作確認、ヘッダの中身 | 04 |
| `dig` | DNS クエリ・レスポンスのテキスト表示 | DNS サーバーの動作確認、A/CNAME レコード | 05 |
| `xxd` | バイナリファイルの 16 進ダンプ | ファイルやパイプ経由でバイナリの中身を確認 | — |

---

## 2. `tcpdump` 基本

### インターフェイス指定

```bash
# macOS: ループバックは lo0
sudo tcpdump -i lo0 'tcp port 9001'

# Linux: ループバックは lo
sudo tcpdump -i lo 'tcp port 9001'
```

`-i any` を使うと全インターフェイスをキャプチャできる（Linux のみ）。macOS では `-i any` は使えない。

### ペイロード表示オプション

```bash
# -X: 16進数 + ASCII を並べて表示（バイナリプロトコルの観察に最適）
sudo tcpdump -X -i lo0 'tcp port 9005'

# -A: ASCII のみ表示（HTTP のようなテキストプロトコルに向く）
sudo tcpdump -A -i lo0 'tcp port 9003'
```

`-X` は LPP（06章）や WebSocket（07章）のバイナリフレームの解析に、`-A` は HTTP（04章）のテキストヘッダの確認に使い分けると見やすい。

### フィルタ式

```bash
# TCP のポートを絞る
sudo tcpdump -i lo0 'tcp port 9001'

# UDP のポートを絞る
sudo tcpdump -X -i lo0 'udp port 5353'

# 特定ホストのみ
sudo tcpdump -i en0 'host 192.168.1.1'

# SYN パケットのみ（接続確立の観察）
sudo tcpdump -i lo0 'tcp[13] & 2 != 0'
```

フィルタ式は BPF（Berkeley Packet Filter）の構文に従う。`man pcap-filter` で全式を確認できる。

---

## 3. `ss -tnl` / `ss -unl` (Linux)

`ss`（socket statistics）は Linux でソケット状態を表示するコマンドだ。`netstat` の後継にあたる。

```bash
# TCP の LISTEN ソケット一覧
ss -tnl

# UDP のバインド済みソケット一覧
ss -unl

# ポートを絞る
ss -tnlp | grep 9001
```

出力例:

```
State    Recv-Q  Send-Q  Local Address:Port  Peer Address:Port
LISTEN   0       4096    0.0.0.0:9001        0.0.0.0:*
```

`-t` = TCP、`-u` = UDP、`-n` = 名前解決しない（数値で表示）、`-l` = LISTEN のみ、`-p` = プロセス名・PID も表示。

### macOS での代替

macOS には `ss` がない。代わりに `lsof` か `netstat` を使う。

```bash
# macOS: TCP LISTEN を表示
lsof -iTCP -sTCP:LISTEN

# macOS: 全 TCP/UDP ソケット（数値表示）
netstat -an | grep LISTEN
```

---

## 4. `lsof -i :PORT`

```bash
# ポート 9001 を使っているプロセスを確認
lsof -i :9001
```

出力例:

```
COMMAND   PID      USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
main     1234 yui.tanaka    3u  IPv4 0x...        0t0  TCP *:9001 (LISTEN)
```

`lsof` (list open files) は macOS・Linux 両方で使える。ポートが「すでに使われている」エラーで起動できないときの原因特定に使う。`-iTCP` / `-iUDP` でプロトコル絞り込み、`-sTCP:LISTEN` で LISTEN 状態のみに絞れる。

```bash
# TCP LISTEN を全部表示（プロセス名付き）
lsof -iTCP -sTCP:LISTEN -n -P
```

---

## 5. `nc` / `curl -v` / `dig +short +trace` / `xxd`

### `nc` (netcat)

`nc` は「TCP/UDP の生ソケットを手動で使うツール」だ。サーバーが正しく応答するかを最小コストで確認できる。

```bash
# TCP サーバーに接続して手入力
nc localhost 9001

# データをパイプで送って即切断
echo "hello" | nc localhost 9001

# UDP モード (-u)
echo "test" | nc -u -w1 localhost 9002

# 巨大データで TCP 分割を観察
python3 -c "print('A' * 5000)" | nc localhost 9001
```

`-w1` はタイムアウト秒数（UDP で使うことが多い）。nc 自体は WebSocket のマスキングを行わないため、WebSocket サーバー（9005 番）への接続には使えない。

### `curl -v`

HTTP サーバーに対してリクエストを送りながらヘッダを全表示する。

```bash
# 基本 (GET)
curl -v http://localhost:9003/

# POST
curl -v -X POST http://localhost:9003/echo -d "hello"

# 特定ヘッダを追加
curl -v -H "X-Debug: 1" http://localhost:9003/

# レスポンスのみ (ヘッダ非表示)
curl -s http://localhost:9003/
```

`>` で始まる行がリクエスト、`<` で始まる行がレスポンスヘッダだ。`*` で始まる行は curl 自身の状態（接続確立・TLS ネゴシエーションなど）を示す。

### `dig +short +trace`

DNS サーバーへのクエリを送るツールだ。

```bash
# カスタム DNS サーバーに A レコードを問い合わせる
dig @127.0.0.1 -p 5353 example.local A +short

# 存在しないドメイン（NXDOMAIN の確認）
dig @127.0.0.1 -p 5353 unknown.local A

# ルートから順に委任を追いかける（本物の DNS サーバー向け）
dig +trace example.com

# レコードタイプを変える
dig @127.0.0.1 -p 5353 example.local AAAA
```

`+short` は最終結果のみ表示。`@127.0.0.1 -p 5353` でカスタムポートの DNS を指定する。

### `xxd`

バイナリデータを 16 進数ダンプで表示する。ファイルやパイプと組み合わせて使う。

```bash
# ファイルの先頭 32 バイトを 16 進ダンプ
xxd -l 32 somefile.bin

# 標準入力から（echo の -e でバイト指定）
printf '\x81\x85\xAA\xBB\xCC\xDD' | xxd

# バイナリファイルを生成（patch モード）
echo "00000000: 4865 6c6c 6f0a" | xxd -r > hello.bin
```

WebSocket フレームのヘッダバイト（例: `0x81 0x85`）の意味を手作業で検証するときに便利だ。

---

## 6. macOS と Linux の差分

本章のコードは macOS・Linux 両方で動作するが、観察ツールにはプラットフォーム差がある。

| 操作 | macOS | Linux |
|------|-------|-------|
| tcpdump ループバック | `-i lo0` | `-i lo` |
| LISTEN ソケット一覧 | `lsof -iTCP -sTCP:LISTEN` | `ss -tnlp` |
| ソケット統計 | `netstat -an` | `ss -s` または `netstat -an` |
| `ss` コマンド | **なし** | あり（iproute2 パッケージ） |
| プロセスのポート確認 | `lsof -i :PORT` | `lsof -i :PORT` または `ss -tnlp \| grep PORT` |
| パケットキャプチャ GUI | Wireshark (要インストール) | Wireshark / tshark |

`tcpdump` の `-i lo0`（macOS）を `-i lo`（Linux）に変えるのを忘れて「何も出ない」になるのが最もよくある間違いだ。Docker コンテナ内では次節を参照。

---

## 7. Docker 環境での観察

### コンテナ内からの観察

```bash
# コンテナ内シェルに入る
docker compose exec <service-name> sh

# コンテナ内で ss / lsof を使う（ツールが入っているか確認）
ss -tnl
lsof -i :9001
```

Alpine ベースのイメージには `ss` が入っていないことが多い。`apk add iproute2` で追加する。

### ホストから Docker コンテナへの tcpdump

```bash
# Docker が作るブリッジネットワークを確認
ip link show | grep docker

# docker0 ブリッジ上のトラフィックをキャプチャ（Linux）
sudo tcpdump -i docker0 'tcp port 9001' -X
```

コンテナが `ports:` マッピングを持つ場合（`9001:9001`）、ホストの tcpdump はホスト側インターフェイスで見えることが多い。

### --net=host でホストのネットワーク名前空間を共有（Linux のみ）

```bash
# Linux のみ: コンテナがホストの netns を使う
docker run --net=host --rm alpine:latest ss -tnl
```

`--net=host` は macOS の Docker Desktop では機能しない（macOS の Docker は Linux VM の中で動いているため）。

### macOS の Docker Desktop での tcpdump

Docker Desktop (macOS) のコンテナは Linux VM 内で動作するため、ホストの `tcpdump -i lo0` ではコンテナ間のトラフィックは見えない。代わりにコンテナ内で tcpdump を実行する。

```bash
docker compose exec <service-name> sh -c "apk add tcpdump && tcpdump -i eth0 'tcp port 9001' -X"
```

---

## 8. まとめ / 関連 doc

**まとめ**

ネットワークの問題を診断するとき、ツールの役割を層で整理すると選びやすい。「パケットが届いているか」は `tcpdump`（L4 以下）、「ポートが開いているか」は `ss` / `lsof`（L4）、「HTTP が正しく返るか」は `curl -v`（L7）、「DNS が応答するか」は `dig`（L7/アプリ）だ。macOS と Linux でコマンドが異なる最大のポイントは `tcpdump` のインターフェイス名（`lo0` vs `lo`）と `ss` コマンドの有無である。

**関連 doc**

- [02_tcp_basics.md](./02_tcp_basics.md) — tcpdump・ss・lsof の初出
- [04_http_on_tcp.md](./04_http_on_tcp.md) — curl -v で HTTP ヘッダを観察
- [05_dns_on_udp.md](./05_dns_on_udp.md) — dig と tcpdump で DNS バイナリを観察
- [07_websocket.md](./07_websocket.md) — tcpdump でハンドシェイクとバイナリフレームの境目を観察
