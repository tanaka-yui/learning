# AI Agent フレームワーク比較 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 5つのAI Agentフレームワークで同一のタスク管理Agentを実装し、各フレームワークの特性を実践的に比較できる環境を構築する。

**Architecture:** 全フレームワーク共通のタスク管理Agent（ツール4種・スキル2種・Redisメモリ）を実装し、POST /chat エンドポイントで統一的に操作できる。タスクデータはインメモリ管理、会話履歴はRedisに保存する。claude-agent-sdkのみメモリ管理なし。

**Tech Stack:** TypeScript（mastra, mastra-fastify, strands-typescript, claude-agent-sdk） / Python（strands-python） / Fastify / FastAPI / Redis / Docker Compose / Vitest / pytest

---

## Task 1: ベースインフラ構築

**Files:**
- Create: `04_agent/docker-compose.yml`
- Create: `04_agent/Makefile`
- Create: `04_agent/shared/task-schema.md`

**Step 1: docker-compose.yml を作成する**

```yaml
# 04_agent/docker-compose.yml
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

  mastra:
    build: ./mastra
    ports:
      - "4001:4001"
    environment:
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
      - REDIS_URL=redis://redis:6379
    depends_on:
      redis:
        condition: service_healthy
    profiles: [mastra]

  mastra-fastify:
    build: ./mastra-fastify
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
    build: ./strands-python
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
    build: ./strands-typescript
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
    build: ./claude-agent-sdk
    ports:
      - "4005:4005"
    environment:
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
    profiles: [claude-agent-sdk]
```

**Step 2: Makefile を作成する**

```makefile
# 04_agent/Makefile
.PHONY: mastra mastra-fastify strands-python strands-typescript claude-agent-sdk all down

mastra:
	docker compose --profile mastra up --build

mastra-fastify:
	docker compose --profile mastra-fastify up --build

strands-python:
	docker compose --profile strands-python up --build

strands-typescript:
	docker compose --profile strands-typescript up --build

claude-agent-sdk:
	docker compose --profile claude-agent-sdk up --build

all:
	docker compose --profile mastra --profile mastra-fastify --profile strands-python --profile strands-typescript --profile claude-agent-sdk up --build

down:
	docker compose down
```

**Step 3: 共通API仕様ドキュメントを作成する**

`04_agent/shared/task-schema.md` に以下を記述：

```markdown
# 共通タスクAPI仕様

## タスクデータ構造

```json
{
  "id": "string (uuid)",
  "title": "string",
  "description": "string",
  "priority": "low | medium | high",
  "status": "todo | in_progress | done",
  "createdAt": "ISO8601 datetime"
}
```

## エンドポイント

| Method | Path | 説明 |
|--------|------|------|
| GET | /tasks | タスク一覧取得 |
| POST | /tasks | タスク作成 |
| PUT | /tasks/:id | タスク更新 |
| DELETE | /tasks/:id | タスク削除 |
| POST | /chat | Agent会話 |

## POST /chat

Request: `{ "message": "string", "sessionId": "string" }`
Response: `{ "response": "string" }`
```

**Step 4: コミット**

```bash
git add 04_agent/docker-compose.yml 04_agent/Makefile 04_agent/shared/
git commit -m "add 04_agent base infrastructure (docker-compose, Makefile, shared schema)"
```

---

## Task 2: mastra 実装

**Files:**
- Create: `04_agent/mastra/package.json`
- Create: `04_agent/mastra/tsconfig.json`
- Create: `04_agent/mastra/src/tools/taskTools.ts`
- Create: `04_agent/mastra/src/skills/prioritize.ts`
- Create: `04_agent/mastra/src/skills/summarize.ts`
- Create: `04_agent/mastra/src/agent.ts`
- Create: `04_agent/mastra/src/index.ts`
- Create: `04_agent/mastra/src/__tests__/tools.test.ts`
- Create: `04_agent/mastra/Dockerfile`

**Step 1: package.json を作成する**

```json
{
  "name": "agent-mastra",
  "version": "1.0.0",
  "type": "module",
  "scripts": {
    "dev": "tsx watch src/index.ts",
    "start": "tsx src/index.ts",
    "test": "vitest run"
  },
  "dependencies": {
    "@mastra/core": "latest",
    "@ai-sdk/anthropic": "latest",
    "ioredis": "^5.4.2",
    "uuid": "^11.1.0"
  },
  "devDependencies": {
    "@types/uuid": "^10.0.0",
    "tsx": "^4.19.3",
    "typescript": "^5.8.3",
    "vitest": "^3.1.1"
  }
}
```

