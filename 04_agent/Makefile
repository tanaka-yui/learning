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
	docker compose --profile mastra --profile mastra-fastify --profile strands-python --profile strands-typescript --profile claude-agent-sdk down
