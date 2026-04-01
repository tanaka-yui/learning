# Plan: 05_antipatterns.md にテーブル定義・サンプルデータ・ER図を追加

## 概要

`05_antipatterns.md` の各アンチパターンセクションに、テーブル定義表・サンプルデータ表・Mermaid ER図を追加する。既存のテキスト（説明文、なぜダメなのか、💡 tips）はすべて保持する。

## 変更対象

単一ファイル: `/Users/yui/Documents/workspace/tanaka-yui/learning/05_database/02/docs/05_antipatterns.md`

## セクション別の変更内容

### 1. ジェイウォーク（信号無視）
- **❌ BAD**: CREATE TABLE の後にテーブル定義表を追加（tags TEXT カラム含む）。サンプルデータ5行（tags に `'旅行,グルメ,写真'` 等のカンマ区切り値）を追加
- **✅ GOOD**: CREATE TABLE の後に hashtags・hashtag_posts のテーブル定義表とサンプルデータを追加。Mermaid ER図を追加（`posts ||--o{ hashtag_posts` / `hashtags ||--o{ hashtag_posts`）

### 2. マルチカラムアトリビュート
- **❌ BAD**: CREATE TABLE の後にテーブル定義表を追加（tag1/tag2/tag3 カラム）。サンプルデータ5行（未使用タグカラムは NULL）を追加

### 3. EAV
- **❌ BAD**: CREATE TABLE の後に user_attributes テーブル定義表を追加。サンプルデータ（key-value 形式の行）を追加
- **✅ GOOD**: CREATE TABLE の後に users テーブル定義表とサンプルデータ（00_schema.md と同じ形式）を追加

### 4. ポリモーフィック関連
- **❌ BAD**: CREATE TABLE の後に reactions テーブル定義表を追加。サンプルデータを追加。Mermaid ER図（reactions が posts と post_replies の両方を指す問題を図示）を追加
- **✅ GOOD**: CREATE TABLE の後に post_favorites・post_stamps テーブル定義表を追加。サンプルデータを追加。Mermaid ER図（正しい分離された関係）を追加

### 5. キーレスエントリ
- **❌ BAD**: CREATE TABLE の後にテーブル定義表を追加（REFERENCES なし）。サンプルデータ（存在しないユーザーIDを含む孤立参照）を追加
- **✅ GOOD**: CREATE TABLE の後にテーブル定義表を追加（REFERENCES・CHECK あり）。Mermaid ER図（正しい FK 関係）を追加

### 6. IDリクワイアド
- **❌ BAD**: CREATE TABLE の後にテーブル定義表を追加（id SERIAL カラム含む）。サンプルデータ（同一 user_id + follow_user_id で異なる id の重複行）を追加
- **✅ GOOD**: CREATE TABLE の後にテーブル定義表を追加（複合主キー）

### 7. インデックスショットガン — 変更なし（SQLの例のまま）
### 8. カーディナリティ — 変更なし（SQLの例のまま）

### 9. ナイーブツリー
- **❌ BAD**: CREATE TABLE の後にテーブル定義表を追加（parent_reply_id 含む）。サンプルデータ（ネストされたスレッド）を追加。Mermaid ER図（自己参照）を追加
- **✅ GOOD**: CREATE TABLE の後にテーブル定義表を追加（フラット構造）

### 10. ソフトデリート
- **❌ BAD**: テーブル定義表を追加（deleted_at カラム含む posts テーブル）。サンプルデータ（削除済みと有効な投稿の混在）を追加

### 11. God Table
- **❌ BAD**: CREATE TABLE の後にテーブル定義表を追加。サンプルデータ（NULL だらけの行）を追加
- **✅ GOOD**: 12テーブル構成への言及のみ（00_schema.md 参照）

### 12. インプリシットカラム — 変更なし
### 13. NULL恐怖症 — 変更なし

## スタイルルール

- テーブル定義: `| カラム名 | 型 | 説明 |` ヘッダー形式
- サンプルデータ: 5行程度、UUID は `01905a3b-...` 形式
- ER図: Mermaid erDiagram、日本語カーディナリティラベル
- 全文日本語
- セクション間の `---` 区切りを保持

## 実装手順

1. ファイル全体を Write で書き換え（1回の操作で完了）
