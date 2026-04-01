#!/bin/bash
set -e

# ----------------------------------------------------------------
# chapter01: 01章（正規化）
# ----------------------------------------------------------------
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
    -c "CREATE DATABASE chapter01;"

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "chapter01" \
    -f /sql/01/normalization.sql

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "chapter01" \
    -f /sql/01/exercise.sql

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "chapter01" \
    -f /sql/01/answer.sql

# ----------------------------------------------------------------
# chapter02: 02章（クエリ最適化）
# ※ SQL ファイルが揃ったためコメントを解除
# ----------------------------------------------------------------
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
    -c "CREATE DATABASE chapter02;"

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "chapter02" \
    -f /sql/02/00_schema.sql

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "chapter02" \
    -f /sql/02/01_explain.sql

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "chapter02" \
    -f /sql/02/02_indexing.sql

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "chapter02" \
    -f /sql/02/03_query_tuning.sql

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "chapter02" \
    -f /sql/02/04_denormalization.sql

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "chapter02" \
    -f /sql/02/exercise.sql

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "chapter02" \
    -f /sql/02/answer.sql
