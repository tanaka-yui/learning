output "state_machine_arn" {
  value = try(aws_sfn_state_machine.demo.arn, "n/a")
}

output "eventbridge_rule" {
  value = try(aws_cloudwatch_event_rule.demo.name, "n/a")
}
