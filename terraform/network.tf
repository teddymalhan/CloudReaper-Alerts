resource "aws_vpc" "private" {
  cidr_block           = "10.42.0.0/16"
  enable_dns_support   = true
  enable_dns_hostnames = true

  tags = merge(local.tags, {
    Name = "${var.name_prefix}-vpc"
  })
}

resource "aws_subnet" "private_a" {
  vpc_id            = aws_vpc.private.id
  cidr_block        = "10.42.1.0/24"
  availability_zone = "${var.aws_region}a"

  tags = merge(local.tags, {
    Name = "${var.name_prefix}-private-a"
  })
}

resource "aws_subnet" "private_b" {
  vpc_id            = aws_vpc.private.id
  cidr_block        = "10.42.2.0/24"
  availability_zone = "${var.aws_region}b"

  tags = merge(local.tags, {
    Name = "${var.name_prefix}-private-b"
  })
}

resource "aws_security_group" "eks_control_plane" {
  name        = "${var.name_prefix}-eks-control-plane"
  description = "Local EKS control plane security group"
  vpc_id      = aws_vpc.private.id

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(local.tags, {
    Name = "${var.name_prefix}-eks-control-plane"
  })
}

