#!/bin/sh
set -e
case "$1" in
  migrate|sqlc|golangci-lint|buf|mockgen|hadolint) exec "$@";;
  help|"") cat <<'EOF'
tool-runner: migrate, sqlc, golangci-lint, buf, mockgen, hadolint
Usage: docker run --rm -v "$PWD":/work tool-runner <tool> <args...>
EOF
  ;;
  *) exec "$@";;
esac
