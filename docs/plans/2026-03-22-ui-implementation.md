# UI追加 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 5つの Agent フレームワーク実装それぞれに `backend/`（既存コード移動）と `frontend/`（React + Vite チャット UI）を追加する。

**Architecture:** 各フレームワークのディレクトリを `backend/` と `frontend/` に分割。mastra の frontend は `@mastra/client-js` で Mastra 組み込み API を直接呼ぶ。他4つは plain `fetch` で各 `/chat` エンドポイントを呼ぶ。全 frontend は Docker Compose に含め、`make mastra` で backend + frontend が同時起動する。

**Tech Stack:** React 18 / TypeScript / Vite 6 / `@mastra/client-js`（mastra のみ） / `@fastify/cors` / FastAPI CORSMiddleware / Docker Compose

---

## Task 1: mastra を backend/ に移動

**Files:**
- Modify: `04_agent/mastra/` → `04_agent/mastra/backend/`

**Step 1: backend/ ディレクトリを作成して既存ファイルを移動する**

```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning/04_agent

mkdir mastra/backend
git mv mastra/src mastra/backend/src
git mv mastra/Dockerfile mastra/backend/Dockerfile
git mv mastra/package.json mastra/backend/package.json
git mv mastra/pnpm-lock.yaml mastra/backend/pnpm-lock.yaml
git mv mastra/tsconfig.json mastra/backend/tsconfig.json
```

**Step 2: node_modules を再インストールする**

```bash
cd mastra/backend && pnpm install
```

**Step 3: コミット**

```bash
git add -A
git commit -m "refactor(mastra): move existing code to backend/ subdirectory"
```

---

## Task 2: mastra-fastify を backend/ に移動

**Step 1: ファイルを移動する**

```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning/04_agent

mkdir mastra-fastify/backend
git mv mastra-fastify/src mastra-fastify/backend/src
git mv mastra-fastify/Dockerfile mastra-fastify/backend/Dockerfile
git mv mastra-fastify/package.json mastra-fastify/backend/package.json
git mv mastra-fastify/pnpm-lock.yaml mastra-fastify/backend/pnpm-lock.yaml
git mv mastra-fastify/tsconfig.json mastra-fastify/backend/tsconfig.json
```

**Step 2: node_modules を再インストールする**

```bash
cd mastra-fastify/backend && pnpm install
```

**Step 3: コミット**

```bash
git add -A
git commit -m "refactor(mastra-fastify): move existing code to backend/ subdirectory"
```

---

## Task 3: strands-python を backend/ に移動

**Step 1: ファイルを移動する**

```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning/04_agent

mkdir strands-python/backend
git mv strands-python/agent.py strands-python/backend/agent.py
git mv strands-python/main.py strands-python/backend/main.py
git mv strands-python/skills strands-python/backend/skills
git mv strands-python/tools strands-python/backend/tools
git mv strands-python/tests strands-python/backend/tests
git mv strands-python/Dockerfile strands-python/backend/Dockerfile
git mv strands-python/pyproject.toml strands-python/backend/pyproject.toml
git mv strands-python/uv.lock strands-python/backend/uv.lock
```

**Step 2: .venv を backend/ に再作成する**

```bash
cd strands-python/backend && uv sync
```

**Step 3: コミット**

```bash
git add -A
git commit -m "refactor(strands-python): move existing code to backend/ subdirectory"
```

---

## Task 4: strands-typescript を backend/ に移動

**Step 1: ファイルを移動する**

```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning/04_agent

mkdir strands-typescript/backend
git mv strands-typescript/src strands-typescript/backend/src
git mv strands-typescript/Dockerfile strands-typescript/backend/Dockerfile
git mv strands-typescript/package.json strands-typescript/backend/package.json
git mv strands-typescript/pnpm-lock.yaml strands-typescript/backend/pnpm-lock.yaml
git mv strands-typescript/tsconfig.json strands-typescript/backend/tsconfig.json
```

**Step 2: node_modules を再インストールする**

```bash
cd strands-typescript/backend && pnpm install
```

**Step 3: コミット**

```bash
git add -A
git commit -m "refactor(strands-typescript): move existing code to backend/ subdirectory"
```

---

## Task 5: claude-agent-sdk を backend/ に移動

**Step 1: ファイルを移動する**

