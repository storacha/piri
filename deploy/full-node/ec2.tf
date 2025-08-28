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
  subnet_id              = aws_subnet.public.id
  vpc_security_group_ids = [aws_security_group.piri.id]
  iam_instance_profile   = aws_iam_instance_profile.piri.name

  root_block_device {
    volume_type = "gp3"
    volume_size = 30
    encrypted   = false
  }

  user_data = templatefile("${path.module}/scripts/user-data.sh.tpl", {
    install_method       = var.install_method
    install_source       = var.install_source
    domain_name          = local.domain_name
    operator_email       = var.operator_email
    service_pem_content  = var.service_pem_content
    wallet_hex_content   = var.wallet_hex_content
    nginx_conf_content   = local.nginx_conf_content
    systemd_service_content = local.systemd_service_content
    install_from_release_script = local.install_from_release_script
    install_from_branch_script  = local.install_from_branch_script
  })

  tags = {
    Name        = "piri-${var.environment}"
    Environment = var.environment
  }

  depends_on = [aws_internet_gateway.piri]
}

resource "aws_ebs_volume" "piri_data" {
  availability_zone = aws_instance.piri.availability_zone
  size              = var.ebs_volume_size
  type              = "gp3"

  tags = {
    Name        = "piri-data-${var.environment}"
    Environment = var.environment
  }
  
  # Note: This volume will persist if instance is manually terminated in console
  # Use terraform destroy to ensure proper cleanup
}

resource "aws_volume_attachment" "piri_data" {
  device_name = "/dev/sdf"
  volume_id   = aws_ebs_volume.piri_data.id
  instance_id = aws_instance.piri.id
}