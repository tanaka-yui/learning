# 認可設計パターン

## 1. 概要

**認可（Authorization）** とは、「認証済みの主体が、特定のリソースに対して特定の操作を実行してよいか」を判定することである。認証（Authentication）が「誰であるか」を確認するのに対し、認可は「何が許可されるか」を決定する。両者は隣接するが明確に分離すべき責務であり、同一コンポーネントに混在させると変更・テスト・監査が困難になる。

認可ロジックを散らかさないことが重要な理由は次の通りである。

- **変更の局所化**: ポリシーが変わるたびにハンドラーを散策する必要がなくなる
- **テスト容易性**: ポリシーを単体で検証できる
- **監査性**: 「誰が何を許可されているか」を一箇所で確認できる
- **deny-by-default の一貫した適用**: 判定ロジックが分散していると許可チェック漏れ（「deny漏れ」）が発生しやすい

---

## 2. 認可モデル

### 2-1. RBAC（ロールベースアクセス制御）

**定義**: ユーザーをロールに割り当て、ロールに権限を付与する。主体→ロール→権限の3層構造。ロール継承（admin ⊇ editor ⊇ viewer）により階層表現が可能。

**例（本モジュール `policy.csv`）**:

```
# ロールへの権限付与
p, admin,  /docs, GET
p, admin,  /docs, POST
p, admin,  /docs, DELETE
p, editor, /docs, GET
p, editor, /docs, POST
p, viewer, /docs, GET

# ユーザーのロール割り当て
g, alice, admin
g, bob,   editor
g, carol, viewer
```

**長所**:
- 管理がシンプル。ロール数が少ない組織では直感的
- 監査が容易（ロール一覧で権限を把握できる）
- Go の Casbin や多くの IAM でネイティブサポート

**短所**:
- きめ細かい制御が必要になると「ロール爆発」（role explosion）が起きる
- 「自分のリソースだけ編集可能」のような所有者条件を表現しにくい
- 動的な属性（時間帯、IPアドレスなど）を扱えない

---

### 2-2. ABAC（属性ベースアクセス制御）

**定義**: 主体・リソース・環境の属性を組み合わせてポリシーを評価する。RBAC より表現力が高く、ロールを介さずに属性値で直接判定できる。

主な属性の種類:
- **主体属性**: ユーザーID、部署、契約種別、信頼レベル
- **リソース属性**: 所有者ID、機密レベル、作成日時
- **環境属性**: 時刻、送信元IP、認証強度

**例（本モジュール `abac_model.conf`）**:

```
[matchers]
m = r.sub == r.obj.Owner && r.act == "edit"
```

Go 側でリソースを struct として渡すことで属性比較を実現している:

```go
type Resource struct {
    ID    string
    Owner string
}

res := lookupResource(id)          // PIP: リソース属性を取得
allowed, _ := abac.Enforce(sub, res, "edit")  // PDP: 判定
```

`doc1` の Owner は `alice` なので、`alice` だけが `POST /docs/doc1/edit` を許可される。

**長所**:
- 所有者・部署・機密度など動的な条件を自然に表現できる
- ロール爆発を回避できる
- 環境属性でコンテキスト依存の制御が可能

**短所**:
- ポリシーが複雑になると把握・デバッグが難しい
- 属性の収集（PIP）インフラが必要になる
- RBAC より学習コストが高い

---

### 2-3. ReBAC（関係ベースアクセス制御）

**定義**: ユーザーとリソース間の**関係グラフ**に基づいて権限を判定する。Google の Zanzibar 論文（2019年）が基礎。代表的実装は OpenFGA・SpiceDB。

主な関係の例:
- **owner**: ユーザーがリソースを所有している
- **editor**: ユーザーがリソースを編集できる（owner から継承可能）
- **viewer**: ユーザーがリソースを閲覧できる（editor から継承可能）
- **member**: ユーザーがグループのメンバー（グループ→リソースの関係に連鎖）

**権限グラフの例**:

