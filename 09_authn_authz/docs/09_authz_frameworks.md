# 09. Go 認可フレームワーク比較

## 1. 概要

Go でアプリケーションに認可ロジックを組み込む際、大きく **ライブラリ型** と **サービス型** の二種類に分類できる。

| 区分 | 特徴 | 代表例 |
|------|------|--------|
| **ライブラリ型（プロセス内）** | アプリプロセスに直接組み込む。ネットワーク通信なしで評価でき、低レイテンシ。ポリシーの更新はアプリの再起動やリロードが必要になるケースが多い。 | Casbin、oso（OSS版）|
| **サービス型（外部 PDP）** | 認可ロジックを独立プロセスまたはサイドカーとして切り出す。ポリシーをアプリから分離して管理・更新できる。ネットワーク呼び出しのオーバーヘッドが生じる。 | OPA、OpenFGA、Cerbos、Oso Cloud |

認可モデル（RBAC / ABAC / ReBAC）や PEP/PDP/PIP アーキテクチャについては [10_authz_design.md](./10_authz_design.md) を参照。本ドキュメントは各フレームワーク・ツールの特性と選定基準に絞る。

本モジュールの authz デモは **Casbin** を採用している（`09_authn_authz/authz/`）。`model.conf` で RBAC モデルを定義し、`abac_model.conf` で ABAC モデルを定義する 2 エンフォーサ構成になっている。

---

## 2. 各フレームワーク

### 2.1 Casbin

**リポジトリ**: `github.com/casbin/casbin/v2`（現在は v3 も進行中）  
**ライセンス**: Apache-2.0  
**区分**: ライブラリ型（プロセス内）

#### モデル

Casbin のモデルは **PERM メタモデル**（Policy / Effect / Request / Matchers）をベースにした `.conf` ファイルで定義する。ACL・RBAC・RBAC with ドメイン・ABAC・RESTful パスマッチングなど、ほぼあらゆるモデルを構成ファイルで表現できる。

```ini
# model.conf（RBAC の例）
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act
```

ABAC はマッチャー内で Go の struct フィールドを直接参照できる。

```ini
# abac_model.conf（所有者チェックの例）
[matchers]
m = r.sub == r.obj.Owner && r.act == "edit"
```

#### Go での使い方

```go
import "github.com/casbin/casbin/v2"

// ファイルから生成
e, _ := casbin.NewEnforcer("model.conf", "policy.csv")

// embed.FS からも生成可能（本モジュールの実装例）
m, _ := model.NewModelFromString(modelData)
e, _ := casbin.NewEnforcer(m, NewBytesAdapter(policyData))

// 評価
allowed, _ := e.Enforce("alice", "/docs", "GET")

// ロール追加
e.AddRoleForUser("bob", "editor")
```

ポリシーストレージは CSV ファイルのほか、MySQL・PostgreSQL・Redis・MongoDB 等 30 以上のアダプタが公式提供されている。

#### デプロイ形態

アプリプロセスに組み込む **ライブラリ型**。Casbin Server という gRPC/HTTP PDP を別プロセスで立てる選択肢もあるが、利用例は限られる。

#### 長所

- Go ネイティブで外部プロセス不要、レイテンシが最小
- RBAC・ABAC・マルチテナント RBAC を単一ライブラリで対応
- アダプタ経由でポリシーを DB に保存・動的更新できる
- `EnforceEx` でマッチしたルールを取得でき、デバッグが容易

#### 短所

- ポリシーロジックを `.conf` の DSL で書く学習コストが存在する
- 「誰がどのオブジェクトにアクセス可能か」を逆引きする List クエリは標準 API では非効率
- 大規模なグラフ型権限（ReBAC）は不得意

#### 向くユースケース

小〜中規模 Go アプリで RBAC または ABAC を組み込みたい場合。マルチテナント SaaS でロールをテナントごとに管理したい場合。

---

### 2.2 OPA（Open Policy Agent）

**リポジトリ**: `github.com/open-policy-agent/opa`  
**ライセンス**: Apache-2.0  
**区分**: ライブラリ型・サービス型 両対応

