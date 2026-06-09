resource "aws_dynamodb_table" "idempotency" {
  name         = "${var.name_prefix}-idempotency"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "message_id"
  range_key    = "business_key"

  attribute {
    name = "message_id"
    type = "S"
  }

  attribute {
    name = "business_key"
    type = "S"
  }

  ttl {
    attribute_name = "expires_at"
    enabled        = true
  }

  point_in_time_recovery {
    enabled = true
  }

  server_side_encryption {
    enabled = true
  }

  tags = merge(local.tags, {
    Name = "${var.name_prefix}-idempotency"
  })
}

resource "aws_dynamodb_table" "orders" {
  name         = "${var.name_prefix}-orders"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "order_id"

  attribute {
    name = "order_id"
    type = "S"
  }

  point_in_time_recovery {
    enabled = true
  }

  server_side_encryption {
    enabled = true
  }

  tags = merge(local.tags, {
    Name = "${var.name_prefix}-orders"
  })
}

resource "aws_s3_bucket" "quarantine" {
  bucket        = "${var.name_prefix}-quarantine-local"
  force_destroy = true

  tags = merge(local.tags, {
    Name = "${var.name_prefix}-quarantine-local"
  })
}

resource "aws_s3_bucket_versioning" "quarantine" {
  bucket = aws_s3_bucket.quarantine.id

  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "quarantine" {
  bucket = aws_s3_bucket.quarantine.id

  rule {
    apply_server_side_encryption_by_default {
      kms_master_key_id = aws_kms_key.work.arn
      sse_algorithm     = "aws:kms"
    }
  }
}

resource "aws_s3_bucket_public_access_block" "quarantine" {
  bucket = aws_s3_bucket.quarantine.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}