```bash
cd /Users/yui/Documents/workspace/tanaka-yui/learning/04_agent

mkdir claude-agent-sdk/backend
git mv claude-agent-sdk/src claude-agent-sdk/backend/src
git mv claude-agent-sdk/Dockerfile claude-agent-sdk/backend/Dockerfile
git mv claude-agent-sdk/package.json claude-agent-sdk/backend/package.json
git mv claude-agent-sdk/pnpm-lock.yaml claude-agent-sdk/backend/pnpm-lock.yaml
git mv claude-agent-sdk/tsconfig.json claude-agent-sdk/backend/tsconfig.json
```

**Step 2: node_modules を再インストールする**

```bash
cd claude-agent-sdk/backend && pnpm install
```

**Step 3: コミット**

```bash
git add -A
git commit -m "refactor(claude-agent-sdk): move existing code to backend/ subdirectory"
```

---

## Task 6: docker-compose.yml の build パスを更新

**Files:**
- Modify: `04_agent/docker-compose.yml`

**Step 1: 各サービスの build パスを backend/ に変更する**

`build: ./mastra` → `build: ./mastra/backend` に変更（全5フレームワーク）。

```yaml
# 04_agent/docker-compose.yml（変更後）
services:
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 5

  postgres:
    image: postgres:16-alpine
    ports:
      - "5432:5432"
    environment:
      - POSTGRES_DB=mastra
      - POSTGRES_USER=postgres
      - POSTGRES_PASSWORD=postgres
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 5s
      timeout: 3s
      retries: 5
    profiles: [mastra]

  mastra:
    build: ./mastra/backend
    ports:
      - "4001:4001"
    environment:
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
      - DATABASE_URL=postgresql://postgres:postgres@postgres:5432/mastra
    depends_on:
      postgres:
        condition: service_healthy
    profiles: [mastra]

  mastra-fastify:
    build: ./mastra-fastify/backend
    ports:
      - "4002:4002"
    environment:
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
      - REDIS_URL=redis://redis:6379
    depends_on:
      redis:
        condition: service_healthy
    profiles: [mastra-fastify]

  strands-python:
    build: ./strands-python/backend
    ports:
      - "4003:4003"
    environment:
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
      - REDIS_URL=redis://redis:6379
    depends_on:
      redis:
        condition: service_healthy
    profiles: [strands-python]

  strands-typescript:
    build: ./strands-typescript/backend
    ports:
      - "4004:4004"
    environment:
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
      - REDIS_URL=redis://redis:6379
    depends_on:
      redis:
        condition: service_healthy
    profiles: [strands-typescript]

  claude-agent-sdk:
    build: ./claude-agent-sdk/backend
    ports:
      - "4005:4005"
    environment:
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
    profiles: [claude-agent-sdk]
```

**Step 2: コミット**

```bash
git add 04_agent/docker-compose.yml
git commit -m "fix(docker-compose): update build paths to backend/ subdirectories"
```

---

## Task 7: 各バックエンドに CORS を追加

**Files:**
- Modify: `04_agent/mastra/backend/src/mastra.ts`（または `agent.ts`）
- Modify: `04_agent/mastra-fastify/backend/src/index.ts`
- Modify: `04_agent/strands-python/backend/main.py`
- Modify: `04_agent/strands-typescript/backend/src/index.ts`
- Modify: `04_agent/claude-agent-sdk/backend/src/index.ts`

### mastra（Hono 組み込みサーバー）

`mastra.ts` の `Mastra` コンストラクタの `server` に `cors` オプションを追加する。

```typescript
export const mastra = new Mastra({
  agents: { taskAgent },
  server: {
    host: "0.0.0.0",
    cors: {
      origin: "*",
      allowMethods: ["GET", "POST", "OPTIONS"],
      allowHeaders: ["Content-Type"],
    },
    apiRoutes: [ /* 既存のまま */ ],
  },
});
```

> **注意:** mastra の `src/index.ts` は現在 Fastify を使っているが、`mastra.ts` への移行が前提。もし `index.ts` が残っている場合は先に `mastra.ts` を作成して `index.ts` を削除する。

### mastra-fastify / strands-typescript / claude-agent-sdk（Fastify）

```bash
cd 04_agent/mastra-fastify/backend && pnpm add @fastify/cors
cd 04_agent/strands-typescript/backend && pnpm add @fastify/cors
cd 04_agent/claude-agent-sdk/backend && pnpm add @fastify/cors
```

