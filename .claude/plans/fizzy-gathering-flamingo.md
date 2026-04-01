# Mastra Workspace Skills が "No skills" になる問題の修正

## Context

`workspace.ts` で Workspace を以下のように設定している:

```ts
new Workspace({
  filesystem: new LocalFilesystem({ basePath: './workspace' }),
  skills: ["./skills"],
  bm25: true,
});
```

`skills: ["./skills"]` は `basePath` (`./workspace`) からの相対パスとして解決されるため、実際に探索されるパスは `./workspace/skills/` になる。しかし `./workspace/` ディレクトリ自体が存在しない。

一方、SKILL.md ファイルは `src/mastra/agents/skills/` に配置されている:
- `src/mastra/agents/skills/summarize/SKILL.md`
- `src/mastra/agents/skills/prioritize/SKILL.md`

## 修正方針

`workspace.ts` のパス設定を修正し、実際の SKILL.md の配置場所を正しく参照するようにする。

### 変更ファイル

**`04_agent/mastra/backend/src/mastra/agents/workspace.ts`**

`basePath` を `./src/mastra/agents` に変更（`mastra dev` の cwd がプロジェクトルートであるため）:

```ts
import { Workspace, LocalFilesystem } from "@mastra/core/workspace";

export const workspace = new Workspace({
  filesystem: new LocalFilesystem({ basePath: './src/mastra/agents' }),
  skills: ["./skills"],
  bm25: true,
});
```

これにより `./src/mastra/agents/skills/` が探索され、`summarize/SKILL.md` と `prioritize/SKILL.md` が検出される。

## 検証

1. `mastra dev` でバックエンドを起動
2. Mastra UI の TaskAgent Overview で Skills セクションに `summarize` と `prioritize` が表示されることを確認
