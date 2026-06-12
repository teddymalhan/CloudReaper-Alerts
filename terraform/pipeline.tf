# The Slack delivery pipeline (mirrors slacked):
#   API Gateway POST /send-message ─► SQS (main, with DLQ) ─► notifier Lambda ─► Slack

# ── Queues ──────────────────────────────────────────────────────────────────────
resource "aws_sqs_queue" "dlq" {
  name                      = "${var.project}-orphan-dlq"
  message_retention_seconds = 1209600 # 14 days
  tags                      = merge(local.common_tags, { Name = "${var.project}-orphan-dlq" })
}

resource "aws_sqs_queue" "main" {
  name                       = "${var.project}-orphan-queue"
  visibility_timeout_seconds = 150
  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.dlq.arn
    maxReceiveCount     = 3
  })
  tags = merge(local.common_tags, { Name = "${var.project}-orphan-queue" })
}

# ── Slack credentials in Secrets Manager ─────────────────────────────────────────
resource "aws_secretsmanager_secret" "slack" {
  name = "orphan-watch/slack-credentials"
  tags = local.common_tags
}

resource "aws_secretsmanager_secret_version" "slack" {
  secret_id = aws_secretsmanager_secret.slack.id
  secret_string = jsonencode({
    SLACK_BOT_TOKEN  = var.slack_bot_token
    SLACK_CHANNEL_ID = var.slack_channel_id
  })
}

# ── Notifier Lambda + role ───────────────────────────────────────────────────────
data "aws_iam_policy_document" "lambda_assume" {
  statement {
    actions = ["sts:AssumeRole"]
    effect  = "Allow"
    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "lambda" {
  name               = "${var.project}-orphan-notifier"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume.json
  tags               = local.common_tags
}

resource "aws_iam_role_policy" "lambda" {
  name = "${var.project}-orphan-notifier"
  role = aws_iam_role.lambda.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid      = "ConsumeQueue"
        Effect   = "Allow"
        Action   = ["sqs:ReceiveMessage", "sqs:DeleteMessage", "sqs:GetQueueAttributes"]
        Resource = aws_sqs_queue.main.arn
      },
      {
        Sid      = "ReadSlackSecret"
        Effect   = "Allow"
        Action   = ["secretsmanager:GetSecretValue"]
        Resource = aws_secretsmanager_secret.slack.arn
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

resource "aws_lambda_function" "notifier" {
  function_name    = "${var.project}-orphan-notifier"
  role             = aws_iam_role.lambda.arn
  runtime          = "provided.al2023"
  handler          = "bootstrap"
  filename         = var.lambda_zip
  source_code_hash = filebase64sha256(var.lambda_zip)
  timeout          = 25

  environment {
    variables = {
      SECRET_NAME = aws_secretsmanager_secret.slack.name
    }
  }

  tags = local.common_tags
}

resource "aws_lambda_event_source_mapping" "sqs_to_lambda" {
  event_source_arn = aws_sqs_queue.main.arn
  function_name    = aws_lambda_function.notifier.arn
  batch_size       = 1
  enabled          = true
}

# ── API Gateway POST /send-message → SQS ─────────────────────────────────────────
resource "aws_api_gateway_rest_api" "orphan" {
  name = "${var.project}-orphan-api"
  tags = local.common_tags
}

resource "aws_api_gateway_resource" "send" {
  rest_api_id = aws_api_gateway_rest_api.orphan.id
  parent_id   = aws_api_gateway_rest_api.orphan.root_resource_id
  path_part   = "send-message"
}

resource "aws_api_gateway_method" "post" {
  rest_api_id   = aws_api_gateway_rest_api.orphan.id
  resource_id   = aws_api_gateway_resource.send.id
  http_method   = "POST"
  authorization = "NONE"
}

data "aws_iam_policy_document" "apigw_assume" {
  statement {
    actions = ["sts:AssumeRole"]
    effect  = "Allow"
    principals {
      type        = "Service"
      identifiers = ["apigateway.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "apigw_sqs" {
  name               = "${var.project}-orphan-apigw"
  assume_role_policy = data.aws_iam_policy_document.apigw_assume.json
  tags               = local.common_tags
}

resource "aws_iam_role_policy" "apigw_sqs" {
  name = "${var.project}-orphan-apigw"
  role = aws_iam_role.apigw_sqs.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["sqs:SendMessage"]
      Resource = aws_sqs_queue.main.arn
    }]
  })
}

# AWS service integration: forward the request body as the SQS message body.
resource "aws_api_gateway_integration" "sqs" {
  rest_api_id             = aws_api_gateway_rest_api.orphan.id
  resource_id             = aws_api_gateway_resource.send.id
  http_method             = aws_api_gateway_method.post.http_method
  type                    = "AWS"
  integration_http_method = "POST"
  credentials             = aws_iam_role.apigw_sqs.arn
  uri                     = "arn:aws:apigateway:${var.aws_region}:sqs:path/${local.account_id}/${aws_sqs_queue.main.name}"

  request_parameters = {
    "integration.request.header.Content-Type" = "'application/x-www-form-urlencoded'"
  }
  request_templates = {
    "application/json" = "Action=SendMessage&MessageBody=$util.urlEncode($input.body)"
  }
}

resource "aws_api_gateway_method_response" "ok" {
  rest_api_id = aws_api_gateway_rest_api.orphan.id
  resource_id = aws_api_gateway_resource.send.id
  http_method = aws_api_gateway_method.post.http_method
  status_code = "200"
}

resource "aws_api_gateway_integration_response" "ok" {
  rest_api_id = aws_api_gateway_rest_api.orphan.id
  resource_id = aws_api_gateway_resource.send.id
  http_method = aws_api_gateway_method.post.http_method
  status_code = aws_api_gateway_method_response.ok.status_code

  response_templates = {
    "application/json" = jsonencode({ status = "message queued to SQS" })
  }

  depends_on = [aws_api_gateway_integration.sqs]
}

resource "aws_api_gateway_deployment" "orphan" {
  rest_api_id = aws_api_gateway_rest_api.orphan.id

  triggers = {
    redeploy = sha1(jsonencode([
      aws_api_gateway_resource.send.id,
      aws_api_gateway_method.post.id,
      aws_api_gateway_integration.sqs.id,
    ]))
  }

  lifecycle {
    create_before_destroy = true
  }

  depends_on = [aws_api_gateway_integration.sqs]
}

resource "aws_api_gateway_stage" "prod" {
  rest_api_id   = aws_api_gateway_rest_api.orphan.id
  deployment_id = aws_api_gateway_deployment.orphan.id
  stage_name    = "prod"
  tags          = local.common_tags
}