**Step 2: ツールの失敗テストを書く**

```typescript
// 04_agent/mastra/src/__tests__/tools.test.ts
import { describe, it, expect, beforeEach } from "vitest";
import { taskStore, createTask, listTasks, updateTask, deleteTask } from "../tools/taskTools.js";

describe("taskTools", () => {
  beforeEach(() => {
    taskStore.length = 0;
  });

  it("creates a task with required fields", () => {
    const task = createTask({ title: "Test task", description: "desc", priority: "high" });
    expect(task.id).toBeDefined();
    expect(task.title).toBe("Test task");
    expect(task.status).toBe("todo");
  });

  it("lists all tasks", () => {
    createTask({ title: "Task 1", description: "", priority: "low" });
    createTask({ title: "Task 2", description: "", priority: "medium" });
    expect(listTasks()).toHaveLength(2);
  });

  it("updates a task", () => {
    const task = createTask({ title: "Old", description: "", priority: "low" });
    const updated = updateTask(task.id, { title: "New" });
    expect(updated?.title).toBe("New");
  });

  it("deletes a task", () => {
    const task = createTask({ title: "To delete", description: "", priority: "low" });
    expect(deleteTask(task.id)).toBe(true);
    expect(listTasks()).toHaveLength(0);
  });
});
```

**Step 3: テストが失敗することを確認する**

```bash
cd 04_agent/mastra && pnpm install && pnpm test
```
Expected: FAIL (taskTools モジュールが存在しない)

**Step 4: taskTools.ts を実装する**

```typescript
// 04_agent/mastra/src/tools/taskTools.ts
import { v4 as uuidv4 } from "uuid";

type Priority = "low" | "medium" | "high";
type Status = "todo" | "in_progress" | "done";

type Task = {
  id: string;
  title: string;
  description: string;
  priority: Priority;
  status: Status;
  createdAt: string;
};

export const taskStore: Task[] = [];

type CreateInput = { title: string; description: string; priority: Priority };

export const createTask = (input: CreateInput): Task => {
  const task: Task = {
    id: uuidv4(),
    ...input,
    status: "todo",
    createdAt: new Date().toISOString(),
  };
  taskStore.push(task);
  return task;
};

export const listTasks = (): Task[] => [...taskStore];

export const updateTask = (id: string, updates: Partial<Omit<Task, "id" | "createdAt">>): Task | null => {
  const idx = taskStore.findIndex((t) => t.id === id);
  if (idx === -1) return null;
  taskStore[idx] = { ...taskStore[idx], ...updates };
  return taskStore[idx];
};

export const deleteTask = (id: string): boolean => {
  const idx = taskStore.findIndex((t) => t.id === id);
  if (idx === -1) return false;
  taskStore.splice(idx, 1);
  return true;
};
```

**Step 5: テストが通ることを確認する**

```bash
pnpm test
```
Expected: PASS (4 tests)

**Step 6: Mastra ツール定義とスキルを実装する**

```typescript
// 04_agent/mastra/src/tools/index.ts
import { createTool } from "@mastra/core/tools";
import { z } from "zod";
import { createTask, listTasks, updateTask, deleteTask } from "./taskTools.js";

export const createTaskTool = createTool({
  id: "createTask",
  description: "新しいタスクを作成する",
  inputSchema: z.object({
    title: z.string(),
    description: z.string(),
    priority: z.enum(["low", "medium", "high"]),
  }),
  execute: async ({ context }) => createTask(context),
});

export const listTasksTool = createTool({
  id: "listTasks",
  description: "タスク一覧を取得する",
  inputSchema: z.object({}),
  execute: async () => listTasks(),
});

export const updateTaskTool = createTool({
  id: "updateTask",
  description: "タスクを更新する",
  inputSchema: z.object({
    id: z.string(),
    title: z.string().optional(),
    status: z.enum(["todo", "in_progress", "done"]).optional(),
    priority: z.enum(["low", "medium", "high"]).optional(),
  }),
  execute: async ({ context }) => {
    const { id, ...updates } = context;
    return updateTask(id, updates);
  },
});

export const deleteTaskTool = createTool({
  id: "deleteTask",
  description: "タスクを削除する",
  inputSchema: z.object({ id: z.string() }),
  execute: async ({ context }) => deleteTask(context.id),
});
```

