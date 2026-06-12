# Resources the detector scans. Two are deliberately "orphaned" so a run produces findings;
# the tagged EC2 instances are healthy and should be ignored.

resource "aws_vpc" "main" {
  cidr_block = "10.20.0.0/16"
  tags       = merge(local.common_tags, { Name = "${var.project}-vpc" })
}

resource "aws_subnet" "main" {
  vpc_id            = aws_vpc.main.id
  cidr_block        = "10.20.1.0/24"
  availability_zone = var.availability_zone
  tags              = merge(local.common_tags, { Name = "${var.project}-subnet" })
}

# Healthy, fully tagged instances — the detector should NOT flag these.
resource "aws_instance" "web" {
  count         = 2
  ami           = "ami-00000000"
  instance_type = "t3.micro"
  subnet_id     = aws_subnet.main.id

  tags = merge(local.common_tags, {
    Name = "${var.project}-web-${count.index + 1}"
    Tier = "web"
  })
}

# ORPHAN #1: an unattached EBS volume (status "available").
resource "aws_ebs_volume" "orphan" {
  availability_zone = var.availability_zone
  size              = 8
  tags              = merge(local.common_tags, { Name = "${var.project}-orphan-vol" })
}

# ORPHAN #2: an unassociated Elastic IP.
resource "aws_eip" "orphan" {
  domain = "vpc"
  tags   = merge(local.common_tags, { Name = "${var.project}-orphan-eip" })
}
