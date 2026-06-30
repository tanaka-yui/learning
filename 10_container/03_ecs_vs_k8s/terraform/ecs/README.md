# Terraform: ECS (LocalStack)

## Apply

```sh
cd terraform/ecs
terraform init
terraform apply -auto-approve -var="ecr_repo=localhost:5000/demo-api"
```

ECR が実際に使える環境では:

```sh
ECR_REPO=$(aws --endpoint-url=http://localhost:4566 ecr describe-repositories \
  --repository-names demo-api \
  --query 'repositories[0].repositoryUri' --output text)
terraform apply -auto-approve -var="ecr_repo=$ECR_REPO"
```

## 注意

- LocalStack Community では ALB / network resources は限定。本 module は ECS Cluster / TaskDef / Service / IAM / CloudWatch Logs に絞っている
- `subnets` / `security_groups` は LocalStack 用プレースホルダー (`subnet-localstack-1` / `sg-localstack-1`)
- 実 AWS へ流すときは `endpoint` を空に、`subnets` / `security_groups` を本物に置換
- image は `registry:2` ローカルレジストリ (`localhost:5000/demo-api:v1`) を使用