各 `src/index.ts` の先頭に追加：

```typescript
import cors from "@fastify/cors";

// app を作成した直後に登録
await app.register(cors, { origin: true });
```

### strands-python（FastAPI）

`backend/main.py` の FastAPI アプリ初期化直後に追加：

```python
from fastapi.middleware.cors import CORSMiddleware

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_methods=["*"],
    allow_headers=["*"],
)
```

**Step: コミット**

```bash
git add -A
git commit -m "feat: add CORS support to all backends"
```

---

## Task 8: 共通 frontend テンプレートを理解する

以下の構造を各 frontend に作成する（Task 9〜13 で使用）。

### package.json（mastra 以外の共通）

```json
{
  "name": "agent-{framework}-frontend",
  "version": "1.0.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build"
  },
  "dependencies": {
    "react": "^18.3.1",
    "react-dom": "^18.3.1",
    "uuid": "^11.1.0"
  },
  "devDependencies": {
    "@types/react": "^18.3.12",
    "@types/react-dom": "^18.3.1",
    "@types/uuid": "^10.0.0",
    "@vitejs/plugin-react": "^4.3.4",
    "typescript": "^5.8.3",
    "vite": "^6.3.5"
  }
}
```

### tsconfig.json（全 frontend 共通）

```json
{
  "compilerOptions": {
    "target": "ES2020",
    "useDefineForClassFields": true,
    "lib": ["ES2020", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "isolatedModules": true,
    "moduleDetection": "force",
    "noEmit": true,
    "jsx": "react-jsx",
    "strict": true
  },
  "include": ["src"]
}
```

### index.html（全 frontend 共通）

```html
<!doctype html>
<html lang="ja">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Agent Chat</title>
    <style>* { box-sizing: border-box; margin: 0; padding: 0; }</style>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

### src/main.tsx（全 frontend 共通）

```tsx
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import App from "./App.tsx";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>
);
```

### App.tsx（mastra 以外の共通テンプレート）

環境変数:
- `VITE_BACKEND_URL` — バックエンドURL（例: `http://localhost:4002`）
- `VITE_THEME_COLOR` — ヘッダーカラー
- `VITE_FRAMEWORK_NAME` — フレームワーク名

```tsx
import { useState, useRef, useEffect } from "react";
import { v4 as uuidv4 } from "uuid";

const BACKEND_URL = import.meta.env.VITE_BACKEND_URL as string;
const THEME_COLOR = (import.meta.env.VITE_THEME_COLOR as string) || "#6366f1";
const FRAMEWORK_NAME = (import.meta.env.VITE_FRAMEWORK_NAME as string) || "Agent";
const SESSION_ID = uuidv4();

type Message = { role: "user" | "assistant"; content: string };

export default function App() {
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState("");
  const [loading, setLoading] = useState(false);
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  const send = async () => {
    if (!input.trim() || loading) return;
    const userMsg = input.trim();
    setInput("");
    setMessages((prev) => [...prev, { role: "user", content: userMsg }]);
    setLoading(true);
    try {
      const res = await fetch(`${BACKEND_URL}/chat`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ message: userMsg, sessionId: SESSION_ID }),
      });
      const data = await res.json();
      setMessages((prev) => [...prev, { role: "assistant", content: data.response }]);
    } catch {
      setMessages((prev) => [...prev, { role: "assistant", content: "エラーが発生しました。" }]);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ height: "100vh", display: "flex", flexDirection: "column", fontFamily: "sans-serif" }}>
      <header style={{ background: THEME_COLOR, color: "#fff", padding: "12px 16px" }}>
        <h1 style={{ fontSize: "18px" }}>{FRAMEWORK_NAME} Chat</h1>
        <p style={{ fontSize: "11px", opacity: 0.7, marginTop: "2px" }}>Session: {SESSION_ID}</p>
      </header>
      <div style={{ flex: 1, overflow: "auto", padding: "16px", display: "flex", flexDirection: "column", gap: "8px" }}>
        {messages.map((m, i) => (
          <div key={i} style={{ alignSelf: m.role === "user" ? "flex-end" : "flex-start", maxWidth: "70%" }}>
            <div
              style={{
                background: m.role === "user" ? THEME_COLOR : "#f3f4f6",
                color: m.role === "user" ? "#fff" : "#111",
                padding: "8px 12px",
                borderRadius: "12px",
                whiteSpace: "pre-wrap",
                fontSize: "14px",
              }}
            >
              {m.content}
            </div>
          </div>
        ))}
        {loading && <div style={{ alignSelf: "flex-start", color: "#9ca3af", fontSize: "14px" }}>...</div>}
        <div ref={bottomRef} />
      </div>
      <div style={{ padding: "12px 16px", borderTop: "1px solid #e5e7eb", display: "flex", gap: "8px" }}>
        <input
          style={{ flex: 1, padding: "8px 12px", borderRadius: "8px", border: "1px solid #d1d5db", fontSize: "14px" }}
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && !e.shiftKey && send()}
          placeholder="メッセージを入力..."
          disabled={loading}
        />
        <button
          onClick={send}
          disabled={loading || !input.trim()}
          style={{
            padding: "8px 16px",
            background: THEME_COLOR,
            color: "#fff",
            border: "none",
            borderRadius: "8px",
            cursor: loading || !input.trim() ? "not-allowed" : "pointer",
            opacity: loading || !input.trim() ? 0.5 : 1,
            fontSize: "14px",
          }}
        >
          送信
        </button>
      </div>
    </div>
  );
}
```

