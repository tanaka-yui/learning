# 10-3 ECS vs k8s

5 軸対応で AWS ECS と Kubernetes の差分を網羅。同一 `demo-api` image を LocalStack 上 ECS と kind 上 k8s の双方で動かして比較する。

| 軸 | docs |
|---|---|
| workload | docs/01-workload-mapping.md |
| network  | docs/02-network.md |
| storage  | docs/03-storage.md |
| auth+secret | docs/04-auth-secrets.md |
| autoscale | docs/05-autoscale.md |
| Fargate vs node | docs/06-fargate-vs-node.md |
| 意思決定 | docs/decision-matrix.md |

検証: `make verify`。
