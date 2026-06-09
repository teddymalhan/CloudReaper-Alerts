data "aws_iam_policy_document" "states_assume_role" {
  statement {
    actions = ["sts:AssumeRole"]
    effect  = "Allow"

    principals {
      type        = "Service"
      identifiers = ["states.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "remediation" {
  name               = "${var.name_prefix}-remediation"
  assume_role_policy = data.aws_iam_policy_document.states_assume_role.json

}

resource "aws_iam_role_policy" "remediation" {
  name = "${var.name_prefix}-remediation"
  role = aws_iam_role.remediation.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid      = "SendReplayMessage"
        Effect   = "Allow"
        Action   = ["sqs:SendMessage"]
        Resource = aws_sqs_queue.replay.arn
      },
      {
        Sid      = "WriteQuarantineObject"
        Effect   = "Allow"
        Action   = ["s3:PutObject"]
        Resource = "${aws_s3_bucket.quarantine.arn}/*"
      },
      {
        Sid      = "UseWorkKey"
        Effect   = "Allow"
        Action   = ["kms:Encrypt", "kms:GenerateDataKey"]
        Resource = aws_kms_key.work.arn
      }
    ]
  })
}

locals {
  remediation_definition = jsonencode({
    Comment = "Manual DLQ remediation: approve replay or quarantine payloads."
    StartAt = "Decision"
    States = {
      Decision = {
        Type = "Choice"
        Choices = [
          {
            Variable     = "$.decision"
            StringEquals = "replay"
            Next         = "SendToReplayQueue"
          },
          {
            Variable     = "$.decision"
            StringEquals = "quarantine"
            Next         = "QuarantinePayload"
          }
        ]
        Default = "RejectUnknownDecision"
      }
      SendToReplayQueue = {
        Type     = "Task"
        Resource = "arn:aws:states:::sqs:sendMessage"
        Parameters = {
          QueueUrl = aws_sqs_queue.replay.url
          MessageBody = {
            "original.$" = "$.message"
            "reason.$"   = "$.reason"
          }
          MessageGroupId = "orders-replay"
        }
        End = true
      }
      QuarantinePayload = {
        Type     = "Task"
        Resource = "arn:aws:states:::aws-sdk:s3:putObject"
        Parameters = {
          Bucket               = aws_s3_bucket.quarantine.bucket
          "Key.$"              = "States.Format('dlq/{}.json', $.messageId)"
          "Body.$"             = "States.JsonToString($)"
          ServerSideEncryption = "aws:kms"
          SSEKMSKeyId          = aws_kms_key.work.arn
        }
        End = true
      }
      RejectUnknownDecision = {
        Type  = "Fail"
        Error = "InvalidDecision"
        Cause = "decision must be replay or quarantine"
      }
    }
  })
}

resource "terraform_data" "remediation_state_machine" {
  triggers_replace = {
    definition_sha = sha256(local.remediation_definition)
    role_arn       = aws_iam_role.remediation.arn
  }

  provisioner "local-exec" {
    command = "aws --endpoint-url=\"$AWS_ENDPOINT_URL\" stepfunctions delete-state-machine --state-machine-arn \"$STATE_MACHINE_ARN\" || true; aws --endpoint-url=\"$AWS_ENDPOINT_URL\" stepfunctions create-state-machine --name \"$STATE_MACHINE_NAME\" --role-arn \"$ROLE_ARN\" --definition \"$DEFINITION\""

    environment = {
      AWS_ACCESS_KEY_ID     = "test"
      AWS_SECRET_ACCESS_KEY = "test"
      AWS_DEFAULT_REGION    = var.aws_region
      AWS_ENDPOINT_URL      = var.aws_endpoint
      DEFINITION            = local.remediation_definition
      ROLE_ARN              = aws_iam_role.remediation.arn
      STATE_MACHINE_ARN     = local.remediation_machine_arn
      STATE_MACHINE_NAME    = "${var.name_prefix}-remediation"
    }
  }

  depends_on = [aws_iam_role_policy.remediation]
}

