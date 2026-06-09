locals {
  account_id              = "000000000000"
  oidc_provider_host      = trimprefix(var.eks_oidc_issuer_url, "https://")
  oidc_provider_arn       = "arn:aws:iam::${local.account_id}:oidc-provider/${local.oidc_provider_host}"
  remediation_machine_arn = "arn:aws:states:${var.aws_region}:${local.account_id}:stateMachine:${var.name_prefix}-remediation"

  tags = {
    Application = "sqs-dlq-on-eks"
    Environment = var.environment
    ManagedBy   = "terraform-opentofu"
  }
}