#### モデル

**Rego** という汎用ポリシー言語を使う。Rego は宣言型の論理言語で、JSON 形式の入力データに対して任意のポリシーを記述できる。RBAC・ABAC・混合モデルすべてを Rego で表現可能。

```rego
package authz

default allow := false

allow if {
    input.method == "GET"
    input.path[0] == "docs"
    "viewer" in data.roles[input.user]
}
```

#### Go での使い方

**ライブラリとして組み込む**場合（`rego` パッケージ）:

```go
import "github.com/open-policy-agent/opa/v1/rego"

query, _ := rego.New(
    rego.Query("data.authz.allow"),
    rego.Load([]string{"policy.rego"}, nil),
).PrepareForEval(ctx)

rs, _ := query.Eval(ctx, rego.EvalInput(map[string]any{
    "user":   "alice",
    "method": "GET",
    "path":   []string{"docs"},
}))
allowed := rs[0].Expressions[0].Value.(bool)
```

**Go SDK として組み込む**場合（バンドルサーバ + `sdk` パッケージ）:

```go
import "github.com/open-policy-agent/opa/v1/sdk"

opa, _ := sdk.New(ctx, sdk.Options{Config: bytes.NewReader(config)})
result, _ := opa.Decision(ctx, sdk.DecisionOptions{
    Path:  "/authz/allow",
    Input: map[string]any{"open": "sesame"},
})
```

**サイドカー/外部プロセス** として立てる場合は HTTP REST API (`/v1/data/{path}`) または gRPC でクエリする。

#### デプロイ形態

- **ライブラリ**: アプリプロセスに組み込み、ポリシーをバンドルとして取得
- **サイドカー**: Kubernetes 上で Pod と同一ノードに OPA を展開
- **中央 PDP**: クラスタ外部に OPA サービスとして立てる

#### 長所

- Rego は Kubernetes Admission Control・API ゲートウェイ・CI パイプラインなど認可以外にも流用できる汎用エンジン
- CNCF 卒業プロジェクト。大規模採用実績あり
- バンドル機能でポリシーをアプリから独立して配布・更新可能
- Go ネイティブ実装のため、ライブラリ利用時はゼロ依存で組み込める

#### 短所

- Rego の学習コストが高い（ロジックプログラミングの概念が必要）
- サイドカー構成にすると運用コスト（OPA プロセスの管理・バンドル配布基盤）が増える
- 関係グラフ型権限（ReBAC）には不向き

#### 向くユースケース

Kubernetes やインフラ層のポリシーを同一エンジンで一元管理したい場合。ポリシーをコードとして Git 管理して CI で検証したい場合。複数サービスにまたがる汎用ポリシー基盤を作りたい場合。

---

### 2.3 OpenFGA

**リポジトリ**: `github.com/openfga/openfga`、Go SDK: `github.com/openfga/go-sdk`  
**ライセンス**: Apache-2.0  
**区分**: サービス型（外部 PDP）  
**ステータス**: CNCF Incubating（2022〜）

#### モデル

Google Zanzibar 論文に基づく **ReBAC（関係ベースアクセス制御）**。権限の有無を「ユーザーとオブジェクト間に特定の関係が存在するか」で判定する。

**Authorization Model**（DSL で記述）:

```
model
  schema 1.1

type user

type document
  relations
    define owner: [user]
    define editor: [user] or owner
    define viewer: [user] or editor
```

**関係タプル**（実行時データ）:

```
user:alice  →  editor  →  document:report-1
user:bob    →  viewer  →  document:report-1
```

#### Go での使い方

```go
import (
    openfga "github.com/openfga/go-sdk"
    "github.com/openfga/go-sdk/client"
)

fgaClient, _ := client.NewSdkClient(&client.ClientConfiguration{
    ApiUrl: "http://localhost:8080",
    StoreId: "store-id",
    AuthorizationModelId: "model-id",
})

// 権限チェック
resp, _ := fgaClient.Check(ctx).Body(client.ClientCheckRequest{
    User:     "user:alice",
    Relation: "viewer",
    Object:   "document:report-1",
}).Execute()
allowed := resp.GetAllowed()

// 関係タプルの書き込み
fgaClient.WriteTuples(ctx).Body(client.ClientWriteTuplesBody{
    {User: "user:carol", Relation: "editor", Object: "document:report-1"},
}).Execute()
```