```typescript
// 04_agent/mastra/src/skills/prioritize.ts
import { listTasks } from "../tools/taskTools.js";

const priorityOrder = { high: 0, medium: 1, low: 2 };

export const prioritize = (): string => {
  const tasks = listTasks().filter((t) => t.status !== "done");
  const sorted = [...tasks].sort((a, b) => priorityOrder[a.priority] - priorityOrder[b.priority]);
  if (sorted.length === 0) return "未完了のタスクはありません。";
  return sorted.map((t, i) => `${i + 1}. [${t.priority}] ${t.title} (${t.status})`).join("\n");
};
```

```typescript
// 04_agent/mastra/src/skills/summarize.ts
import { listTasks } from "../tools/taskTools.js";

export const summarize = (): string => {
  const tasks = listTasks();
  const total = tasks.length;
  const done = tasks.filter((t) => t.status === "done").length;
  const inProgress = tasks.filter((t) => t.status === "in_progress").length;
  const todo = tasks.filter((t) => t.status === "todo").length;
  return `タスク合計: ${total}件（完了: ${done}件、進行中: ${inProgress}件、未着手: ${todo}件）`;
};
```

**Step 7: Agent と HTTPサーバーを実装する**

```typescript
// 04_agent/mastra/src/agent.ts
import { Agent } from "@mastra/core/agent";
import { anthropic } from "@ai-sdk/anthropic";
import { createTaskTool, listTasksTool, updateTaskTool, deleteTaskTool } from "./tools/index.js";

export const taskAgent = new Agent({
  name: "TaskAgent",
  instructions: `あなたはタスク管理エージェントです。
ユーザーのタスク管理を支援します。
タスクの作成・一覧・更新・削除ができます。
優先度順の並び替えやサマリーも提供できます。`,
  model: anthropic("claude-sonnet-4-6"),
  tools: {
    createTask: createTaskTool,
    listTasks: listTasksTool,
    updateTask: updateTaskTool,
    deleteTask: deleteTaskTool,
  },
});
```

```typescript
// 04_agent/mastra/src/index.ts
import Fastify from "fastify";
import { Redis } from "ioredis";
import { taskAgent } from "./agent.js";
import { prioritize } from "./skills/prioritize.js";
import { summarize } from "./skills/summarize.js";

const app = Fastify({ logger: true });
const redis = new Redis(process.env.REDIS_URL ?? "redis://localhost:6379");
const PORT = 4001;

app.post<{ Body: { message: string; sessionId: string } }>("/chat", async (req, reply) => {
  const { message, sessionId } = req.body;

  const historyRaw = await redis.get(`session:${sessionId}:history`);
  const history = historyRaw ? JSON.parse(historyRaw) : [];

  // スキルキーワード検出
  if (message.includes("優先") || message.includes("prioritize")) {
    const result = prioritize();
    history.push({ role: "user", content: message }, { role: "assistant", content: result });
    await redis.set(`session:${sessionId}:history`, JSON.stringify(history));
    return reply.send({ response: result });
  }

  if (message.includes("サマリ") || message.includes("summarize")) {
    const result = summarize();
    history.push({ role: "user", content: message }, { role: "assistant", content: result });
    await redis.set(`session:${sessionId}:history`, JSON.stringify(history));
    return reply.send({ response: result });
  }

  const result = await taskAgent.generate(message, { conversationHistory: history });
  const response = result.text;

  history.push({ role: "user", content: message }, { role: "assistant", content: response });
  await redis.set(`session:${sessionId}:history`, JSON.stringify(history));

  return reply.send({ response });
});

app.listen({ port: PORT, host: "0.0.0.0" }, (err) => {
  if (err) throw err;
});
```

**Step 8: Dockerfile を作成する**

```dockerfile
# 04_agent/mastra/Dockerfile
FROM node:20-alpine
WORKDIR /app
RUN corepack enable && corepack prepare pnpm@latest --activate
COPY package.json pnpm-lock.yaml* ./
RUN pnpm install --frozen-lockfile
COPY . .
CMD ["pnpm", "start"]
```

**Step 9: コミット**

```bash
git add 04_agent/mastra/
git commit -m "add mastra agent implementation with task tools and Redis memory"
```

---

## Task 3: mastra-fastify 実装