```
alice --[owner]--> doc1
  |
  +-- owner implies editor
  +-- editor implies viewer

team-a --[member]--> bob
team-a --[viewer]--> doc2
  => bob can view doc2 (グループ経由)
```

**長所**:
- Google Drive や GitHub のような「共有」「継承」「グループ」を自然にモデリングできる
- 細粒度かつ拡張性が高い（数十億タプルのスケール実績あり）
- 権限の伝播が明示的なグラフで可視化できる

**短所**:
- 関係タプルのストアが必要（追加インフラ）
- RBAC より実装・運用コストが高い
- Casbin など軽量ライブラリでは ReBAC をネイティブサポートしていない

> Go の ReBAC/OpenFGA 実装については [`09_authz_frameworks.md`](./09_authz_frameworks.md) で扱う。

---

### 2-4. PBAC / ポリシーベースアクセス制御

**定義**: アクセスポリシーを宣言的な言語（Rego / CEL など）で記述し、PDP エンジンが評価する。OPA（Open Policy Agent）や Cerbos が代表例。RBAC・ABAC のルールを Rego などで記述する形式の上位概念とも言える。

**例（OPA Rego のイメージ）**:

```rego
allow {
    input.user.role == "admin"
}

allow {
    input.user.role == "editor"
    input.action != "DELETE"
}
```

**長所**:
- ポリシーをコードから完全に分離し、バージョン管理・CI/CD テストができる
- 複雑なロジックを統一言語で表現できる
- 外部 PDP として複数サービスが共有できる（集中管理）

**短所**:
- OPA / Rego の学習コストがある
- 外部 PDP はネットワーク遅延・SPoF のリスクがある
- 小規模システムではオーバーエンジニアリングになりやすい

---

### 比較表

| 項目 | RBAC | ABAC | ReBAC | PBAC |
|------|------|------|-------|------|
| **表現力** | 低〜中（ロール粒度） | 高（属性条件） | 高（関係グラフ） | 高（宣言的言語） |
| **運用コスト** | 低 | 中 | 高（タプルストア） | 中〜高（OPA等） |
| **監査性** | 高（ロール一覧で把握） | 中（属性組み合わせが複雑） | 中（グラフ可視化が必要） | 高（ポリシーファイルが集約） |
| **スケール** | 中（ロール爆発の懸念） | 中 | 高（Zanzibar 実績） | 高（外部 PDP） |
| **適したユースケース** | 社内業務アプリ | 所有者・機密度制御 | ドライブ・SNS共有 | マイクロサービス横断 |

---

## 3. PEP / PDP / PIP

### 役割の定義

| コンポーネント | 正式名称 | 責務 |
|--------------|----------|------|
| **PEP** | Policy Enforcement Point | リクエストを捕捉し、PDP の判定を強制する。許可なら処理継続、拒否なら 403 を返す |
| **PDP** | Policy Decision Point | ポリシーとリクエストのコンテキストを評価し、allow/deny を決定する |
| **PIP** | Policy Information Point | 判定に必要な属性（ユーザー情報・リソースメタデータ等）を PDP に提供する |

### フロー

```
クライアント
    |
    | HTTP リクエスト
    v
+-------+
|  PEP  |  ← ミドルウェア / ゲートウェイ
+-------+
    |  (sub, obj, act) + 属性取得要求
    v
+-------+     属性問い合わせ     +-------+
|  PDP  | <-------------------> |  PIP  |
+-------+                       +-------+
    |  allow / deny              (DB, IdP, etc.)
    v
+-------+
|  PEP  |  → allow: 後続ハンドラーに委譲
+-------+  → deny:  403 Forbidden を返す
```

### 本モジュールの Casbin デモでの対応

| 役割 | デモでの実体 | 該当コード |
|------|-------------|-----------|
| **PEP** | `requireRBAC` ミドルウェア | `handlers.go: requireRBAC()` |
| **PDP** | `casbin.Enforcer`（`newRBACEnforcer` / `newABACEnforcer`） | `enforcer.go` |
| **PIP（RBAC）** | `policy.csv` + `model.conf`（埋め込みアダプタ） | `enforcer.go: //go:embed` |
| **PIP（ABAC）** | `lookupResource()` が返す `Resource{Owner: ...}` | `handlers.go: lookupResource()` |

