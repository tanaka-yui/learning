import os
import time
from datetime import datetime, timezone

from fastapi import FastAPI
from fastapi.responses import JSONResponse

app = FastAPI()

HEAVY_CALC_N = int(os.environ.get("HEAVY_CALC_N", "38"))


def fibonacci(n: int) -> int:
    if n <= 1:
        return n
    return fibonacci(n - 1) + fibonacci(n - 2)


def utc_now_iso() -> str:
    now = datetime.now(timezone.utc)
    return now.strftime("%Y-%m-%dT%H:%M:%S.") + f"{now.microsecond // 1000:03d}Z"


@app.get("/health")
async def health() -> JSONResponse:
    return JSONResponse({"status": "ok", "language": "python"})


@app.get("/heavy")
async def heavy() -> JSONResponse:
    started_at = utc_now_iso()
    start = time.monotonic()

    fibonacci(HEAVY_CALC_N)

    duration_ms = round((time.monotonic() - start) * 1000)
    finished_at = utc_now_iso()

    return JSONResponse({
        "language": "python",
        "threadId": f"uvicorn-worker-{os.getpid()}",
        "startedAt": started_at,
        "finishedAt": finished_at,
        "durationMs": duration_ms,
    })