**Files:**
- Create: `04_agent/mastra-fastify/package.json`
- Create: `04_agent/mastra-fastify/src/tools/taskTools.ts` （mastraと同じ内容）
- Create: `04_agent/mastra-fastify/src/skills/prioritize.ts` （mastraと同じ内容）
- Create: `04_agent/mastra-fastify/src/skills/summarize.ts` （mastraと同じ内容）
- Create: `04_agent/mastra-fastify/src/agent.ts`
- Create: `04_agent/mastra-fastify/src/index.ts`
- Create: `04_agent/mastra-fastify/Dockerfile`

**Step 1: package.json を作成する**

mastraと同じ依存関係に `fastify` を明示的に追加（@mastra/coreの低レベルAPIを使用）。

```json
{
  "name": "agent-mastra-fastify",
  "version": "1.0.0",
  "type": "module",
  "scripts": {
    "dev": "tsx watch src/index.ts",
    "start": "tsx src/index.ts",
    "test": "vitest run"
  },
  "dependencies": {
    "@mastra/core": "latest",
    "@ai-sdk/anthropic": "latest",
    "fastify": "^5.3.3",
    "ioredis": "^5.4.2",
    "uuid": "^11.1.0"
  },
  "devDependencies": {
    "@types/uuid": "^10.0.0",
    "tsx": "^4.19.3",
    "typescript": "^5.8.3",
    "vitest": "^3.1.1"
  }
}
```

**Step 2: taskTools.ts・スキルをコピーし、Agent構成の差異を実装する**

mastra-fastify の特徴: `@mastra/core` を直接 import してMastraインスタンスを構築し、Fastifyと手動で連携する。

```typescript
// 04_agent/mastra-fastify/src/agent.ts
import { Mastra } from "@mastra/core";
import { Agent } from "@mastra/core/agent";
import { anthropic } from "@ai-sdk/anthropic";
import { createTaskTool, listTasksTool, updateTaskTool, deleteTaskTool } from "./tools/index.js";

const agent = new Agent({
  name: "TaskAgent",
  instructions: `あなたはタスク管理エージェントです。`,
  model: anthropic("claude-sonnet-4-6"),
  tools: {
    createTask: createTaskTool,
    listTasks: listTasksTool,
    updateTask: updateTaskTool,
    deleteTask: deleteTaskTool,
  },
});

export const mastra = new Mastra({ agents: { taskAgent: agent } });
```

```typescript
// 04_agent/mastra-fastify/src/index.ts
import Fastify from "fastify";
import { Redis } from "ioredis";
import { mastra } from "./agent.js";
import { prioritize } from "./skills/prioritize.js";
import { summarize } from "./skills/summarize.js";

const app = Fastify({ logger: true });
const redis = new Redis(process.env.REDIS_URL ?? "redis://localhost:6379");
const PORT = 4002;

const agent = mastra.getAgent("taskAgent");

app.post<{ Body: { message: string; sessionId: string } }>("/chat", async (req, reply) => {
  const { message, sessionId } = req.body;

  const historyRaw = await redis.get(`session:${sessionId}:history`);
  const history = historyRaw ? JSON.parse(historyRaw) : [];

  if (message.includes("優先") || message.includes("prioritize")) {
    const result = prioritize();
    history.push({ role: "user", content: message }, { role: "assistant", content: result });
    await redis.set(`session:${sessionId}:history`, JSON.stringify(history));
    return reply.send({ response: result });
  }

  if (message.includes("サマリ") || message.includes("summarize")) {
    const result = summarize();
    history.push({ role: "user", content: message }, { role: "assistant", content: result });
    await redis.set(`session:${sessionId}:history`, JSON.stringify(history));
    return reply.send({ response: result });
  }

  const result = await agent.generate(message, { conversationHistory: history });
  const response = result.text;

  history.push({ role: "user", content: message }, { role: "assistant", content: response });
  await redis.set(`session:${sessionId}:history`, JSON.stringify(history));

  return reply.send({ response });
});

app.listen({ port: PORT, host: "0.0.0.0" });
```

**Step 3: Dockerfile を作成する（mastraと同一内容、ポートのみ異なる）**

**Step 4: コミット**

```bash
git add 04_agent/mastra-fastify/
git commit -m "add mastra-fastify agent implementation"
```

---

## Task 4: strands-python 実装