### Dockerfile（全 frontend 共通パターン、ポートのみ異なる）

```dockerfile
FROM node:24-alpine
WORKDIR /app
RUN corepack enable && corepack prepare pnpm@latest --activate
COPY package.json pnpm-lock.yaml* ./
RUN pnpm install --frozen-lockfile
COPY . .
EXPOSE {PORT}
CMD ["pnpm", "dev", "--host", "0.0.0.0", "--port", "{PORT}"]
```

---

## Task 9: mastra frontend 実装

**Files:**
- Create: `04_agent/mastra/frontend/package.json`
- Create: `04_agent/mastra/frontend/tsconfig.json`
- Create: `04_agent/mastra/frontend/index.html`
- Create: `04_agent/mastra/frontend/vite.config.ts`
- Create: `04_agent/mastra/frontend/src/main.tsx`
- Create: `04_agent/mastra/frontend/src/App.tsx`
- Create: `04_agent/mastra/frontend/Dockerfile`

**Step 1: package.json を作成する**

mastra は `@mastra/client-js` を使うため追加する。

```json
{
  "name": "agent-mastra-frontend",
  "version": "1.0.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build"
  },
  "dependencies": {
    "@mastra/client-js": "latest",
    "react": "^18.3.1",
    "react-dom": "^18.3.1",
    "uuid": "^11.1.0"
  },
  "devDependencies": {
    "@types/react": "^18.3.12",
    "@types/react-dom": "^18.3.1",
    "@types/uuid": "^10.0.0",
    "@vitejs/plugin-react": "^4.3.4",
    "typescript": "^5.8.3",
    "vite": "^6.3.5"
  }
}
```

**Step 2: tsconfig.json、index.html、src/main.tsx を作成する**

Task 8 の共通テンプレートをそのまま使用する。

**Step 3: vite.config.ts を作成する**

```typescript
// 04_agent/mastra/frontend/vite.config.ts
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    host: "0.0.0.0",
    port: 5001,
  },
});
```

**Step 4: App.tsx を作成する（@mastra/client-js を使用）**

mastra のフロントエンドは `@mastra/client-js` で Mastra 組み込みサーバーの Agent API を直接呼ぶ。
カスタム `/chat` ルートは使わない。

