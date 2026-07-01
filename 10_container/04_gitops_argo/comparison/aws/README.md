# 10-4 AWS 比較 (LocalStack + Terraform)

前提: LocalStack Community が動作中 (10-3 の compose を使う)

```sh
docker compose -f ../../03_ecs_vs_k8s/localstack/docker-compose.yml up -d
cd terraform && terraform init && terraform apply -auto-approve
```

Community でサポート: IAM, CloudWatch Logs/Events, StepFunctions, EventBridge Rule/Target
Community 非サポート (501): CodePipeline, CodeBuild, CodeDeploy → 本物 AWS 想定の宣言のみ

比較対応:
| Argo | AWS |
|---|---|
| Argo CD Application | CodePipeline (source→build→deploy) |
| Argo Workflows DAG  | Step Functions Parallel/Choice |
| Argo Events Sensor  | EventBridge Rule + Target |
| Rollout AnalysisTemplate | CodeDeploy Blue/Green hook |
