# 02_cache: 各パターンのアーキテクチャドキュメント作成

## Context

02_cacheプロジェクトには4つのキャッシュパターン（app-cache, shared-cache, cdn-nginx, cdn-go）が実装されているが、各パターンのアーキテクチャを視覚的に理解するためのドキュメントがない。各パターンごとに`docs`フォルダを作成し、Mermaid記法でアーキテクチャ図を記載し、README.mdからリンクする。

## 作成するファイル

### 1. 各パターンのdocsフォルダ・ファイル

| ファイル | 内容 |
|---------|------|
| `02_cache/app-cache/docs/architecture.md` | アプリ内メモリキャッシュのアーキテクチャ |
| `02_cache/shared-cache/docs/architecture.md` | Valkey共有キャッシュのアーキテクチャ |
| `02_cache/cdn-nginx/docs/architecture.md` | nginxキャッシュのアーキテクチャ |
| `02_cache/cdn-go/docs/architecture.md` | Go自作CDNキャッシュのアーキテクチャ |

### 2. 全体比較ドキュメント

| ファイル | 内容 |
|---------|------|
| `02_cache/docs/overview.md` | 全パターン横断比較のアーキテクチャ概要図 |

### 3. README.md更新

`02_cache/README.md` に各docsへのリンクを追加

## 各docsの構成（共通）

各 `architecture.md` には以下の3種類のMermaid図を含める：

### A. リクエストフロー図（sequence diagram）
- クライアント → キャッシュ層 → バックエンドのリクエスト/レスポンスフロー
- HIT/MISS の分岐を含む
- レスポンスヘッダー（X-Cache, X-Backend-Instance）の付与タイミング

### B. コンポーネント図（flowchart）
- 内部構造（キャッシュストレージ、キャッシュキー生成、Cache-Control解析）
- パターン固有の要素：
  - **app-cache**: `sync.RWMutex` + `map[string]*cacheEntry`
  - **shared-cache**: Go → Valkey (Redis互換) + JSON シリアライズ
  - **cdn-nginx**: `proxy_cache_path` ディスクキャッシュ + `keys_zone` 共有メモリ
  - **cdn-go**: `sync.RWMutex` + `map` + Varyヘッダー対応キャッシュキー

### C. 比較図（全体overview.mdのみ）
- 4パターンの全体アーキテクチャ概要（flowchart）
- クライアント → 各ポート → 各キャッシュ層 → バックエンド2台の全体像

## 実装手順

1. `02_cache/app-cache/docs/architecture.md` を作成
2. `02_cache/shared-cache/docs/architecture.md` を作成
3. `02_cache/cdn-nginx/docs/architecture.md` を作成
4. `02_cache/cdn-go/docs/architecture.md` を作成
5. `02_cache/docs/overview.md` を作成（全体比較図）
6. `02_cache/README.md` を更新（各docsへのリンク追加）

## 参照ファイル

- `02_cache/README.md` — 既存README
- `02_cache/docker-compose.yml` — ポートマッピング・サービス構成
- `02_cache/app-cache/main.go`, `cache.go`, `cachecontrol.go`
- `02_cache/shared-cache/main.go`, `cache.go`, `cachecontrol.go`
- `02_cache/cdn-nginx/nginx.conf`
- `02_cache/cdn-go/main.go`, `cache.go`, `cachecontrol.go`
- `02_cache/backend/main.go`

## 検証

- 各Mermaid図がGitHub上で正しくレンダリングされるか確認（Mermaid記法の構文チェック）
- README.mdのリンクが正しいパスを指しているか確認
