locals {
  nginx_conf_content = templatefile("${path.module}/files/nginx.conf.tpl", {
    server_name = var.domain_name
  })

  # Determine if Docker is needed for native backends
  needs_docker = var.storage_backend == "minio" || var.database_backend == "postgres"

  systemd_service_content = templatefile("${path.module}/files/piri.service.tpl", {
    network          = var.network
    lotus_endpoint   = var.pdp_lotus_endpoint
    operator_email   = var.operator_email
    public_url       = "https://${var.domain_name}"
    needs_docker     = local.needs_docker
    storage_backend  = var.storage_backend
    database_backend = var.database_backend
    # PostgreSQL CLI flags
    postgres_url               = "postgres://${var.postgres_user}:${var.postgres_password}@localhost:5432/${var.postgres_database}?sslmode=disable"
    postgres_max_open_conns    = 10
    postgres_max_idle_conns    = 5
    postgres_conn_max_lifetime = "30m"
    # S3/MinIO CLI flags
    s3_endpoint          = "localhost:9000"
    s3_bucket_prefix     = var.minio_bucket_prefix
    s3_access_key_id     = var.minio_root_user
    s3_secret_access_key = var.minio_root_password
  })

  install_from_release_script = file("${path.module}/scripts/install-from-release.sh")
  install_from_branch_script  = file("${path.module}/scripts/install-from-branch.sh")

  # Docker Compose content
  docker_compose = local.needs_docker ? templatefile(
    "${path.module}/files/docker-compose.yml.tpl",
    {
      enable_postgres     = var.database_backend == "postgres"
      enable_minio        = var.storage_backend == "minio"
      postgres_user       = var.postgres_user
      postgres_password   = var.postgres_password
      postgres_database   = var.postgres_database
      minio_root_user     = var.minio_root_user
      minio_root_password = var.minio_root_password
    }
  ) : ""
}

data "aws_ami" "ubuntu" {
  most_recent = true
  owners      = ["099720109477"] # Canonical

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }
}

resource "aws_instance" "piri" {
  ami                    = data.aws_ami.ubuntu.id
  instance_type          = var.instance_type
  key_name               = var.key_name
  subnet_id              = var.subnet_id
  vpc_security_group_ids = [var.security_group_id]
  iam_instance_profile   = var.iam_instance_profile_name

  root_block_device {
    volume_type = "gp3"
    volume_size = 30
    encrypted   = false
  }

  user_data = templatefile("${path.module}/scripts/user-data.sh.tpl", {
    install_method              = coalesce(var.install_method, "version")
    install_source              = coalesce(var.install_source, "v0.0.12")
    domain_name                 = var.domain_name
    operator_email              = var.operator_email
    service_pem_content         = local.service_pem_content
    wallet_hex_content          = local.wallet_hex_content
    nginx_conf_content          = local.nginx_conf_content
    systemd_service_content     = local.systemd_service_content
    install_from_release_script = local.install_from_release_script
    install_from_branch_script  = local.install_from_branch_script
    use_letsencrypt_staging     = var.use_letsencrypt_staging
    # Backend configuration
    storage_backend  = var.storage_backend
    database_backend = var.database_backend
    needs_docker     = local.needs_docker
    docker_compose   = local.docker_compose
  })

  tags = merge(
    var.tags,
    {
      Name        = "piri-${var.environment}-${var.name}"
      Environment = var.environment
      Instance    = var.name
    }
  )

  lifecycle {
    # Force replacement when user_data changes (includes install_source changes)
    create_before_destroy = true
  }

  # Force replacement when user_data changes
  user_data_replace_on_change = true

  depends_on = [var.internet_gateway_id]
}

# Volume without protection (default)
resource "aws_ebs_volume" "piri_data" {
  count = var.protect_volume ? 0 : 1

  availability_zone = var.availability_zone
  size              = var.ebs_volume_size
  type              = "gp3"

  tags = merge(
    var.tags,
    {
      Name        = "piri-data-${var.environment}-${var.name}"
      Environment = var.environment
      Instance    = var.name
    }
  )

  lifecycle {
    ignore_changes = [size] # Allow manual resizing outside of Terraform
  }
}

# Volume with protection
resource "aws_ebs_volume" "piri_data_protected" {
  count = var.protect_volume ? 1 : 0

  availability_zone = var.availability_zone
  size              = var.ebs_volume_size
  type              = "gp3"

  tags = merge(
    var.tags,
    {
      Name        = "piri-data-${var.environment}-${var.name}"
      Environment = var.environment
      Instance    = var.name
    }
  )

  lifecycle {
    prevent_destroy = true
    ignore_changes  = [size] # Allow manual resizing outside of Terraform
  }
}

resource "aws_volume_attachment" "piri_data" {
  device_name = "/dev/sdf"
  volume_id   = var.protect_volume ? aws_ebs_volume.piri_data_protected[0].id : aws_ebs_volume.piri_data[0].id
  instance_id = aws_instance.piri.id

  # Force detach on instance replacement to allow reattachment
  force_detach = true
}

resource "aws_route53_record" "piri" {
  zone_id = var.route53_zone_id
  name    = var.domain_name
  type    = "A"
  ttl     = 300
  records = [aws_instance.piri.public_ip]
}