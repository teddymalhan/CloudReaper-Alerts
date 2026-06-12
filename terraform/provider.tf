provider "aws" {
  region     = var.aws_region
  access_key = "test"
  secret_key = "test"

  # LocalStack: skip real AWS validation and route every service to the local gateway.
  skip_credentials_validation = true
  skip_metadata_api_check     = true
  skip_requesting_account_id  = true
  s3_use_path_style           = true

  endpoints {
    apigateway     = var.aws_endpoint
    cloudwatchlogs = var.aws_endpoint
    ec2            = var.aws_endpoint
    iam            = var.aws_endpoint
    lambda         = var.aws_endpoint
    s3             = var.aws_endpoint
    secretsmanager = var.aws_endpoint
    sqs            = var.aws_endpoint
    sts            = var.aws_endpoint
  }
}
