resource "aws_cloudwatch_event_bus" "orders" {
  name = "${var.name_prefix}-events"

  tags = local.tags
}

resource "aws_cloudwatch_event_rule" "orders_to_source_queue" {
  name           = "${var.name_prefix}-orders-to-source-queue"
  description    = "Route normalized order work events to the FIFO source queue."
  event_bus_name = aws_cloudwatch_event_bus.orders.name

  event_pattern = jsonencode({
    source      = ["orders.api", "orders.replay"]
    detail-type = ["orders.work"]
  })

  tags = local.tags
}

resource "aws_cloudwatch_event_target" "source_queue" {
  rule           = aws_cloudwatch_event_rule.orders_to_source_queue.name
  event_bus_name = aws_cloudwatch_event_bus.orders.name
  target_id      = "source-queue"
  arn            = aws_sqs_queue.source.arn

  sqs_target {
    message_group_id = "orders"
  }
}

data "aws_iam_policy_document" "api_gateway_assume_role" {
  statement {
    actions = ["sts:AssumeRole"]
    effect  = "Allow"

    principals {
      type        = "Service"
      identifiers = ["apigateway.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "api_gateway_events" {
  name               = "${var.name_prefix}-api-gateway-events"
  assume_role_policy = data.aws_iam_policy_document.api_gateway_assume_role.json

}

resource "aws_iam_role_policy" "api_gateway_events" {
  name = "${var.name_prefix}-api-gateway-events"
  role = aws_iam_role.api_gateway_events.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["events:PutEvents"]
      Resource = aws_cloudwatch_event_bus.orders.arn
    }]
  })
}

resource "aws_apigatewayv2_api" "orders" {
  name          = "${var.name_prefix}-http-api"
  protocol_type = "HTTP"

  tags = local.tags
}

resource "aws_apigatewayv2_integration" "put_events" {
  api_id              = aws_apigatewayv2_api.orders.id
  credentials_arn     = aws_iam_role.api_gateway_events.arn
  integration_type    = "AWS_PROXY"
  integration_subtype = "EventBridge-PutEvents"

  request_parameters = {
    Detail       = "$request.body"
    DetailType   = "orders.work"
    EventBusName = aws_cloudwatch_event_bus.orders.name
    Source       = "orders.api"
  }

  lifecycle {
    ignore_changes = [
      credentials_arn,
      integration_subtype,
      request_templates,
      timeout_milliseconds
    ]
  }

  depends_on = [aws_iam_role_policy.api_gateway_events]
}

resource "aws_apigatewayv2_route" "post_orders" {
  api_id    = aws_apigatewayv2_api.orders.id
  route_key = "POST /orders"
  target    = "integrations/${aws_apigatewayv2_integration.put_events.id}"
}

resource "aws_apigatewayv2_stage" "local" {
  api_id      = aws_apigatewayv2_api.orders.id
  name        = "local"
  auto_deploy = true


}