#### デプロイ形態

OpenFGA は **独立した gRPC/HTTP サービス** として動作する。ストレージバックエンドは In-Memory・PostgreSQL・MySQL・SQLite（ベータ）を選択できる。Kubernetes 上では Helm Chart が提供されている。

#### 長所

- グラフ構造の権限（「フォルダが閲覧できればファイルも閲覧できる」など）を自然に表現できる
- `ListObjects` API でユーザーがアクセスできるリソース一覧を効率よく取得できる（Zanzibar の逆引き）
- Auth0/Okta・Grafana Labs・Docker・Canonical など大手が採用
- 監査ログ・OpenTelemetry 対応

#### 短所

- 外部サービスとして管理が必要（可用性・バックアップ）
- 属性情報（例: 時刻・IP・金額）をタプルに変換する設計が必要で、純粋な ABAC は不得意
- 関係モデルの設計スキルが必要

#### 向くユースケース

SaaS のドキュメント共有・チーム権限のような「誰が誰に何を共有しているか」がグラフ構造になるケース。Googleドライブ型の「フォルダ権限を子に継承」のような階層的権限管理。

---

### 2.4 Cerbos

**リポジトリ**: `github.com/cerbos/cerbos`、Go SDK: `github.com/cerbos/cerbos-sdk-go`  
**ライセンス**: Apache-2.0（コアは OSS、Hub は SaaS）  
**区分**: サービス型（外部 PDP）

#### モデル

**RBAC + ABAC の混合**をサポートする **YAML ポリシー**。ポリシーはリソース単位で定義し、プリンシパル（ユーザー）の属性とリソースの属性の両方を条件に使える。

```yaml
# resource_policy: expense.yaml
apiVersion: api.cerbos.dev/v1
resourcePolicy:
  version: "default"
  resource: "expense"
  rules:
    - actions: ["view"]
      effect: EFFECT_ALLOW
      roles: ["employee"]
    - actions: ["approve"]
      effect: EFFECT_ALLOW
      roles: ["manager"]
      condition:
        match:
          expr: request.resource.attr.amount < 1000
```

#### Go での使い方

```go
import (
    "github.com/cerbos/cerbos-sdk-go/cerbos"
)

c, _ := cerbos.New("localhost:3593", cerbos.WithPlaintext())

pp := cerbos.NewPrincipal("alice", "employee").
    WithAttr("department", "engineering")
rr := cerbos.NewResource("expense", "exp-001").
    WithAttr("amount", 500)

resp, _ := c.CheckResources(ctx,
    cerbos.NewCheckResourcesRequest(pp,
        cerbos.NewResourceBatch().Add(rr, "view", "approve"),
    ),
)
allowed := resp.IsAllowed("exp-001", "approve")
```

#### デプロイ形態

Cerbos PDP は **ステートレスなコンテナ**として動作する。Kubernetes サービス・サイドカー・systemd・AWS Lambda のいずれにも対応。ポリシーファイルは Git・ローカル FS・クラウドストレージ（S3 等）から読み込める。ポリシー更新はプロセス再起動不要でホットリロードされる。評価レイテンシは 1ms 未満。

#### 長所

- YAML ポリシーは非エンジニア（ビジネスアナリスト等）と共有・レビューしやすい
- `PlanResources` API でアクセス可能なリソース一覧のフィルタ条件を生成できる（DB クエリに適用可能）
- 監査ログが組み込み済み（OpenTelemetry・stdout 対応）
- ステートレスなのでスケールアウトが容易

#### 短所