**Files:**
- Create: `04_agent/strands-python/pyproject.toml`
- Create: `04_agent/strands-python/tools/task_tools.py`
- Create: `04_agent/strands-python/skills/prioritize.py`
- Create: `04_agent/strands-python/skills/summarize.py`
- Create: `04_agent/strands-python/agent.py`
- Create: `04_agent/strands-python/main.py`
- Create: `04_agent/strands-python/tests/test_task_tools.py`
- Create: `04_agent/strands-python/Dockerfile`

**Step 1: pyproject.toml を作成する**

```toml
[project]
name = "agent-strands-python"
version = "1.0.0"
requires-python = ">=3.12"
dependencies = [
  "strands-agents>=0.1.0",
  "strands-agents-tools>=0.1.0",
  "fastapi>=0.115.0",
  "uvicorn>=0.34.0",
  "redis>=5.2.1",
  "pydantic>=2.11.1",
]

[project.optional-dependencies]
dev = ["pytest>=8.0", "httpx>=0.28.0", "pytest-asyncio>=0.25.0"]

[tool.pytest.ini_options]
asyncio_mode = "auto"
```

**Step 2: ツールの失敗テストを書く**

```python
# 04_agent/strands-python/tests/test_task_tools.py
import pytest
from tools.task_tools import create_task, list_tasks, update_task, delete_task, task_store

@pytest.fixture(autouse=True)
def clear_store():
    task_store.clear()
    yield
    task_store.clear()

def test_create_task():
    task = create_task(title="Test", description="desc", priority="high")
    assert task["status"] == "todo"
    assert task["title"] == "Test"

def test_list_tasks():
    create_task(title="T1", description="", priority="low")
    create_task(title="T2", description="", priority="medium")
    assert len(list_tasks()) == 2

def test_update_task():
    task = create_task(title="Old", description="", priority="low")
    updated = update_task(task["id"], title="New")
    assert updated["title"] == "New"

def test_delete_task():
    task = create_task(title="Del", description="", priority="low")
    assert delete_task(task["id"]) is True
    assert len(list_tasks()) == 0
```

**Step 3: テストが失敗することを確認する**

```bash
cd 04_agent/strands-python && uv sync --extra dev && uv run pytest tests/ -v
```
Expected: FAIL

**Step 4: task_tools.py を実装する**

```python
# 04_agent/strands-python/tools/task_tools.py
from uuid import uuid4
from datetime import datetime, timezone
from strands import tool

task_store: list[dict] = []

@tool
def create_task(title: str, description: str, priority: str) -> dict:
    """新しいタスクを作成する。priorityはlow/medium/highのいずれか。"""
    task = {
        "id": str(uuid4()),
        "title": title,
        "description": description,
        "priority": priority,
        "status": "todo",
        "createdAt": datetime.now(timezone.utc).isoformat(),
    }
    task_store.append(task)
    return task

@tool
def list_tasks() -> list[dict]:
    """タスク一覧を取得する。"""
    return list(task_store)

@tool
def update_task(id: str, title: str | None = None, status: str | None = None, priority: str | None = None) -> dict | None:
    """タスクを更新する。"""
    for task in task_store:
        if task["id"] == id:
            if title is not None:
                task["title"] = title
            if status is not None:
                task["status"] = status
            if priority is not None:
                task["priority"] = priority
            return task
    return None

@tool
def delete_task(id: str) -> bool:
    """タスクを削除する。"""
    for i, task in enumerate(task_store):
        if task["id"] == id:
            task_store.pop(i)
            return True
    return False
```

**Step 5: テストが通ることを確認する**

```bash
uv run pytest tests/ -v
```
Expected: PASS (4 tests)

**Step 6: スキル・Agent・FastAPIサーバーを実装する**

```python
# 04_agent/strands-python/skills/prioritize.py
from tools.task_tools import task_store

PRIORITY_ORDER = {"high": 0, "medium": 1, "low": 2}

def prioritize() -> str:
    incomplete = [t for t in task_store if t["status"] != "done"]
    sorted_tasks = sorted(incomplete, key=lambda t: PRIORITY_ORDER.get(t["priority"], 99))
    if not sorted_tasks:
        return "未完了のタスクはありません。"
    return "\n".join(f"{i+1}. [{t['priority']}] {t['title']} ({t['status']})"
                     for i, t in enumerate(sorted_tasks))
```

