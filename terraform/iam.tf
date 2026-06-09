data "aws_iam_policy_document" "eks_assume_role" {
  statement {
    actions = ["sts:AssumeRole"]
    effect  = "Allow"

    principals {
      type        = "Service"
      identifiers = ["eks.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "eks_cluster" {
  name               = "${var.name_prefix}-eks-cluster"
  assume_role_policy = data.aws_iam_policy_document.eks_assume_role.json

}

resource "aws_iam_role_policy" "eks_cluster" {
  name = "${var.name_prefix}-eks-cluster"
  role = aws_iam_role.eks_cluster.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Action = [
        "ec2:Describe*",
        "eks:*",
        "iam:PassRole"
      ]
      Resource = "*"
    }]
  })
}


data "aws_iam_policy_document" "worker_assume_role" {
  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]
    effect  = "Allow"

    principals {
      type        = "Federated"
      identifiers = [local.oidc_provider_arn]
    }

    condition {
      test     = "StringEquals"
      variable = "${local.oidc_provider_host}:sub"
      values   = ["system:serviceaccount:orders:order-worker"]
    }
  }
}

resource "aws_iam_role" "worker" {
  name               = "${var.name_prefix}-worker"
  assume_role_policy = data.aws_iam_policy_document.worker_assume_role.json

}

resource "aws_iam_role_policy" "worker" {
  name = "${var.name_prefix}-worker"
  role = aws_iam_role.worker.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "ConsumeSourceQueue"
        Effect = "Allow"
        Action = [
          "sqs:ReceiveMessage",
          "sqs:DeleteMessage",
          "sqs:ChangeMessageVisibility",
          "sqs:GetQueueAttributes",
          "sqs:GetQueueUrl"
        ]
        Resource = aws_sqs_queue.source.arn
      },
      {
        Sid    = "WriteOrdersAndIdempotency"
        Effect = "Allow"
        Action = [
          "dynamodb:GetItem",
          "dynamodb:PutItem",
          "dynamodb:UpdateItem",
          "dynamodb:ConditionCheckItem"
        ]
        Resource = [
          aws_dynamodb_table.idempotency.arn,
          aws_dynamodb_table.orders.arn
        ]
      },
      {
        Sid      = "ReadRuntimeSecret"
        Effect   = "Allow"
        Action   = ["secretsmanager:GetSecretValue"]
        Resource = aws_secretsmanager_secret.runtime.arn
      },
      {
        Sid      = "DecryptWorkData"
        Effect   = "Allow"
        Action   = ["kms:Decrypt", "kms:GenerateDataKey"]
        Resource = aws_kms_key.work.arn
      },
      {
        Sid    = "EmitTelemetry"
        Effect = "Allow"
        Action = [
          "logs:CreateLogStream",
          "logs:PutLogEvents",
          "xray:PutTraceSegments",
          "xray:PutTelemetryRecords"
        ]
        Resource = "*"
      }
    ]
  })
}

data "aws_iam_policy_document" "triage_assume_role" {
  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]
    effect  = "Allow"

    principals {
      type        = "Federated"
      identifiers = [local.oidc_provider_arn]
    }

    condition {
      test     = "StringEquals"
      variable = "${local.oidc_provider_host}:sub"
      values   = ["system:serviceaccount:orders:dlq-triage"]
    }
  }
}

resource "aws_iam_role" "triage" {
  name               = "${var.name_prefix}-triage"
  assume_role_policy = data.aws_iam_policy_document.triage_assume_role.json

}

resource "aws_iam_role_policy" "triage" {
  name = "${var.name_prefix}-triage"
  role = aws_iam_role.triage.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "ReadAndDrainDlq"
        Effect = "Allow"
        Action = [
          "sqs:ReceiveMessage",
          "sqs:DeleteMessage",
          "sqs:ChangeMessageVisibility",
          "sqs:GetQueueAttributes",
          "sqs:GetQueueUrl"
        ]
        Resource = aws_sqs_queue.dlq.arn
      },
      {
        Sid    = "ReplayMessages"
        Effect = "Allow"
        Action = ["sqs:SendMessage", "sqs:GetQueueUrl"]
        Resource = [
          aws_sqs_queue.replay.arn,
          aws_sqs_queue.source.arn
        ]
      },
      {
        Sid      = "StartRemediation"
        Effect   = "Allow"
        Action   = ["states:StartExecution"]
        Resource = local.remediation_machine_arn
      },
      {
        Sid      = "WriteQuarantine"
        Effect   = "Allow"
        Action   = ["s3:PutObject"]
        Resource = "${aws_s3_bucket.quarantine.arn}/*"
      },
      {
        Sid      = "UseWorkKey"
        Effect   = "Allow"
        Action   = ["kms:Decrypt", "kms:Encrypt", "kms:GenerateDataKey"]
        Resource = aws_kms_key.work.arn
      }
    ]
  })
}