- 外部プロセスが必要（ライブラリ組み込みに比べて運用負荷が上がる）
- グラフ型権限（ReBAC）は OpenFGA に比べて表現力が劣る
- Oso Cloud・Cerbos Hub のような SaaS 管理 UI は有償

#### 向くユースケース

ポリシーを運用チームや非エンジニアと共同管理したい場合。監査・コンプライアンス要件が厳しい場合。マイクロサービス構成でポリシーを集中管理したい場合。

---

### 2.5 oso / Oso Cloud

**リポジトリ**: OSS 版 `github.com/osohq/oso`（非推奨）、Oso Cloud Go SDK: `github.com/osohq/go-oso-cloud/v2`  
**ライセンス**: Apache-2.0  
**区分**: SaaS 型（Oso Cloud）

> **注意**: オリジナルの OSS ライブラリ `osohq/oso`（プロセス内埋め込み）は 2023 年に非推奨となった。現在の主力製品は **Oso Cloud**（Authorization as a Service）で、Go SDK（v2）は 2026 年 3 月時点でも活発に更新されている。

#### モデル

**Polar** という宣言型ロジックプログラミング言語でポリシーを記述する。RBAC・ReBAC・ABAC を統一した構文で表現できる。

```polar
# Polar ポリシーの例
actor User {}

resource Document {
    roles = ["viewer", "editor", "owner"];
    permissions = ["read", "write", "delete"];

    "read" if "viewer";
    "write" if "editor";
    "delete" if "owner";

    "editor" if "owner";
    "viewer" if "editor";
}

has_role(user: User, "owner", doc: Document) if
    doc.owner_id = user.id;
```

#### Go での使い方

Oso Cloud SDK を使って外部サービスに権限チェックを委譲する。

```go
import oso "github.com/osohq/go-oso-cloud/v2"

client := oso.NewClient("https://cloud.osohq.com", os.Getenv("OSO_AUTH"))

// ローカル DB と組み合わせたチェック（AuthorizeLocal）
allowed, _ := client.Authorize(ctx,
    oso.TypedID("User", "alice"),
    "read",
    oso.TypedID("Document", "doc-1"),
)

// ユーザーがアクセスできるリソース一覧
docs, _ := client.ListLocal(ctx,
    oso.TypedID("User", "alice"),
    "read",
    "Document",
    localQuery,
)
```

#### デプロイ形態

**Oso Cloud**（SaaS）への API 呼び出し。ポリシーは Oso Cloud のダッシュボードまたは CLI で管理する。ローカル DB クエリと組み合わせる `AuthorizeLocal` / `ListLocal` API がある。

#### 長所

- インフラ管理不要で即座に利用開始できる
- Polar 言語はロジック思考に合った簡潔な記述ができる
- `ListLocal` でローカル DB との結合フィルタリングが可能
- リアルタイムの決定ログと監査機能が付属

#### 短所

- SaaS なのでデータが外部に出る（プライバシー・コンプライアンス上の制約あり）
- OSS 版（プロセス内組み込み）は非推奨のため、ベンダー依存になる
- 無料プランの制限を超えると有償

#### 向くユースケース

スタートアップや小規模チームでインフラ運用コストを最小化したい場合。Polar 言語の表現力で複雑な権限モデルを柔軟に実装したい場合。

---

## 3. 比較表