```python
# 04_agent/strands-python/skills/summarize.py
from tools.task_tools import task_store

def summarize() -> str:
    total = len(task_store)
    done = sum(1 for t in task_store if t["status"] == "done")
    in_progress = sum(1 for t in task_store if t["status"] == "in_progress")
    todo = sum(1 for t in task_store if t["status"] == "todo")
    return f"タスク合計: {total}件（完了: {done}件、進行中: {in_progress}件、未着手: {todo}件）"
```

```python
# 04_agent/strands-python/agent.py
import os
from strands import Agent
from strands.models import BedrockModel
from strands_tools import http_request
from tools.task_tools import create_task, list_tasks, update_task, delete_task

model = BedrockModel(model_id="anthropic.claude-sonnet-4-5")

task_agent = Agent(
    model=model,
    tools=[create_task, list_tasks, update_task, delete_task],
    system_prompt="""あなたはタスク管理エージェントです。
ユーザーのタスク管理を支援します。""",
)
```

```python
# 04_agent/strands-python/main.py
import os
import json
from fastapi import FastAPI
from pydantic import BaseModel
import redis.asyncio as aioredis
from agent import task_agent
from skills.prioritize import prioritize
from skills.summarize import summarize

app = FastAPI()
redis_client = aioredis.from_url(os.getenv("REDIS_URL", "redis://localhost:6379"))

class ChatRequest(BaseModel):
    message: str
    sessionId: str

@app.post("/chat")
async def chat(req: ChatRequest):
    history_raw = await redis_client.get(f"session:{req.sessionId}:history")
    history = json.loads(history_raw) if history_raw else []

    if "優先" in req.message or "prioritize" in req.message:
        result = prioritize()
        history.extend([{"role": "user", "content": req.message},
                        {"role": "assistant", "content": result}])
        await redis_client.set(f"session:{req.sessionId}:history", json.dumps(history))
        return {"response": result}

    if "サマリ" in req.message or "summarize" in req.message:
        result = summarize()
        history.extend([{"role": "user", "content": req.message},
                        {"role": "assistant", "content": result}])
        await redis_client.set(f"session:{req.sessionId}:history", json.dumps(history))
        return {"response": result}

    response = task_agent(req.message)
    response_text = str(response)

    history.extend([{"role": "user", "content": req.message},
                    {"role": "assistant", "content": response_text}])
    await redis_client.set(f"session:{req.sessionId}:history", json.dumps(history))

    return {"response": response_text}
```

**Step 7: Dockerfile を作成する**

```dockerfile
# 04_agent/strands-python/Dockerfile
FROM python:3.12-slim
WORKDIR /app
RUN pip install uv
COPY pyproject.toml ./
RUN uv sync
COPY . .
CMD ["uv", "run", "uvicorn", "main:app", "--host", "0.0.0.0", "--port", "4003"]
```

**Step 8: コミット**

```bash
git add 04_agent/strands-python/
git commit -m "add strands-python agent implementation"
```

---

## Task 5: strands-typescript 実装

**Files:**
- Create: `04_agent/strands-typescript/package.json`
- Create: `04_agent/strands-typescript/src/tools/taskTools.ts` （mastraと同じツール関数）
- Create: `04_agent/strands-typescript/src/tools/index.ts` （Strands形式のツール定義）
- Create: `04_agent/strands-typescript/src/skills/prioritize.ts`
- Create: `04_agent/strands-typescript/src/skills/summarize.ts`
- Create: `04_agent/strands-typescript/src/agent.ts`
- Create: `04_agent/strands-typescript/src/index.ts`
- Create: `04_agent/strands-typescript/src/__tests__/tools.test.ts`
- Create: `04_agent/strands-typescript/Dockerfile`

**Step 1: package.json を作成する**

```json
{
  "name": "agent-strands-typescript",
  "version": "1.0.0",
  "type": "module",
  "scripts": {
    "dev": "tsx watch src/index.ts",
    "start": "tsx src/index.ts",
    "test": "vitest run"
  },
  "dependencies": {
    "@strands/agent-sdk": "latest",
    "fastify": "^5.3.3",
    "ioredis": "^5.4.2",
    "uuid": "^11.1.0",
    "zod": "^3.24.2"
  },
  "devDependencies": {
    "@types/uuid": "^10.0.0",
    "tsx": "^4.19.3",
    "typescript": "^5.8.3",
    "vitest": "^3.1.1"
  }
}
```

> **注意:** `@strands/agent-sdk` の正確なパッケージ名は公式ドキュメントを確認してインストール時に調整する。

