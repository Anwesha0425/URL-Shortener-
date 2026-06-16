variable "aws_region"   { default = "ap-south-1" }
variable "environment"  { default = "production" }
variable "cluster_name" { default = "url-shortener" }
variable "db_password"  { sensitive = true }

# ─────────────────────────────────────────────────────────────────
# VPC
# ─────────────────────────────────────────────────────────────────
resource "aws_vpc" "main" {
  cidr_block           = "10.0.0.0/16"
  enable_dns_hostnames = true
  enable_dns_support   = true
}

resource "aws_subnet" "private" {
  count             = 2
  vpc_id            = aws_vpc.main.id
  cidr_block        = "10.0.${count.index}.0/24"
  availability_zone = data.aws_availability_zones.available.names[count.index]
  tags = { Name = "url-shortener-private-${count.index}" }
}

resource "aws_subnet" "public" {
  count                   = 2
  vpc_id                  = aws_vpc.main.id
  cidr_block              = "10.0.${count.index + 10}.0/24"
  availability_zone       = data.aws_availability_zones.available.names[count.index]
  map_public_ip_on_launch = true
  tags = { Name = "url-shortener-public-${count.index}" }
}

data "aws_availability_zones" "available" {}

# ─────────────────────────────────────────────────────────────────
# EKS Cluster
# ─────────────────────────────────────────────────────────────────
resource "aws_eks_cluster" "main" {
  name     = var.cluster_name
  role_arn = aws_iam_role.eks_cluster.arn
  version  = "1.28"

  vpc_config {
    subnet_ids              = concat(aws_subnet.private[*].id, aws_subnet.public[*].id)
    endpoint_private_access = true
    endpoint_public_access  = true
  }

  depends_on = [aws_iam_role_policy_attachment.eks_cluster_policy]
}

resource "aws_eks_node_group" "workers" {
  cluster_name    = aws_eks_cluster.main.name
  node_group_name = "url-shortener-workers"
  node_role_arn   = aws_iam_role.eks_node.arn
  subnet_ids      = aws_subnet.private[*].id
  instance_types  = ["t3.medium"]

  scaling_config {
    desired_size = 3
    max_size     = 20
    min_size     = 2
  }

  update_config {
    max_unavailable = 1
  }
}

# ─────────────────────────────────────────────────────────────────
# RDS PostgreSQL (Multi-AZ for HA)
# ─────────────────────────────────────────────────────────────────
resource "aws_db_instance" "postgres" {
  identifier              = "url-shortener-pg"
  engine                  = "postgres"
  engine_version          = "16.1"
  instance_class          = "db.r6g.large"
  allocated_storage       = 100
  max_allocated_storage   = 1000          # Auto-scaling storage
  storage_encrypted       = true
  multi_az                = true          # High availability
  db_name                 = "urldb"
  username                = "urluser"
  password                = var.db_password
  db_subnet_group_name    = aws_db_subnet_group.main.name
  vpc_security_group_ids  = [aws_security_group.rds.id]
  backup_retention_period = 7             # 7 days PITR
  deletion_protection     = true
  skip_final_snapshot     = false
  final_snapshot_identifier = "url-shortener-final"

  performance_insights_enabled = true
  monitoring_interval          = 60       # Enhanced monitoring
}

resource "aws_db_subnet_group" "main" {
  name       = "url-shortener-db-subnet"
  subnet_ids = aws_subnet.private[*].id
}

# ─────────────────────────────────────────────────────────────────
# ElastiCache Redis (Cluster mode)
# ─────────────────────────────────────────────────────────────────
resource "aws_elasticache_replication_group" "redis" {
  replication_group_id = "url-shortener-redis"
  description          = "URL shortener Redis cache"
  node_type            = "cache.r6g.large"
  num_cache_clusters   = 3               # 1 primary + 2 replicas
  parameter_group_name = "default.redis7"
  engine_version       = "7.0"
  port                 = 6379
  subnet_group_name    = aws_elasticache_subnet_group.main.name
  security_group_ids   = [aws_security_group.redis.id]
  at_rest_encryption_enabled = true
  transit_encryption_enabled = true
  automatic_failover_enabled = true
}

resource "aws_elasticache_subnet_group" "main" {
  name       = "url-shortener-redis-subnet"
  subnet_ids = aws_subnet.private[*].id
}

# ─────────────────────────────────────────────────────────────────
# Security Groups
# ─────────────────────────────────────────────────────────────────
resource "aws_security_group" "rds" {
  name   = "url-shortener-rds"
  vpc_id = aws_vpc.main.id

  ingress {
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [aws_security_group.eks_workers.id]
  }
}

resource "aws_security_group" "redis" {
  name   = "url-shortener-redis"
  vpc_id = aws_vpc.main.id

  ingress {
    from_port       = 6379
    to_port         = 6379
    protocol        = "tcp"
    security_groups = [aws_security_group.eks_workers.id]
  }
}

resource "aws_security_group" "eks_workers" {
  name   = "url-shortener-eks-workers"
  vpc_id = aws_vpc.main.id

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

# ─────────────────────────────────────────────────────────────────
# IAM Roles
# ─────────────────────────────────────────────────────────────────
resource "aws_iam_role" "eks_cluster" {
  name = "url-shortener-eks-cluster"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{ Effect = "Allow", Principal = { Service = "eks.amazonaws.com" }, Action = "sts:AssumeRole" }]
  })
}

resource "aws_iam_role_policy_attachment" "eks_cluster_policy" {
  role       = aws_iam_role.eks_cluster.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"
}

resource "aws_iam_role" "eks_node" {
  name = "url-shortener-eks-node"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{ Effect = "Allow", Principal = { Service = "ec2.amazonaws.com" }, Action = "sts:AssumeRole" }]
  })
}

# ─────────────────────────────────────────────────────────────────
# Outputs
# ─────────────────────────────────────────────────────────────────
output "eks_cluster_endpoint"  { value = aws_eks_cluster.main.endpoint }
output "rds_endpoint"          { value = aws_db_instance.postgres.endpoint }
output "redis_endpoint"        { value = aws_elasticache_replication_group.redis.primary_endpoint_address }
