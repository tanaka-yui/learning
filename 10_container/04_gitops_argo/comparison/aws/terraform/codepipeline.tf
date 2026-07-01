# CodePipeline は LocalStack Community で未対応 (Pro)。宣言のみ、apply は 501。
# 本物 AWS を想定した学習用サンプル。
resource "aws_iam_role" "codepipeline" {
  count = 0 # デフォルト無効化、count=1 で有効化
  name  = "learning-codepipeline"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{ Effect = "Allow", Principal = { Service = "codepipeline.amazonaws.com" }, Action = "sts:AssumeRole" }]
  })
}
