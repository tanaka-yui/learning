FROM python:3.13-slim-bookworm

WORKDIR /app

COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt

COPY main.py ./

EXPOSE 8080

CMD exec uvicorn main:app --host 0.0.0.0 --port 8080 --workers ${UVICORN_WORKERS:-4}
