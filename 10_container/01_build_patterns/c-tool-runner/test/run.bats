#!/usr/bin/env bats
# NOTE: bats-core is not installed in this environment.
# These tests are informational / CI-intended.
# Primary verification: `make test` (runs all 6 --version checks directly).

@test "migrate present" { run docker run --rm tool-runner:latest migrate -version; [ "$status" -eq 0 ]; }
@test "sqlc present"    { run docker run --rm tool-runner:latest sqlc    version; [ "$status" -eq 0 ]; }
@test "golangci-lint present" { run docker run --rm tool-runner:latest golangci-lint --version; [ "$status" -eq 0 ]; }
@test "buf present"     { run docker run --rm tool-runner:latest buf --version; [ "$status" -eq 0 ]; }
@test "mockgen present" { run docker run --rm tool-runner:latest mockgen -version; [ "$status" -eq 0 ]; }
@test "hadolint present"{ run docker run --rm tool-runner:latest hadolint --version; [ "$status" -eq 0 ]; }
