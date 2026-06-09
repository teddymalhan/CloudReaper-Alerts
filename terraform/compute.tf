resource "aws_eks_cluster" "orders" {
  name     = "${var.name_prefix}-eks"
  role_arn = aws_iam_role.eks_cluster.arn
  version  = "1.30"

  vpc_config {
    endpoint_private_access = true
    endpoint_public_access  = false
    security_group_ids      = [aws_security_group.eks_control_plane.id]
    subnet_ids              = [aws_subnet.private_a.id, aws_subnet.private_b.id]
  }

  tags = local.tags

  depends_on = [aws_iam_role_policy.eks_cluster]
}

