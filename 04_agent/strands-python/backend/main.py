import os
import json
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel
import redis.asyncio as aioredis
from agent import create_agent
from skills.prioritize import prioritize
from skills.summarize import summarize

app = FastAPI()

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_methods=["*"],
    allow_headers=["*"],
)
redis_client = aioredis.from_url(os.getenv("REDIS_URL", "redis://localhost:6379"))


class ChatRequest(BaseModel):
    message: str
    sessionId: str


@app.post("/chat")
async def chat(req: ChatRequest) -> dict:
    history_raw = await redis_client.get(f"session:{req.sessionId}:history")
    history: list = json.loads(history_raw) if history_raw else []

    if "優先" in req.message or "prioritize" in req.message:
        result = prioritize()
        history.extend([
            {"role": "user", "content": [{"text": req.message}]},
            {"role": "assistant", "content": [{"text": result}]},
        ])
        await redis_client.set(f"session:{req.sessionId}:history", json.dumps(history))
        return {"response": result}

    if "サマリ" in req.message or "summarize" in req.message:
        result = summarize()
        history.extend([
            {"role": "user", "content": [{"text": req.message}]},
            {"role": "assistant", "content": [{"text": result}]},
        ])
        await redis_client.set(f"session:{req.sessionId}:history", json.dumps(history))
        return {"response": result}

    agent = create_agent(messages=history if history else None)
    result = agent(req.message)
    response_text = str(result).strip()

    history.extend([
        {"role": "user", "content": [{"text": req.message}]},
        {"role": "assistant", "content": [{"text": response_text}]},
    ])
    await redis_client.set(f"session:{req.sessionId}:history", json.dumps(history))

    return {"response": response_text}
