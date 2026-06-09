provider "aws" {
  region     = var.aws_region
  access_key = "test"
  secret_key = "test"

  skip_credentials_validation = true
  skip_metadata_api_check     = true
  skip_requesting_account_id  = true
  s3_use_path_style           = true

  endpoints {
    apigateway     = var.aws_endpoint
    apigatewayv2   = var.aws_endpoint
    cloudwatch     = var.aws_endpoint
    cloudwatchlogs = var.aws_endpoint
    dynamodb       = var.aws_endpoint
    ec2            = var.aws_endpoint
    eks            = var.aws_endpoint
    events         = var.aws_endpoint
    iam            = var.aws_endpoint
    kms            = var.aws_endpoint
    s3             = var.aws_endpoint
    secretsmanager = var.aws_endpoint
    sns            = var.aws_endpoint
    sqs            = var.aws_endpoint
    ssm            = var.aws_endpoint
    sfn            = var.aws_endpoint
    sts            = var.aws_endpoint
    xray           = var.aws_endpoint
  }
}
