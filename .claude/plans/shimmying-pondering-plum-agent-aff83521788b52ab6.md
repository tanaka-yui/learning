# Plan: Rewrite 05_antipatterns.md with テーブル定義, サンプルデータ, Mermaid ER diagrams

## Overview
Rewrite `/Users/yui/Documents/workspace/tanaka-yui/learning/05_database/02/docs/05_antipatterns.md` completely using the Write tool in ONE operation.

## What to add per section

For each section with a CREATE TABLE, add BEFORE the SQL block:
1. A **テーブル定義** table (カラム名 | 型 | 説明)
2. **サンプルデータ** showing 3-5 rows with abbreviated UUIDs like `01905a3b-...`
3. **Mermaid ER diagrams** where relationships exist

### Section-by-section plan:

| Section | ❌ additions | ✅ additions |
|---------|-------------|-------------|
| 1 ジェイウォーク | テーブル定義 + サンプルデータ (tags with CSV) | テーブル定義 for hashtags + hashtag_posts, サンプルデータ, Mermaid ER |
| 2 マルチカラムアトリビュート | テーブル定義 + サンプルデータ (tag1/tag2/tag3 with NULLs) | (same solution as 1, keep as-is) |
| 3 EAV | テーブル定義 for user_attributes + サンプルデータ | テーブル定義 for users + サンプルデータ |
| 4 ポリモーフィック関連 | テーブル定義 for reactions + サンプルデータ + Mermaid ER (dashed lines) | テーブル定義 for post_favorites + post_stamps + サンプルデータ + Mermaid ER |
| 5 キーレスエントリ | テーブル定義 for follows WITHOUT references + サンプルデータ (orphaned UUID) | テーブル定義 for follows WITH references + Mermaid ER |
| 6 IDリクワイアド | テーブル定義with unnecessary id + サンプルデータ (duplicate pairs) | テーブル定義 with composite PK |
| 7, 8 | Keep as-is | Keep as-is |
| 9 ナイーブツリー | テーブル定義 with parent_reply_id + サンプルデータ + Mermaid ER (self-ref) | テーブル定義 for flat post_replies |
| 10 ソフトデリート | テーブル定義 with deleted_at + サンプルデータ | (keep ✅ as-is) |
| 11 God Table | テーブル定義 + サンプルデータ (NULL-heavy) | (keep as-is) |
| 12, 13 | Keep as-is | Keep as-is |

## Critical rules
- KEEP all existing text, SQL blocks, なぜダメなのか bullets, 💡 tips
- New テーブル定義 tables go BEFORE the SQL, not replacing it
- All Japanese, Mermaid labels in Japanese
- Use `---` separators between sections
- UUIDs abbreviated as `01905a3b-...`

## Execution
Single Write tool call to completely rewrite the file.
