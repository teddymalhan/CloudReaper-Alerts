resource "aws_sqs_queue" "dlq" {
  name                              = "${var.name_prefix}-dlq.fifo"
  fifo_queue                        = true
  content_based_deduplication       = true
  message_retention_seconds         = 1209600
  kms_master_key_id                 = aws_kms_key.work.key_id
  kms_data_key_reuse_period_seconds = 300

  tags = merge(local.tags, {
    Name = "${var.name_prefix}-dlq.fifo"
  })
}

resource "aws_sqs_queue" "source" {
  name                              = "${var.name_prefix}.fifo"
  fifo_queue                        = true
  content_based_deduplication       = true
  visibility_timeout_seconds        = 180
  message_retention_seconds         = 345600
  receive_wait_time_seconds         = 20
  kms_master_key_id                 = aws_kms_key.work.key_id
  kms_data_key_reuse_period_seconds = 300

  tags = merge(local.tags, {
    Name = "${var.name_prefix}.fifo"
  })
}

resource "aws_sqs_queue" "replay" {
  name                              = "${var.name_prefix}-replay.fifo"
  fifo_queue                        = true
  content_based_deduplication       = true
  delay_seconds                     = 5
  message_retention_seconds         = 345600
  receive_wait_time_seconds         = 20
  kms_master_key_id                 = aws_kms_key.work.key_id
  kms_data_key_reuse_period_seconds = 300

  tags = merge(local.tags, {
    Name = "${var.name_prefix}-replay.fifo"
  })
}

resource "aws_sqs_queue_redrive_policy" "source" {
  queue_url = aws_sqs_queue.source.id

  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.dlq.arn
    maxReceiveCount     = 5
  })
}

resource "aws_sqs_queue_redrive_allow_policy" "dlq" {
  queue_url = aws_sqs_queue.dlq.id

  redrive_allow_policy = jsonencode({
    redrivePermission = "byQueue"
    sourceQueueArns   = [aws_sqs_queue.source.arn]
  })
}

data "aws_iam_policy_document" "source_queue" {
  statement {
    sid     = "AllowEventBridgeToSendOrders"
    effect  = "Allow"
    actions = ["sqs:SendMessage"]

    principals {
      type        = "Service"
      identifiers = ["events.amazonaws.com"]
    }

    resources = [aws_sqs_queue.source.arn]
  }
}

resource "aws_sqs_queue_policy" "source" {
  queue_url = aws_sqs_queue.source.id
  policy    = data.aws_iam_policy_document.source_queue.json
}
