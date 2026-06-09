resource "aws_cloudwatch_log_group" "worker" {
  name              = "/aws/eks/${var.name_prefix}/order-worker"
  retention_in_days = 14

  tags = local.tags
}

resource "aws_cloudwatch_log_group" "triage" {
  name              = "/aws/eks/${var.name_prefix}/dlq-triage"
  retention_in_days = 14

  tags = local.tags
}

resource "aws_sns_topic" "operator_alerts" {
  name              = "${var.name_prefix}-operator-alerts"
  kms_master_key_id = aws_kms_key.work.key_id

  tags = local.tags
}

resource "aws_cloudwatch_metric_alarm" "dlq_depth" {
  alarm_name          = "${var.name_prefix}-dlq-depth"
  alarm_description   = "Page when any message remains in the DLQ for 5 minutes."
  namespace           = "AWS/SQS"
  metric_name         = "ApproximateNumberOfMessagesVisible"
  statistic           = "Sum"
  period              = 60
  evaluation_periods  = 5
  threshold           = 0
  comparison_operator = "GreaterThanThreshold"
  treat_missing_data  = "notBreaching"
  alarm_actions       = [aws_sns_topic.operator_alerts.arn]

  dimensions = {
    QueueName = aws_sqs_queue.dlq.name
  }

  tags = local.tags

  lifecycle {
    ignore_changes = [
      alarm_actions,
      dimensions,
      treat_missing_data
    ]
  }
}

resource "aws_cloudwatch_metric_alarm" "dlq_oldest_age" {
  alarm_name          = "${var.name_prefix}-dlq-oldest-age"
  alarm_description   = "Page when the oldest DLQ message is older than 30 minutes."
  namespace           = "AWS/SQS"
  metric_name         = "ApproximateAgeOfOldestMessage"
  statistic           = "Maximum"
  period              = 60
  evaluation_periods  = 1
  threshold           = 1800
  comparison_operator = "GreaterThanThreshold"
  treat_missing_data  = "notBreaching"
  alarm_actions       = [aws_sns_topic.operator_alerts.arn]

  dimensions = {
    QueueName = aws_sqs_queue.dlq.name
  }

  tags = local.tags

  lifecycle {
    ignore_changes = [
      alarm_actions,
      dimensions,
      treat_missing_data
    ]
  }
}