| 項目 | Casbin | OPA | OpenFGA | Cerbos | Oso Cloud |
|------|--------|-----|---------|--------|-----------|
| **主なモデル** | RBAC / ABAC / ACL（マルチモデル） | 汎用（Rego で自由定義） | ReBAC（関係タプル） | RBAC + ABAC | RBAC / ReBAC / ABAC（Polar） |
| **ポリシー記述** | `.conf` DSL + CSV / DB | Rego（宣言型ロジック言語） | Authorization Model DSL + タプル | YAML | Polar（宣言型ロジック言語） |
| **デプロイ形態** | プロセス内ライブラリ | ライブラリ / サイドカー / 中央 PDP | 独立サービス（Docker/K8s） | ステートレス PDP（サイドカー/サービス） | SaaS（Oso Cloud） |
| **データの持ち方** | ポリシーファイル / DB アダプタ | バンドル（JSON/Rego） | 関係タプル（PostgreSQL / MySQL 等） | YAML ポリシー（Git / S3 等） | Oso Cloud が管理 |
| **Go サポート成熟度** | ★★★★★（Go がファースト言語） | ★★★★★（Go 実装） | ★★★★（公式 SDK あり） | ★★★★（公式 SDK あり） | ★★★（SDK は活発だが OSS 本体は非推奨） |
| **運用負荷** | 低（ライブラリ組み込み） | 中（バンドル配布基盤が必要） | 高（外部サービス管理） | 中（ステートレス PDP 管理） | 低（SaaS 依存） |
| **逆引きクエリ** | 弱（非効率） | 弱（非ネイティブ） | 強（ListObjects） | 中（PlanResources） | 強（ListLocal） |
| **向くケース** | 単一アプリの RBAC/ABAC | インフラ横断ポリシー集中管理 | グラフ型オーナーシップ権限 | 監査重視の RBAC/ABAC | 小規模・SaaS 優先 |

---

## 4. 選定ガイド

```
小規模・Go 単体アプリ、組み込み優先
  → Casbin（RBAC/ABAC 両対応、Go ネイティブ）

汎用ポリシーを複数サービス・インフラで集中管理
  → OPA（Rego の汎用性、CNCF 卒業、CI 検証も可能）

SaaS のドキュメント共有・チーム権限・階層的オーナーシップ
  → OpenFGA（Zanzibar 型 ReBAC、ListObjects が強力）

ポリシーを非エンジニアと共有・YAML で管理・監査ログ重視
  → Cerbos（YAML 可読性、PlanResources、ステートレス PDP）

インフラ運用コスト最小化・複雑な権限を Polar で表現したい
  → Oso Cloud（SaaS、ListLocal でローカル DB と結合可能）
```

### 追加の判断軸

- **チームに Rego/Polar の学習コストを払えるか**: OPA・Oso Cloud はポリシー言語の習得が前提
- **外部サービスへのデータ送信が許容されるか**: Oso Cloud は SaaS のため、機密データのポリシー評価に注意
- **ポリシー更新の即時性が必要か**: Cerbos はホットリロード対応、Casbin は DB アダプタ経由で動的更新可能
- **監査・コンプライアンス要件**: Cerbos と OPA は監査ログが組み込み済み、OpenFGA も OpenTelemetry 対応

---

## 5. フレームワークを使わない場合

フレームワークを導入せず、アプリ内で認可ロジックを自前実装することもある。その場合でも、**PEP（Policy Enforcement Point）と PDP（Policy Decision Point）の分離**、**段階的な設計（RBAC → ABAC → ReBAC）** を意識することが重要になる。

設計指針については [10_authz_design.md](./10_authz_design.md) を参照。PEP/PDP の役割分担、PIP（Policy Information Point）によるデータ提供、段階設計のロードマップを解説している。

---

## 6. まとめ

| フレームワーク | 一言まとめ |
|--------------|-----------|
| **Casbin** | Go ネイティブの多モデル対応ライブラリ。小〜中規模アプリに最適 |
| **OPA** | Rego による汎用ポリシーエンジン。インフラ横断の集中管理に強い |
| **OpenFGA** | Zanzibar 型 ReBAC。SaaS のグラフ的オーナーシップ権限に強い |
| **Cerbos** | YAML ポリシーのステートレス PDP。監査・非エンジニア協業に強い |
| **Oso Cloud** | Polar 言語 + SaaS。運用コスト最小で複雑なモデルを実装できる |

本ドキュメントの情報は **2026 年 6 月時点**のものである。各フレームワークのバージョン・API は変更される可能性があるため、最新情報は各プロジェクトの公式ドキュメントを参照すること。

- Casbin: https://casbin.apache.org/
- OPA: https://www.openpolicyagent.org/
- OpenFGA: https://openfga.dev/
- Cerbos: https://docs.cerbos.dev/
- Oso Cloud: https://www.osohq.com/