**RBAC PEP の例**（`handlers.go`）:

```go
// PEP: X-User ヘッダからサブジェクトを取り出し、PDP(Enforcer)に問い合わせる
func requireRBAC(e *casbin.Enforcer, obj, act string, next http.HandlerFunc) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        sub := r.Header.Get("X-User")
        allowed, err := e.Enforce(sub, obj, act)  // PDP 呼び出し
        if err != nil || !allowed {
            http.Error(w, "アクセス拒否", http.StatusForbidden)
            return
        }
        next(w, r)
    })
}
```

**ABAC PIP の例**（`handlers.go`）:

```go
// PIP: リソース属性（Owner）を取得して PDP に渡す
res := lookupResource(id)                         // PIP 相当
allowed, err := abac.Enforce(sub, res, "edit")    // PDP 呼び出し
```

デモの起動:

```bash
make authz
# → http://localhost:9200
```

---

## 4. 認可をどこに置くか

### レイヤー別の配置

```
インターネット
    |
+-------------------+
|  API ゲートウェイ   |  ← 粗粒度の認可（ルートレベル JWT 検証、IP フィルタ等）
+-------------------+
    |
+-------------------+
|  サービス内 PEP    |  ← ビジネスルール（ロール・所有者チェック）
| （ミドルウェア）    |    ← 本モジュールの requireRBAC はここ
+-------------------+
    |
+-------------------+
|  データ層          |  ← 行レベルセキュリティ（RLS）・クエリフィルタ
| （DB / ORM）       |    例: WHERE owner_id = $user_id
+-------------------+
```

### 集中 PDP vs 組み込み PDP

| 方式 | 特徴 | 向くケース |
|------|------|-----------|
| **組み込み PDP**（Casbin 等） | プロセス内で判定。レイテンシ最小。サービスごとにポリシーを管理 | 単一サービス・小〜中規模 |
| **集中 PDP**（OPA サイドカー / Cerbos サーバー） | ポリシーを一元管理。ネットワーク呼び出しが発生。SPoF のリスクあり | マイクロサービス横断・ポリシー一元管理が必要な場合 |

**集中 PDP のトレードオフ**:
- ポリシー変更を全サービスに即時反映できる（プラス）
- ネットワーク障害時に認可判定が止まる可能性がある（マイナス）
- レイテンシが増加する（マイナス）→ キャッシュで緩和

---

## 5. 段階的進化

システムの成長に合わせて認可モデルを段階的に進化させる指針を示す。

```
RBAC
 |
 | きめ細かい属性条件が必要になったとき
 | 例: 「自分が所有するリソースだけ編集可能」
 | 例: 「機密度 HIGH のドキュメントは特定部署のみ閲覧可能」
 v
ABAC
 |
 | 所有・共有・グループのグラフ権限が必要になったとき
 | 例: 「フォルダを共有すると配下のファイルも権限継承」
 | 例: 「チームのメンバーがチームのリソースにアクセス可能」
 v
ReBAC（OpenFGA / SpiceDB）
```

### 移行を検討するシグナル

| シグナル | 推奨アクション |
|---------|--------------|
| ロール数が 20 を超え、ロールの組み合わせが爆発している | ABAC への移行を検討 |
| 「自分のリソースだけ」「所属チームのリソースだけ」という条件が増えた | ABAC または ReBAC を検討 |
| Google Drive / Notion のような共有・継承モデルが必要になった | ReBAC（OpenFGA 等）を検討 |
| 複数マイクロサービスでポリシーを一元管理したい | PBAC（OPA / Cerbos）を検討 |
| 現状の RBAC で問題なく、ロールが安定している | 移行不要 |

---

## 6. 実装のアンチパターン

### アンチパターン 1: ハードコードされた権限チェックの散在

