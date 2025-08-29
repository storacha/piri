locals {
  nginx_conf_content = templatefile("${path.module}/files/nginx.conf.tpl", {
    server_name = var.domain_name
  })

  systemd_service_content = templatefile("${path.module}/files/piri.service.tpl", {
    registrar_url  = var.registrar_url
    lotus_endpoint = var.pdp_lotus_endpoint
    operator_email = var.operator_email
    public_url     = "https://${var.domain_name}"
  })
  
  install_from_release_script = file("${path.module}/scripts/install-from-release.sh")
  install_from_branch_script  = file("${path.module}/scripts/install-from-branch.sh")
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
  })

  tags = merge(
    var.tags,
    {
      Name        = "piri-${var.environment}-${var.name}"
      Environment = var.environment
      Instance    = var.name
    }
  )

  depends_on = [var.internet_gateway_id]
}

resource "aws_ebs_volume" "piri_data" {
  availability_zone = aws_instance.piri.availability_zone
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
}

resource "aws_volume_attachment" "piri_data" {
  device_name = "/dev/sdf"
  volume_id   = aws_ebs_volume.piri_data.id
  instance_id = aws_instance.piri.id
}

resource "aws_route53_record" "piri" {
  zone_id = var.route53_zone_id
  name    = var.domain_name
  type    = "A"
  ttl     = 300
  records = [aws_instance.piri.public_ip]
}