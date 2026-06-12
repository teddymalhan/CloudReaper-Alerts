# LocalStack REST API invoke path (the "_user_request_" form is the reliable one on LocalStack).
output "send_message_endpoint" {
  description = "POST orphan alerts here (consumed by cmd/sender)."
  value       = "${var.aws_endpoint}/restapis/${aws_api_gateway_rest_api.orphan.id}/${aws_api_gateway_stage.prod.stage_name}/_user_request_/send-message"
}

output "main_queue_url" {
  description = "Main SQS queue URL (as the emulator reports it, internal host)."
  value       = aws_sqs_queue.main.url
}

output "main_queue_send_url" {
  description = "Host-reachable main queue URL for cmd/sender -queue-url (direct-SQS mode)."
  value       = "${var.aws_endpoint}/${local.account_id}/${aws_sqs_queue.main.name}"
}

output "dlq_url" {
  description = "Dead-letter queue URL."
  value       = aws_sqs_queue.dlq.url
}

output "notifier_function_name" {
  description = "Notifier Lambda function name."
  value       = aws_lambda_function.notifier.function_name
}

output "slack_secret_name" {
  description = "Secrets Manager secret holding Slack credentials."
  value       = aws_secretsmanager_secret.slack.name
}

output "orphan_volume_id" {
  description = "The deliberately unattached EBS volume (should be flagged)."
  value       = aws_ebs_volume.orphan.id
}

output "orphan_eip_id" {
  description = "The deliberately unassociated Elastic IP (should be flagged)."
  value       = aws_eip.orphan.allocation_id
}

output "reactor_function_name" {
  description = "Event-driven reactor Lambda name (null unless enable_reactor=true)."
  value       = one(aws_lambda_function.reactor[*].function_name)
}

output "reactor_event_rule" {
  description = "EventBridge rule matching orphan-creating EC2 actions (null unless enable_reactor=true)."
  value       = one(aws_cloudwatch_event_rule.ec2_orphan_actions[*].name)
}
