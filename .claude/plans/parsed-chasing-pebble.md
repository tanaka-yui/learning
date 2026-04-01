# exercise.md の問題と解答を分離する

## Context

exercise.md に問題と解答が同一ファイルにあるため、読者が先に答えを見てしまう。問題と解答を分離し、自分で考えてから答えを見る学習体験にする。

## 対象ファイル

- 修正: `05_database/01/docs/exercise.md` — 問題部分のみ残す（〜Step 0 + 発生する問題）
- 新規: `05_database/01/docs/answer.md` — Step 1〜3 + 解答まとめを移動
- 修正: `05_database/01/README.md` — answer.md へのリンク追加

## 変更内容

### exercise.md
- 問題文 + Step 0（非正規形）+ 発生する問題 をそのまま残す
- 末尾に「解答は [answer.md](answer.md) を参照」のリンクを追加
- Step 1 以降をすべて削除

### answer.md（新規作成）
- ヘッダー: `# 設計演習: スーパーのレジシステム — 解答`
- Step 1〜3 + 解答まとめをそのまま移動（ER図・サンプルデータ含む）

### README.md
- 読む順番に `3. [設計演習の解答](docs/answer.md)` を追加

## 検証

- exercise.md に Step 1 以降が残っていないこと
- answer.md に Step 1〜3 + 解答まとめがすべて含まれていること
- README.md のリンクが正しいこと