```go
// NG: ハンドラーの随所に if 文で権限チェックが散在
func deleteDoc(w http.ResponseWriter, r *http.Request) {
    user := r.Header.Get("X-User")
    if user != "alice" && user != "superadmin" { // ← ユーザー名のハードコード
        http.Error(w, "禁止", 403)
        return
    }
    // ...
}
```

**問題**: ロール変更のたびに全ハンドラーを修正する必要がある。テストもしにくい。

**対策**: PEP/PDP を分離し、ポリシーは設定ファイルまたは PDP に集約する。

---

### アンチパターン 2: 認可と業務ロジックの密結合

```go
// NG: 権限チェックとビジネスロジックが混在
func updateOrder(w http.ResponseWriter, r *http.Request) {
    user := r.Context().Value("user").(User)
    order := fetchOrder(r.PathValue("id"))
    if order.Status == "shipped" && user.Role != "admin" { // ← 業務ルールと認可が混在
        http.Error(w, "禁止", 403)
        return
    }
    // 注文更新処理...
}
```

**問題**: 認可条件がビジネスロジックに埋め込まれ、ポリシー変更がロジック全体の修正を誘発する。

**対策**: `enforcer.Enforce(user, order, "update")` のようにポリシーエンジンに委譲する。業務状態（`order.Status`）はリソース属性として PIP から提供する。

---

### アンチパターン 3: "deny 漏れ"（デフォルト許可）

```go
// NG: 条件に一致しなかった場合に暗黙的に許可してしまう
func checkAccess(user, resource string) bool {
    if user == "admin" {
        return true
    }
    if user == "editor" && resource == "/docs" {
        return true
    }
    return true // ← バグ: 全員が通過してしまう
}
```

**問題**: 新しいロールやリソースを追加したとき、明示的な deny がないと誰でもアクセスできてしまう。

**対策**: **deny-by-default**（明示的に許可された場合のみ通過）を徹底する。Casbin の `some(where (p.eft == allow))` は allow が一件もなければ deny になるため、この原則を自然に満たす。

---

### アンチパターン 4: 認可のテスト不足

権限チェックのテストを省略すると、ポリシー変更時のリグレッションを検出できない。

```go
// OK: テーブル駆動でロール×リソース×アクションを網羅的にテスト
func TestRBACPolicy(t *testing.T) {
    e, _ := newRBACEnforcer()
    cases := []struct {
        sub, obj, act string
        want          bool
    }{
        {"alice", "/docs", "DELETE", true},
        {"bob",   "/docs", "DELETE", false}, // editor は DELETE 不可
        {"carol", "/docs", "POST",   false}, // viewer は POST 不可
    }
    for _, c := range cases {
        got, _ := e.Enforce(c.sub, c.obj, c.act)
        if got != c.want {
            t.Errorf("Enforce(%q,%q,%q) = %v, want %v", c.sub, c.obj, c.act, got, c.want)
        }
    }
}
```

本モジュールでは `enforcer_test.go` にこの形式のテストが含まれている。

---

## 7. まとめ

認可設計における核心原則を以下に整理する。

| 原則 | 説明 |
|------|------|
| **デフォルト拒否** | 明示的に許可されていない操作はすべて拒否する。Casbin の `some(where (p.eft == allow))` はこれを自然に実現する |
| **PEP / PDP の分離** | リクエストの捕捉（PEP）と判定（PDP）を分離することで、ポリシー変更をコード修正なしに行える |
| **最小権限の原則** | 主体には業務に必要な最低限の権限のみを付与する。ロール継承でも「最上位ロールをデフォルト付与」は避ける |
| **監査可能性** | すべての allow/deny をログに記録し、「なぜ拒否されたか」を追跡できるようにする |
| **段階的進化** | 最初は RBAC でシンプルに始め、要件が複雑化したら ABAC → ReBAC へ段階的に移行する |

各モデルの実装ライブラリ（Casbin / OPA / OpenFGA / Cerbos / oso）の詳細は [`09_authz_frameworks.md`](./09_authz_frameworks.md) を参照。
