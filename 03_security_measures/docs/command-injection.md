# コマンドインジェクション

## 1. 概要

コマンドインジェクションとは、アプリケーションがユーザー入力をOSコマンドの一部として使用する際に、攻撃者が意図しないコマンドを挿入・実行させる攻撃手法である。サーバー上で任意のコマンドが実行されるため、ファイルの読み取り・改ざん、システム情報の取得、さらにはサーバーの完全な乗っ取りにまで発展する可能性がある。

## 2. 攻撃の仕組み

### シェル経由のコマンド実行

アプリケーションがユーザー入力を含む文字列をシェル（`sh -c`）に渡して実行する場合、シェルのメタ文字を利用して追加のコマンドを挿入できる。

### メタ文字による連結

| メタ文字 | 意味 | 例 |
|---------|------|-----|
| `;` | コマンドの区切り | `localhost; cat /etc/passwd` |
| `&&` | 前のコマンドが成功したら次を実行 | `localhost && ls /` |
| `\|` | パイプ（前のコマンドの出力を次に渡す） | `localhost \| echo HACKED` |
| `` ` `` | バッククォート内のコマンドを実行 | `` localhost`whoami` `` |
| `$()` | コマンド置換 | `localhost$(whoami)` |

## 3. デモ起動手順

```bash
cd 03_security_measures
make command-injection
```

起動後、以下のURLにアクセスする:

- 脆弱版フロントエンド: http://localhost:3010
- 対策版フロントエンド: http://localhost:3011
- 脆弱版バックエンドAPI: http://localhost:8090
- 対策版バックエンドAPI: http://localhost:8091

停止する場合:

```bash
make down
```

## 4. 攻撃手順

### ステップ1: 正常な操作を確認

1. 脆弱版フロントエンド（http://localhost:3010）にアクセスする
2. ホスト名に `localhost` を入力し、「DNS Lookup」ボタンを押す
3. nslookupの結果が正常に表示されることを確認する

### ステップ2: コマンドインジェクションを実行

1. ホスト名に `localhost; cat /etc/passwd` を入力する
2. 「DNS Lookup」ボタンを押す
3. nslookupの結果に加えて、`/etc/passwd` の内容が表示される

### ステップ3: 他の攻撃パターンを試す

- `localhost; whoami` -- 実行ユーザーの確認
- `localhost && ls /` -- ルートディレクトリの一覧取得
- `localhost | echo HACKED` -- パイプによる出力の差し替え

### ステップ4: 対策版で同じ攻撃を試す

1. 対策版フロントエンド（http://localhost:3011）にアクセスする
2. 同じ攻撃ペイロードを入力する
3. 全てのインジェクション試行が「不正なホスト名です」というエラーでブロックされることを確認する

## 5. 脆弱コード解説

脆弱版（`command-injection/vulnerable/backend/main.go`）では、ユーザー入力をシェル経由でそのまま実行している:

```go
// 脆弱なコマンド実行: ユーザー入力をシェル経由で直接実行
cmd := exec.Command("sh", "-c", "nslookup "+host)
```

`exec.Command("sh", "-c", ...)` は文字列全体をシェルに渡す。シェルは `;`、`&&`、`|` などのメタ文字を解釈するため、`host` に `localhost; cat /etc/passwd` が渡されると:

```
sh -c "nslookup localhost; cat /etc/passwd"
```

として実行される。シェルはこれを2つの独立したコマンドとして解釈し、`nslookup localhost` の後に `cat /etc/passwd` を実行する。

## 6. 対策コード解説

対策版（`command-injection/secure/backend/main.go`）では、以下の2つの対策を実施している。

### 対策1: 入力バリデーション（allowlist方式）

```go
var validHostPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

func validateHost(host string) bool {
    return validHostPattern.MatchString(host)
}
```

正規表現 `^[a-zA-Z0-9._-]+$` により、英数字、ドット、ハイフン、アンダースコアのみを許可する。`;`、`&&`、`|`、スペースなどのメタ文字を含む入力は全て拒否される。

### 対策2: 引数分離（シェルを経由しない）

```go
// 安全なコマンド実行: シェルを経由せず、引数を分離して渡す
cmd := exec.Command("nslookup", host)
cmd := exec.Command("ping", "-c", "1", host)
```

`exec.Command` にコマンドと引数を分離して渡すことで、シェルを経由せずに直接コマンドを実行する。`host` は単なる引数として扱われ、メタ文字が含まれていてもシェルによる解釈は行われない。

**重要**: `exec.Command("sh", "-c", "nslookup "+host)` のようにシェルを経由する方式は絶対に使用してはならない。

## 7. まとめ

| 項目 | 脆弱版 | 対策版 |
|------|--------|--------|
| コマンド実行方式 | `sh -c` でシェル経由 | `exec.Command` で引数分離 |
| 入力バリデーション | なし | 正規表現によるallowlist |
| メタ文字の扱い | シェルが解釈・実行 | 引数として無視される |
| インジェクション | 成功する | 400 Bad Requestで拒否 |

コマンドインジェクション対策の原則:

1. **シェルを経由しない**: `exec.Command` でコマンドと引数を分離して渡す
2. **入力バリデーション**: allowlist方式で許可された文字のみを受け入れる
3. **最小権限の原則**: アプリケーションの実行ユーザーの権限を最小限にする
4. **OSコマンドの使用を避ける**: 可能であれば、OSコマンドの代わりにライブラリやAPIを使用する
