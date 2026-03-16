# パストラバーサル（Path Traversal）

## 1. 概要

パストラバーサルとは、Webアプリケーションがユーザーの入力をファイルパスの構成要素として使用する際に、入力値の検証が不十分であることを悪用し、本来アクセスできないはずのファイルやディレクトリに不正にアクセスする攻撃手法である。

「ディレクトリトラバーサル」とも呼ばれ、攻撃者は `../`（親ディレクトリへの遡上）を含むパスを指定することで、公開ディレクトリの外にあるシステムファイルやソースコード、設定ファイルなどを読み取ることができる。

## 2. 攻撃の仕組み

### `../` によるディレクトリ遡上

ファイルダウンロード機能において、サーバーが以下のようにユーザー入力をそのままファイルパスに結合している場合を考える。

```
リクエスト: GET /download?file=readme.txt
サーバー内部: filepath.Join("files", "readme.txt") → "files/readme.txt"
```

この場合、攻撃者は `../` を使って `files` ディレクトリの外にあるファイルにアクセスできる。

```
リクエスト: GET /download?file=../main.go
サーバー内部: filepath.Join("files", "../main.go") → "main.go"
→ アプリケーションのソースコードが漏洩する

リクエスト: GET /download?file=../../etc/passwd
サーバー内部: filepath.Join("files", "../../etc/passwd") → "../etc/passwd"
→ システムファイルが漏洩する
```

### サブディレクトリへのアクセス

`/` を含むパスを指定することで、公開ディレクトリ内のサブディレクトリにある非公開ファイルにもアクセスできる。

```
リクエスト: GET /download?file=secret/credentials.txt
サーバー内部: filepath.Join("files", "secret/credentials.txt") → "files/secret/credentials.txt"
→ 秘密の認証情報ファイルが漏洩する
```

## 3. デモ環境の起動手順

```bash
make path-traversal
```

このコマンドで以下のサービスが起動する。

| サービス | URL | 説明 |
|---------|-----|------|
| 脆弱版バックエンド | http://localhost:8088 | パス検証なし、トラバーサル攻撃に脆弱 |
| 対策版バックエンド | http://localhost:8089 | filepath.Base、HasPrefixによるパス検証あり |
| 脆弱版フロントエンド | http://localhost:3008 | 脆弱性を確認できるデモ画面 |
| 対策版フロントエンド | http://localhost:3009 | 対策済みのデモ画面 |

## 4. 攻撃手順

### ステップ1: 正常なファイルダウンロードを確認する

1. ブラウザで http://localhost:3008 を開く
2. ファイル一覧に表示される `readme.txt` または `report.csv` をクリックする
3. ファイル内容が正常に表示されることを確認する

### ステップ2: ディレクトリトラバーサルでソースコードを取得する

1. 手動入力欄に `../main.go` を入力して「ダウンロード」を押す
2. サーバーのソースコード（`package main` で始まるGoコード）が表示される
3. ステータスコード200が返ることを確認する

### ステップ3: システムファイルの取得を試みる

1. 手動入力欄に `../../etc/passwd` を入力して「ダウンロード」を押す
2. Docker環境内の `/etc/passwd` ファイルの内容が表示される

### ステップ4: サブディレクトリの秘密ファイルを取得する

1. 手動入力欄に `secret/credentials.txt` を入力して「ダウンロード」を押す
2. データベースのパスワードやAPIキーなどの機密情報が表示される

### ステップ5: 対策版で同じ攻撃を試す

1. ブラウザで http://localhost:3009 を開く
2. 上記と同じ攻撃ペイロードを入力する
3. すべてのトラバーサル攻撃がステータスコード400でブロックされることを確認する

## 5. 脆弱なコード解説

脆弱版のダウンロードハンドラーでは、ユーザー入力をそのままファイルパスに結合しており、パスの検証を一切行っていない。

```go
// 【脆弱性】ユーザー入力をそのまま結合しており、パスの検証を行っていない
// 攻撃者は "../" を使ってfilesディレクトリの外にあるファイルを読み取ることができる
targetPath := filepath.Join(filesDir, filename)

data, err := os.ReadFile(targetPath)
```

`filepath.Join` は `../` を含むパスを正規化するが、結果として `filesDir` の外を指すパスになることを防止する機能はない。そのため、`filepath.Join("files", "../main.go")` は `main.go` に解決され、アプリケーションのソースコードが読み取れてしまう。

## 6. 対策コード解説

対策版では3つの防御層を実装している。

### 対策1: 危険な文字の拒否

ファイル名に `..`、`/`、`\` が含まれている場合は即座に拒否する。

```go
// 対策1: ".." や "/" や "\" を含むファイル名を拒否する
if strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
    http.Error(w, "不正なファイル名です", http.StatusBadRequest)
    return
}
```

### 対策2: filepath.Base によるファイル名抽出

`filepath.Base()` を使ってパスからファイル名部分のみを抽出し、ディレクトリ要素を完全に除去する。

```go
// 対策2: filepath.Base() でファイル名のみを抽出する（ディレクトリ要素を除去）
safeName := filepath.Base(filename)
if safeName == "." || safeName == ".." {
    http.Error(w, "不正なファイル名です", http.StatusBadRequest)
    return
}
```

### 対策3: 絶対パスによるディレクトリ境界の検証

`filepath.Abs` で絶対パスを解決し、`strings.HasPrefix` でベースディレクトリ内のファイルであることを検証する。

```go
// 対策3: 絶対パスを解決し、ベースディレクトリ内であることを検証する
baseDir, err := filepath.Abs(filesDir)
if err != nil {
    http.Error(w, "サーバー内部エラー", http.StatusInternalServerError)
    return
}

targetPath, err := filepath.Abs(filepath.Join(filesDir, safeName))
if err != nil {
    http.Error(w, "サーバー内部エラー", http.StatusInternalServerError)
    return
}

if !strings.HasPrefix(targetPath, baseDir+string(os.PathSeparator)) {
    http.Error(w, "不正なファイルパスです", http.StatusBadRequest)
    return
}
```

`baseDir + string(os.PathSeparator)` と比較することで、`files2/` のような名前のディレクトリへの誤マッチを防止している。

## 7. まとめ

パストラバーサル攻撃に対するベストプラクティスは以下の通りである。

- **ユーザー入力をファイルパスに直接使用しない**: ファイル名のホワイトリストやIDマッピングを使用し、ユーザーが直接パスを指定できないようにする
- **filepath.Base による正規化**: ディレクトリ要素を除去し、ファイル名のみを使用する
- **危険な文字の拒否**: `..`、`/`、`\` などのディレクトリ遡上に使用される文字を拒否する
- **絶対パスによる境界検証**: `filepath.Abs` と `strings.HasPrefix` を組み合わせて、最終的なファイルパスが許可されたディレクトリ内に収まっていることを検証する
- **最小権限の原則**: アプリケーションの実行ユーザーに必要最小限のファイルアクセス権限のみを付与する
- **chroot やコンテナによる隔離**: ファイルシステムのアクセス範囲をOSレベルで制限する
