variable "aws_region" {
  description = "AWS region (LocalStack)."
  type        = string
  default     = "us-east-1"
}

variable "aws_endpoint" {
  description = "Floci (AWS emulator) gateway endpoint, from the host."
  type        = string
  default     = "http://localhost:4566"
}

variable "lambda_internal_endpoint" {
  description = "AWS endpoint the notifier Lambda uses from inside Floci's docker network. Empty = rely on emulator auto-injection / real AWS."
  type        = string
  default     = "http://floci:4566"
}

variable "project" {
  description = "Name prefix / Project tag for all resources."
  type        = string
  default     = "nimbuskart"
}

variable "environment" {
  description = "Environment tag."
  type        = string
  default     = "staging"
}

variable "owner" {
  description = "Owner tag."
  type        = string
  default     = "devops-team"
}

variable "availability_zone" {
  description = "AZ for subnet / EBS volume placement."
  type        = string
  default     = "us-east-1a"
}

variable "lambda_zip" {
  description = "Path to the built notifier Lambda zip (bootstrap inside)."
  type        = string
  default     = "build/notifier.zip"
}

variable "slack_bot_token" {
  description = "Slack bot token (xoxb-...) for the notifier Lambda. Placeholder works for non-Slack local tests."
  type        = string
  default     = "xoxb-localstack-placeholder"
  sensitive   = true
}

variable "slack_channel_id" {
  description = "Slack channel ID the notifier posts to."
  type        = string
  default     = "C0000000000"
}
