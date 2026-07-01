resource "aws_cloudwatch_event_rule" "demo" {
  name                = "learning-eventbridge-demo"
  description         = "trigger every 5 min"
  schedule_expression = "rate(5 minutes)"
}

resource "aws_cloudwatch_event_target" "demo" {
  rule      = aws_cloudwatch_event_rule.demo.name
  target_id = "sfn-target"
  arn       = aws_sfn_state_machine.demo.arn
  role_arn  = aws_iam_role.eventbridge_to_sfn.arn
}

resource "aws_iam_role" "eventbridge_to_sfn" {
  name = "learning-eventbridge-role"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{ Effect = "Allow", Principal = { Service = "events.amazonaws.com" }, Action = "sts:AssumeRole" }]
  })
}
