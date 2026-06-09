output "api_endpoint" {
  description = "HTTP API endpoint for POST /orders."
  value       = "${aws_apigatewayv2_stage.local.invoke_url}/orders"
}

output "source_queue_url" {
  description = "FIFO source queue URL consumed by order-worker pods."
  value       = aws_sqs_queue.source.url
}

output "dlq_queue_url" {
  description = "Dead-letter queue URL read by dlq-triage pods."
  value       = aws_sqs_queue.dlq.url
}

output "replay_queue_url" {
  description = "Rate-limited replay queue URL."
  value       = aws_sqs_queue.replay.url
}

output "event_bus_name" {
  description = "EventBridge bus that normalizes API and replay work events."
  value       = aws_cloudwatch_event_bus.orders.name
}

output "idempotency_table_name" {
  description = "DynamoDB table used for message/business-key idempotency."
  value       = aws_dynamodb_table.idempotency.name
}

output "orders_table_name" {
  description = "DynamoDB table for committed order side effects."
  value       = aws_dynamodb_table.orders.name
}

output "quarantine_bucket" {
  description = "S3 bucket for poison message quarantine payloads."
  value       = aws_s3_bucket.quarantine.bucket
}

output "worker_role_arn" {
  description = "IRSA role ARN for the order-worker service account."
  value       = aws_iam_role.worker.arn
}

output "triage_role_arn" {
  description = "IRSA role ARN for the dlq-triage service account."
  value       = aws_iam_role.triage.arn
}

output "eks_cluster_name" {
  description = "Local Floci EKS cluster name."
  value       = aws_eks_cluster.orders.name
}

output "remediation_state_machine_arn" {
  description = "Step Functions remediation workflow ARN."
  value       = local.remediation_machine_arn
}

