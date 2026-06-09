variable "aws_region" {
  description = "AWS region used by Floci and resource names."
  type        = string
  default     = "us-east-1"
}

variable "aws_endpoint" {
  description = "Floci edge endpoint."
  type        = string
  default     = "http://localhost:4566"
}

variable "name_prefix" {
  description = "Prefix for all locally emulated resources."
  type        = string
  default     = "orders-work"
}

variable "environment" {
  description = "Environment tag value."
  type        = string
  default     = "local"
}

variable "eks_oidc_issuer_url" {
  description = "OIDC issuer URL used to model IRSA trust for local EKS service accounts."
  type        = string
  default     = "https://oidc.eks.us-east-1.amazonaws.com/id/local-orders-work"
}
