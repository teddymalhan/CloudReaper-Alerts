resource "aws_kms_key" "work" {
  description             = "Local CMK for SQS DLQ architecture resources"
  deletion_window_in_days = 7
  enable_key_rotation     = true

  tags = merge(local.tags, {
    Name = "${var.name_prefix}-kms"
  })
}

resource "aws_kms_alias" "work" {
  name          = "alias/${var.name_prefix}-local"
  target_key_id = aws_kms_key.work.key_id
}

resource "aws_secretsmanager_secret" "runtime" {
  name                    = "${var.name_prefix}/runtime"
  description             = "Runtime secret placeholder; values are injected outside Terraform to avoid state exposure."
  kms_key_id              = aws_kms_key.work.arn
  recovery_window_in_days = 0

  tags = local.tags
}