**Step 2: ツールのテストを書く（mastraと同じ内容）**

`src/__tests__/tools.test.ts` は Task 2 の mastra と同一内容を使用する。

**Step 3: taskTools.ts をコピーし、Strands形式のツール定義を実装する**

```typescript
// 04_agent/strands-typescript/src/tools/index.ts
import { z } from "zod";
import { createTask, listTasks, updateTask, deleteTask } from "./taskTools.js";

export const tools = {
  createTask: {
    description: "新しいタスクを作成する",
    parameters: z.object({
      title: z.string(),
      description: z.string(),
      priority: z.enum(["low", "medium", "high"]),
    }),
    execute: (params: { title: string; description: string; priority: "low" | "medium" | "high" }) =>
      createTask(params),
  },
  listTasks: {
    description: "タスク一覧を取得する",
    parameters: z.object({}),
    execute: () => listTasks(),
  },
  updateTask: {
    description: "タスクを更新する",
    parameters: z.object({
      id: z.string(),
      title: z.string().optional(),
      status: z.enum(["todo", "in_progress", "done"]).optional(),
      priority: z.enum(["low", "medium", "high"]).optional(),
    }),
    execute: ({ id, ...updates }: { id: string; title?: string; status?: "todo" | "in_progress" | "done"; priority?: "low" | "medium" | "high" }) =>
      updateTask(id, updates),
  },
  deleteTask: {
    description: "タスクを削除する",
    parameters: z.object({ id: z.string() }),
    execute: ({ id }: { id: string }) => deleteTask(id),
  },
};
```

**Step 4: Agent・HTTPサーバーを実装する（ポート 4004）**

agent.ts・index.ts は mastra の実装を参考にしつつ、Strands SDKのAPIに合わせて実装する。公式ドキュメントを確認してAPIを調整すること。

**Step 5: テストを確認してコミット**

```bash
cd 04_agent/strands-typescript && pnpm install && pnpm test
git add 04_agent/strands-typescript/
git commit -m "add strands-typescript agent implementation"
```

---

## Task 6: claude-agent-sdk 実装

**Files:**
- Create: `04_agent/claude-agent-sdk/package.json`
- Create: `04_agent/claude-agent-sdk/src/tools/taskTools.ts`
- Create: `04_agent/claude-agent-sdk/src/skills/prioritize.ts`
- Create: `04_agent/claude-agent-sdk/src/skills/summarize.ts`
- Create: `04_agent/claude-agent-sdk/src/agent.ts`
- Create: `04_agent/claude-agent-sdk/src/index.ts`
- Create: `04_agent/claude-agent-sdk/src/__tests__/tools.test.ts`
- Create: `04_agent/claude-agent-sdk/Dockerfile`

**Step 1: package.json を作成する**

```json
{
  "name": "agent-claude-agent-sdk",
  "version": "1.0.0",
  "type": "module",
  "scripts": {
    "dev": "tsx watch src/index.ts",
    "start": "tsx src/index.ts",
    "test": "vitest run"
  },
  "dependencies": {
    "@anthropic-ai/sdk": "latest",
    "fastify": "^5.3.3",
    "uuid": "^11.1.0",
    "zod": "^3.24.2"
  },
  "devDependencies": {
    "@types/uuid": "^10.0.0",
    "tsx": "^4.19.3",
    "typescript": "^5.8.3",
    "vitest": "^3.1.1"
  }
}
```

> **注意:** Claude Code Agent SDKの正確なパッケージ名は公式ドキュメントを確認する。`@anthropic-ai/sdk` を使ったtool_use機能でAgentを実装する。

**Step 2: ツールテストを書く（他と同一内容）**

**Step 3: taskTools.ts・スキルをコピーする**

**Step 4: Anthropic SDK を使ったAgent実装（メモリなし）**

