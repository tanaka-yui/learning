from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel
from agent import create_agent

app = FastAPI()

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_methods=["*"],
    allow_headers=["*"],
)


class ChatRequest(BaseModel):
    message: str
    sessionId: str


@app.post("/chat")
async def chat(req: ChatRequest) -> dict:
    # strands-agentsはメモリ機能を持たない
    # 会話履歴の永続化にはAmazon Bedrock AgentCore Memoryが必要
    agent = create_agent()
    result = agent(req.message)
    response_text = str(result).strip()

    return {"response": response_text}
