# 07_network: ネットワーク学習プロジェクト

TCP/UDP の上に HTTP/DNS/独自プロトコル/WebSocket が「足し算」で乗っていることを Go の `net` パッケージで体感する章。第三者ライブラリは使わず stdlib のみ。

## 学習動線

1. [01: 概念とレイヤーの位置づけ](docs/01_concepts.md)
2. [02: TCP の正体](docs/02_tcp_basics.md)
3. [03: UDP の正体](docs/03_udp_basics.md)
4. [04: TCP の上の HTTP](docs/04_http_on_tcp.md)
5. [05: UDP の上の DNS](docs/05_dns_on_udp.md)
6. [06: TCP の上の自作プロトコル](docs/06_custom_protocol.md)
7. [07: WebSocket（HTTP で始まり TCP に居座る）](docs/07_websocket.md)
8. [08: レイヤーを観るツール集](docs/08_observability.md)

## クイックスタート

```bash
make up           # 6 サーバー起動
make demo-tcp     # 素の TCP echo
make demo-udp     # 素の UDP echo
make demo-http    # HTTP/1.1
make demo-dns     # DNS A クエリ
make demo-lpp     # 自作プロトコル
make demo-ws      # WebSocket（sender/receiver 起動手順）
make test-race    # race 検出付き全テスト
make down
```

## サーバー一覧

| サーバー | ポート | プロトコル | docs |
|---|---|---|---|
| tcp-echo  | 9001/tcp  | 素の TCP | [02](docs/02_tcp_basics.md) |
| udp-echo  | 9002/udp  | 素の UDP | [03](docs/03_udp_basics.md) |
| http      | 9003/tcp  | HTTP/1.1 自前実装 | [04](docs/04_http_on_tcp.md) |
| dns       | 5353/udp  | DNS A クエリ自前実装 | [05](docs/05_dns_on_udp.md) |
| lpp       | 9004/tcp  | 長さプレフィックス独自プロトコル | [06](docs/06_custom_protocol.md) |
| websocket | 9005/tcp  | WebSocket + broadcast hub | [07](docs/07_websocket.md) |

## 環境注意

- **Go バージョン**: `go.work` は `go 1.26` を要求し、`toolchain go1.26.0` でツールチェイン自動DLを指定。ローカル Go が 1.25 系でも `go test`/`go build` は走るが、LSP は 1.26 を手動インストールしないと警告を出す。
- **macOS** の `tcpdump` は loopback が `lo0`。**Linux** は `lo`。
- DNS 用ポートは衝突回避のため `5353/udp`。`dig @localhost -p 5353` で叩く。
- WebSocket クライアント（`cmd/sender`, `cmd/receiver`）はホスト側で `go run` する想定。