```typescript
// 04_agent/claude-agent-sdk/src/agent.ts
import Anthropic from "@anthropic-ai/sdk";
import { createTask, listTasks, updateTask, deleteTask } from "./tools/taskTools.js";

const client = new Anthropic();

const toolDefinitions: Anthropic.Tool[] = [
  {
    name: "createTask",
    description: "新しいタスクを作成する",
    input_schema: {
      type: "object" as const,
      properties: {
        title: { type: "string" },
        description: { type: "string" },
        priority: { type: "string", enum: ["low", "medium", "high"] },
      },
      required: ["title", "description", "priority"],
    },
  },
  {
    name: "listTasks",
    description: "タスク一覧を取得する",
    input_schema: { type: "object" as const, properties: {} },
  },
  {
    name: "updateTask",
    description: "タスクを更新する",
    input_schema: {
      type: "object" as const,
      properties: {
        id: { type: "string" },
        title: { type: "string" },
        status: { type: "string", enum: ["todo", "in_progress", "done"] },
        priority: { type: "string", enum: ["low", "medium", "high"] },
      },
      required: ["id"],
    },
  },
  {
    name: "deleteTask",
    description: "タスクを削除する",
    input_schema: {
      type: "object" as const,
      properties: { id: { type: "string" } },
      required: ["id"],
    },
  },
];

const toolHandlers: Record<string, (input: Record<string, unknown>) => unknown> = {
  createTask: (input) => createTask(input as Parameters<typeof createTask>[0]),
  listTasks: () => listTasks(),
  updateTask: ({ id, ...updates }) => updateTask(id as string, updates as Parameters<typeof updateTask>[1]),
  deleteTask: ({ id }) => deleteTask(id as string),
};

export const runAgent = async (message: string): Promise<string> => {
  const messages: Anthropic.MessageParam[] = [{ role: "user", content: message }];

  while (true) {
    const response = await client.messages.create({
      model: "claude-sonnet-4-6",
      max_tokens: 4096,
      system: "あなたはタスク管理エージェントです。",
      tools: toolDefinitions,
      messages,
    });

    if (response.stop_reason === "end_turn") {
      const textBlock = response.content.find((b) => b.type === "text");
      return textBlock ? textBlock.text : "";
    }

    if (response.stop_reason === "tool_use") {
      messages.push({ role: "assistant", content: response.content });
      const toolResults: Anthropic.ToolResultBlockParam[] = response.content
        .filter((b): b is Anthropic.ToolUseBlock => b.type === "tool_use")
        .map((toolUse) => ({
          type: "tool_result" as const,
          tool_use_id: toolUse.id,
          content: JSON.stringify(toolHandlers[toolUse.name]?.(toolUse.input as Record<string, unknown>) ?? null),
        }));
      messages.push({ role: "user", content: toolResults });
    } else {
      break;
    }
  }

  return "";
};
```

```typescript
// 04_agent/claude-agent-sdk/src/index.ts
import Fastify from "fastify";
import { runAgent } from "./agent.js";
import { prioritize } from "./skills/prioritize.js";
import { summarize } from "./skills/summarize.js";

const app = Fastify({ logger: true });
const PORT = 4005;

app.post<{ Body: { message: string; sessionId: string } }>("/chat", async (req, reply) => {
  const { message } = req.body;
  // メモリなし: sessionIdは受け取るが会話履歴は保存しない

  if (message.includes("優先") || message.includes("prioritize")) {
    return reply.send({ response: prioritize() });
  }

  if (message.includes("サマリ") || message.includes("summarize")) {
    return reply.send({ response: summarize() });
  }

  const response = await runAgent(message);
  return reply.send({ response });
});

app.listen({ port: PORT, host: "0.0.0.0" });
```

**Step 5: テストを確認してコミット**

```bash
cd 04_agent/claude-agent-sdk && pnpm install && pnpm test
git add 04_agent/claude-agent-sdk/
git commit -m "add claude-agent-sdk implementation (no memory)"
```

---

## Task 7: 比較ドキュメント作成

**Files:**
- Create: `04_agent/docs/comparison.md`

**Step 1: 各実装のコード量・特徴を確認した上で比較表を作成する**

全実装が完了した後、以下の観点で `docs/comparison.md` を作成する：

- 各フレームワークの設定ファイル・ツール定義・Agent構築のコードスニペットを引用
- 比較表に実測値（行数・依存関係数）を記載
- 各フレームワークの推奨ユースケースを記述

**Step 2: コミット**

```bash
git add 04_agent/docs/comparison.md
git commit -m "add agent framework comparison documentation"
```

---

## 動作確認手順

全実装完了後、以下で動作確認する：

```bash
cd 04_agent

# 例: mastraを起動
make mastra

# 別ターミナルで動作確認
curl -X POST http://localhost:4001/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "「レポート作成」というタスクを高優先度で作って", "sessionId": "test-session-1"}'

curl -X POST http://localhost:4001/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "今のタスクを優先度順に教えて", "sessionId": "test-session-1"}'
```