```tsx
// 04_agent/mastra/frontend/src/App.tsx
import { useState, useRef, useEffect } from "react";
import { MastraClient } from "@mastra/client-js";
import { v4 as uuidv4 } from "uuid";

const BACKEND_URL = (import.meta.env.VITE_BACKEND_URL as string) || "http://localhost:4001";
const THEME_COLOR = "#6366f1";
const FRAMEWORK_NAME = "mastra";
const SESSION_ID = uuidv4();

const client = new MastraClient({ baseUrl: BACKEND_URL });

type Message = { role: "user" | "assistant"; content: string };

export default function App() {
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState("");
  const [loading, setLoading] = useState(false);
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  const send = async () => {
    if (!input.trim() || loading) return;
    const userMsg = input.trim();
    setInput("");
    setMessages((prev) => [...prev, { role: "user", content: userMsg }]);
    setLoading(true);
    try {
      const agent = client.getAgent("task-agent");
      const result = await agent.generate({
        messages: [{ role: "user", content: userMsg }],
        resourceId: "default-user",
        threadId: SESSION_ID,
      });
      const text = result.text ?? "";
      setMessages((prev) => [...prev, { role: "assistant", content: text }]);
    } catch {
      setMessages((prev) => [...prev, { role: "assistant", content: "エラーが発生しました。" }]);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ height: "100vh", display: "flex", flexDirection: "column", fontFamily: "sans-serif" }}>
      <header style={{ background: THEME_COLOR, color: "#fff", padding: "12px 16px" }}>
        <h1 style={{ fontSize: "18px" }}>{FRAMEWORK_NAME} Chat</h1>
        <p style={{ fontSize: "11px", opacity: 0.7, marginTop: "2px" }}>Session: {SESSION_ID}</p>
      </header>
      <div style={{ flex: 1, overflow: "auto", padding: "16px", display: "flex", flexDirection: "column", gap: "8px" }}>
        {messages.map((m, i) => (
          <div key={i} style={{ alignSelf: m.role === "user" ? "flex-end" : "flex-start", maxWidth: "70%" }}>
            <div
              style={{
                background: m.role === "user" ? THEME_COLOR : "#f3f4f6",
                color: m.role === "user" ? "#fff" : "#111",
                padding: "8px 12px",
                borderRadius: "12px",
                whiteSpace: "pre-wrap",
                fontSize: "14px",
              }}
            >
              {m.content}
            </div>
          </div>
        ))}
        {loading && <div style={{ alignSelf: "flex-start", color: "#9ca3af", fontSize: "14px" }}>...</div>}
        <div ref={bottomRef} />
      </div>
      <div style={{ padding: "12px 16px", borderTop: "1px solid #e5e7eb", display: "flex", gap: "8px" }}>
        <input
          style={{ flex: 1, padding: "8px 12px", borderRadius: "8px", border: "1px solid #d1d5db", fontSize: "14px" }}
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && !e.shiftKey && send()}
          placeholder="メッセージを入力..."
          disabled={loading}
        />
        <button
          onClick={send}
          disabled={loading || !input.trim()}
          style={{
            padding: "8px 16px",
            background: THEME_COLOR,
            color: "#fff",
            border: "none",
            borderRadius: "8px",
            cursor: loading || !input.trim() ? "not-allowed" : "pointer",
            opacity: loading || !input.trim() ? 0.5 : 1,
            fontSize: "14px",
          }}
        >
          送信
        </button>
      </div>
    </div>
  );
}
```

**Step 5: Dockerfile を作成する**

```dockerfile
# 04_agent/mastra/frontend/Dockerfile
FROM node:24-alpine
WORKDIR /app
RUN corepack enable && corepack prepare pnpm@latest --activate
COPY package.json pnpm-lock.yaml* ./
RUN pnpm install --frozen-lockfile
COPY . .
EXPOSE 5001
CMD ["pnpm", "dev", "--host", "0.0.0.0", "--port", "5001"]
```

**Step 6: pnpm install を実行する**

```bash
cd 04_agent/mastra/frontend && pnpm install
```

**Step 7: コミット**

```bash
git add 04_agent/mastra/frontend/
git commit -m "feat(mastra): add React + Vite frontend with @mastra/client-js"
```

---

## Task 10: mastra-fastify frontend 実装

**Files:**
- Create: `04_agent/mastra-fastify/frontend/` （Task 8 の共通テンプレートを使用）

**Step 1: 各ファイルを作成する**

- `package.json` — Task 8 の共通テンプレート（name: `agent-mastra-fastify-frontend`）
- `tsconfig.json` — Task 8 と同じ
- `index.html` — Task 8 と同じ
- `src/main.tsx` — Task 8 と同じ

**Step 2: vite.config.ts を作成する（ポート 5002）**

```typescript
// 04_agent/mastra-fastify/frontend/vite.config.ts
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    host: "0.0.0.0",
    port: 5002,
  },
});
```

**Step 3: App.tsx を作成する（環境変数で設定）**

Task 8 の共通 App.tsx テンプレートをそのまま使用する。環境変数はDockerfileと docker-compose で設定する。

**Step 4: Dockerfile を作成する**

