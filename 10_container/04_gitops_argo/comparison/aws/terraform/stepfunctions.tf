resource "aws_iam_role" "sfn" {
  name = "learning-sfn-role"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{ Effect = "Allow", Principal = { Service = "states.amazonaws.com" }, Action = "sts:AssumeRole" }]
  })
}

resource "aws_sfn_state_machine" "demo" {
  name     = "learning-demo"
  role_arn = aws_iam_role.sfn.arn
  definition = jsonencode({
    Comment = "10-4 comparison: Step Functions 相当を Argo Workflows DAG と対比"
    StartAt = "Choice"
    States = {
      Choice = {
        Type = "Choice"
        Choices = [{ Variable = "$.trigger", StringEquals = "go", Next = "Parallel" }]
        Default = "Fail"
      }
      Parallel = {
        Type = "Parallel"
        End  = true
        Branches = [
          { StartAt = "Build", States = { Build = { Type = "Pass", End = true } } },
          { StartAt = "Scan", States = { Scan = { Type = "Pass", End = true } } }
        ]
      }
      Fail = { Type = "Fail", Cause = "trigger != go" }
    }
  })
}
