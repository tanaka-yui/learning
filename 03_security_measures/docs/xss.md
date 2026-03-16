# XSS(クロスサイトスクリプティング)

## 1. 概要

XSS(Cross-Site Scripting)は、Webアプリケーションの脆弱性を利用して、悪意のあるスクリプトを他のユーザーのブラウザで実行させる攻撃手法である。

### XSSの種類

| 種類 | 説明 |
|------|------|
| **Stored XSS(格納型)** | 悪意のあるスクリプトがサーバー側に保存され、他のユーザーがページを閲覧するたびに実行される。掲示板やコメント欄が典型的な攻撃対象。本デモではこの種類を扱う。 |
| **Reflected XSS(反射型)** | スクリプトがURLのパラメータなどに含まれ、サーバーからのレスポンスにそのまま反映されて実行される。フィッシングメールなどで悪意のあるリンクを送りつける手口が多い。 |
| **DOM-based XSS** | サーバーを経由せず、クライアント側のJavaScriptがDOMを操作する際に発生する。`document.location`や`innerHTML`などを安全でない方法で使用することが原因。 |

## 2. 攻撃の仕組み

Stored XSSの攻撃フローは以下の通りである。

1. 攻撃者が掲示板にスクリプトを含む投稿を送信する
2. サーバーがスクリプトをサニタイズせずにそのまま保存する
3. 被害者がその掲示板ページを閲覧する
4. サーバーが保存済みのスクリプトを含む投稿データを返す
5. ブラウザがそのスクリプトを正規のコンテンツとして実行する
6. 攻撃者のスクリプトにより、Cookie窃取やセッションハイジャックが行われる

## 3. デモ環境の起動手順

```bash
make xss
```

このコマンドで以下のサービスが起動する。

| サービス | URL | 説明 |
|---------|-----|------|
| 脆弱版バックエンド | http://localhost:8082 | サニタイズなしのAPI |
| 対策版バックエンド | http://localhost:8083 | エスケープ処理+CSPヘッダー付きのAPI |
| 脆弱版フロントエンド | http://localhost:3002 | `dangerouslySetInnerHTML`で描画 |
| 対策版フロントエンド | http://localhost:3003 | テキストノードで安全に描画 |

## 4. 攻撃手順

### ステップ1: スクリプトタグによる攻撃

1. ブラウザで http://localhost:3002 を開く
2. テキストエリアに以下を入力して「投稿」をクリックする

```html
<script>alert('XSS')</script>
```

> 注意: `<script>` タグはブラウザの仕様上、`innerHTML` での挿入時には実行されないことがある。次のステップのイベントハンドラが確実に動作する。

### ステップ2: イベントハンドラによる攻撃

1. テキストエリアに以下を入力して「投稿」をクリックする

```html
<img onerror="alert('XSS')" src="x">
```

2. 画像の読み込みに失敗し、`onerror` イベントが発火してアラートが表示される

### ステップ3: JavaScript URLによる攻撃

1. テキストエリアに以下を入力して「投稿」をクリックする

```html
<a href="javascript:alert('XSS')">Click</a>
```

2. 表示されたリンクをクリックすると、JavaScriptが実行される

### ステップ4: 対策版での確認

1. ブラウザで http://localhost:3003 を開く
2. 上記と同じ攻撃ペイロードを投稿する
3. スクリプトが実行されず、入力したHTML文字列がそのままテキストとして表示されることを確認する

## 5. 脆弱なコード解説

### バックエンド(Go)

脆弱版のバックエンドでは、ユーザーの入力をサニタイズせずにそのまま保存している。

```go
// 投稿を追加する（サニタイズなし - 意図的に脆弱）
func (s *PostStore) addPost(content string) Post {
	s.mu.Lock()
	defer s.mu.Unlock()

	post := Post{
		ID:        s.nextID,
		Content:   content, // サニタイズなし - 意図的に脆弱
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	s.nextID++
	s.posts = append(s.posts, post)
	return post
}
```

さらに、JSONレスポンスでもHTMLエスケープを無効化している。

```go
// HTMLエスケープを無効化し、スクリプトタグ等をそのまま返す（意図的に脆弱）
encoder := json.NewEncoder(w)
encoder.SetEscapeHTML(false)
encoder.Encode(posts)
```

### フロントエンド(React)

脆弱版のフロントエンドでは、`dangerouslySetInnerHTML` を使って投稿内容をHTMLとしてそのまま描画している。

```tsx
<div
  className="post-content"
  dangerouslySetInnerHTML={{ __html: post.content }}
/>
```

これにより、投稿に含まれるHTMLタグやスクリプトがブラウザでそのまま解釈・実行される。

## 6. 対策コード解説

### バックエンド(Go)

対策版では、`html.EscapeString` を使って保存前にHTMLエスケープを適用している。

```go
// 投稿を追加する（html.EscapeStringでサニタイズ済み）
func (s *PostStore) addPost(content string) Post {
	s.mu.Lock()
	defer s.mu.Unlock()

	post := Post{
		ID:        s.nextID,
		Content:   html.EscapeString(content), // 保存前にHTMLエスケープを適用する
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	s.nextID++
	s.posts = append(s.posts, post)
	return post
}
```

さらに、セキュリティヘッダーとしてCSP(Content Security Policy)を設定している。

```go
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}
```

`Content-Security-Policy: default-src 'self'` により、外部リソースの読み込みやインラインスクリプトの実行がブラウザレベルでブロックされる。

### フロントエンド(React)

対策版では、`dangerouslySetInnerHTML` を使わず、Reactのテキストノード描画(JSX内の `{}`)で投稿内容を表示している。

```tsx
<p className="post-content">{post.content}</p>
```

Reactはデフォルトでテキストをエスケープしてから描画するため、HTMLタグはブラウザで解釈されず、文字列としてそのまま表示される。

## 7. まとめ

XSS対策のベストプラクティスは以下の通りである。

- **入力のサニタイズ**: サーバー側で `html.EscapeString` 等を使い、保存前にHTMLエスケープを適用する
- **出力のエスケープ**: フロントエンドでは `dangerouslySetInnerHTML` を避け、テキストノードとして描画する
- **CSPヘッダーの設定**: `Content-Security-Policy` ヘッダーでインラインスクリプトの実行を制限する
- **X-Content-Type-Options**: `nosniff` を設定して、MIMEタイプスニッフィングを防止する
- **多層防御**: バックエンド・フロントエンド・ブラウザの各レイヤーで対策を実施し、単一の防御層に依存しない
