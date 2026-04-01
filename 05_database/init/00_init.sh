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
# chapter02: 02章（追加時にコメントアウトを外す）
# ----------------------------------------------------------------
# psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
#     -c "CREATE DATABASE chapter02;"
# psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "chapter02" \
#     -f /sql/02/xxx.sql
