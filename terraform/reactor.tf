# Event-driven trigger (real AWS only — gated by var.enable_reactor):
#   EC2 API call ─► CloudTrail ─► EventBridge rule ─► reactor Lambda ─► SQS (main) ─► notifier ─► Slack
#
# The reactor reacts the moment a resource may have been orphaned, instead of waiting for the
# scheduled scan. It enqueues to the SAME main queue the scanner uses, so everything from SQS
# onward is shared. CloudTrail is unsupported on Floci, so this whole file is a no-op unless
# enable_reactor=true (every resource is counted on the flag).

locals {
  reactor_count = var.enable_reactor ? 1 : 0
}

# ── CloudTrail (delivers "AWS API Call via CloudTrail" events to EventBridge) ─────────────────────
# A trail logging management events in this region is required for EventBridge to see EC2 API calls.
resource "aws_s3_bucket" "trail" {
  count         = local.reactor_count
  bucket        = "${var.project}-orphan-trail-${local.account_id}"
  force_destroy = true
  tags          = local.common_tags
}

data "aws_iam_policy_document" "trail_bucket" {
  count = local.reactor_count

  statement {
    sid       = "AWSCloudTrailAclCheck"
    actions   = ["s3:GetBucketAcl"]
    resources = [aws_s3_bucket.trail[0].arn]
    principals {
      type        = "Service"
      identifiers = ["cloudtrail.amazonaws.com"]
    }
  }

  statement {
    sid       = "AWSCloudTrailWrite"
    actions   = ["s3:PutObject"]
    resources = ["${aws_s3_bucket.trail[0].arn}/AWSLogs/${local.account_id}/*"]
    principals {
      type        = "Service"
      identifiers = ["cloudtrail.amazonaws.com"]
    }
    condition {
      test     = "StringEquals"
      variable = "s3:x-amz-acl"
      values   = ["bucket-owner-full-control"]
    }
  }
}

resource "aws_s3_bucket_policy" "trail" {
  count  = local.reactor_count
  bucket = aws_s3_bucket.trail[0].id
  policy = data.aws_iam_policy_document.trail_bucket[0].json
}

resource "aws_cloudtrail" "orphan" {
  count                         = local.reactor_count
  name                          = "${var.project}-orphan-trail"
  s3_bucket_name                = aws_s3_bucket.trail[0].id
  include_global_service_events = false
  is_multi_region_trail         = false
  enable_logging                = true
  tags                          = local.common_tags

  depends_on = [aws_s3_bucket_policy.trail]
}

# ── EventBridge rule: the EC2 actions that can orphan a resource ───────────────────────────────────
# Note: ReleaseAddress is intentionally excluded — it deletes the EIP (cleanup), so it never orphans.
resource "aws_cloudwatch_event_rule" "ec2_orphan_actions" {
  count       = local.reactor_count
  name        = "${var.project}-orphan-ec2-actions"
  description = "EC2 API calls that can leave a resource orphaned"
  tags        = local.common_tags

  event_pattern = jsonencode({
    source      = ["aws.ec2"]
    detail-type = ["AWS API Call via CloudTrail"]
    detail = {
      eventSource = ["ec2.amazonaws.com"]
      eventName   = ["DetachVolume", "DisassociateAddress", "TerminateInstances"]
    }
  })
}

resource "aws_cloudwatch_event_target" "reactor" {
  count     = local.reactor_count
  rule      = aws_cloudwatch_event_rule.ec2_orphan_actions[0].name
  target_id = "reactor-lambda"
  arn       = aws_lambda_function.reactor[0].arn
}

resource "aws_lambda_permission" "allow_eventbridge" {
  count         = local.reactor_count
  statement_id  = "AllowEventBridgeInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.reactor[0].function_name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.ec2_orphan_actions[0].arn
}

# ── Reactor Lambda + role ──────────────────────────────────────────────────────────────────────────
resource "aws_iam_role" "reactor" {
  count              = local.reactor_count
  name               = "${var.project}-orphan-reactor"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume.json # reused from pipeline.tf
  tags               = local.common_tags
}

resource "aws_iam_role_policy" "reactor" {
  count = local.reactor_count
  name  = "${var.project}-orphan-reactor"
  role  = aws_iam_role.reactor[0].id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid      = "InspectResources"
        Effect   = "Allow"
        Action   = ["ec2:DescribeVolumes", "ec2:DescribeAddresses", "ec2:DescribeInstances"]
        Resource = "*"
      },
      {
        Sid      = "EnqueueAlert"
        Effect   = "Allow"
        Action   = ["sqs:SendMessage"]
        Resource = aws_sqs_queue.main.arn
      },
      {
        Sid      = "Logs"
        Effect   = "Allow"
        Action   = ["logs:CreateLogGroup", "logs:CreateLogStream", "logs:PutLogEvents"]
        Resource = "*"
      }
    ]
  })
}

resource "aws_lambda_function" "reactor" {
  count            = local.reactor_count
  function_name    = "${var.project}-orphan-reactor"
  role             = aws_iam_role.reactor[0].arn
  runtime          = "provided.al2023"
  handler          = "bootstrap"
  filename         = var.reactor_zip
  source_code_hash = filebase64sha256(var.reactor_zip)
  timeout          = 25

  environment {
    variables = {
      QUEUE_URL        = aws_sqs_queue.main.url
      AWS_ENDPOINT_URL = var.lambda_internal_endpoint
    }
  }

  tags = local.common_tags
}