```dockerfile
# 04_agent/mastra-fastify/frontend/Dockerfile
FROM node:24-alpine
WORKDIR /app
RUN corepack enable && corepack prepare pnpm@latest --activate
COPY package.json pnpm-lock.yaml* ./
RUN pnpm install --frozen-lockfile
COPY . .
EXPOSE 5002
CMD ["pnpm", "dev", "--host", "0.0.0.0", "--port", "5002"]
```

**Step 5: pnpm install を実行する**

```bash
cd 04_agent/mastra-fastify/frontend && pnpm install
```

**Step 6: コミット**

```bash
git add 04_agent/mastra-fastify/frontend/
git commit -m "feat(mastra-fastify): add React + Vite frontend"
```

---

## Task 11: strands-python frontend 実装

**Files:**
- Create: `04_agent/strands-python/frontend/`

**Step 1〜6:** Task 10 と同じ手順。ポートは **5003**、name は `agent-strands-python-frontend`。

**Step 7: コミット**

```bash
git add 04_agent/strands-python/frontend/
git commit -m "feat(strands-python): add React + Vite frontend"
```

---

## Task 12: strands-typescript frontend 実装

**Files:**
- Create: `04_agent/strands-typescript/frontend/`

Task 10 と同じ手順。ポートは **5004**、name は `agent-strands-typescript-frontend`。

```bash
git add 04_agent/strands-typescript/frontend/
git commit -m "feat(strands-typescript): add React + Vite frontend"
```

---

## Task 13: claude-agent-sdk frontend 実装

**Files:**
- Create: `04_agent/claude-agent-sdk/frontend/`

Task 10 と同じ手順。ポートは **5005**、name は `agent-claude-agent-sdk-frontend`。

```bash
git add 04_agent/claude-agent-sdk/frontend/
git commit -m "feat(claude-agent-sdk): add React + Vite frontend"
```

---

## Task 14: docker-compose.yml に frontend サービスを追加

**Files:**
- Modify: `04_agent/docker-compose.yml`

Task 6 で更新した docker-compose.yml に、各フレームワークの frontend サービスを追加する。

```yaml
  mastra-frontend:
    build: ./mastra/frontend
    ports:
      - "5001:5001"
    environment:
      - VITE_BACKEND_URL=http://localhost:4001
    depends_on:
      - mastra
    profiles: [mastra]

  mastra-fastify-frontend:
    build: ./mastra-fastify/frontend
    ports:
      - "5002:5002"
    environment:
      - VITE_BACKEND_URL=http://localhost:4002
      - VITE_THEME_COLOR=#8b5cf6
      - VITE_FRAMEWORK_NAME=mastra-fastify
    depends_on:
      - mastra-fastify
    profiles: [mastra-fastify]

  strands-python-frontend:
    build: ./strands-python/frontend
    ports:
      - "5003:5003"
    environment:
      - VITE_BACKEND_URL=http://localhost:4003
      - VITE_THEME_COLOR=#3b82f6
      - VITE_FRAMEWORK_NAME=strands-python
    depends_on:
      - strands-python
    profiles: [strands-python]

  strands-typescript-frontend:
    build: ./strands-typescript/frontend
    ports:
      - "5004:5004"
    environment:
      - VITE_BACKEND_URL=http://localhost:4004
      - VITE_THEME_COLOR=#06b6d4
      - VITE_FRAMEWORK_NAME=strands-typescript
    depends_on:
      - strands-typescript
    profiles: [strands-typescript]

  claude-agent-sdk-frontend:
    build: ./claude-agent-sdk/frontend
    ports:
      - "5005:5005"
    environment:
      - VITE_BACKEND_URL=http://localhost:4005
      - VITE_THEME_COLOR=#f97316
      - VITE_FRAMEWORK_NAME=claude-agent-sdk
    depends_on:
      - claude-agent-sdk
    profiles: [claude-agent-sdk]
```

**Step 2: コミット**

```bash
git add 04_agent/docker-compose.yml
git commit -m "feat(docker-compose): add frontend services for all frameworks"
```

---

## 動作確認

```bash
cd 04_agent

# mastra を起動（backend + frontend）
make mastra

# ブラウザで確認
open http://localhost:5001   # mastra frontend
open http://localhost:4001   # mastra backend API
```

**確認項目:**
1. フロントエンドにアクセスできる
2. メッセージを送信するとレスポンスが返ってくる
3. 会話履歴が表示される（前のメッセージが残っている）
4. セッションIDが表示されている
