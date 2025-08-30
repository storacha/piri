# Option to use AWS Secrets Manager instead of direct variables
data "aws_secretsmanager_secret" "piri_service_key" {
  count = var.use_secrets_manager ? 1 : 0
  name  = "${var.environment}/piri/${var.name}/service-key"
}

data "aws_secretsmanager_secret_version" "piri_service_key" {
  count     = var.use_secrets_manager ? 1 : 0
  secret_id = data.aws_secretsmanager_secret.piri_service_key[0].id
}

data "aws_secretsmanager_secret" "piri_wallet_key" {
  count = var.use_secrets_manager ? 1 : 0
  name  = "${var.environment}/piri/${var.name}/wallet-key"
}

data "aws_secretsmanager_secret_version" "piri_wallet_key" {
  count     = var.use_secrets_manager ? 1 : 0
  secret_id = data.aws_secretsmanager_secret.piri_wallet_key[0].id
}

locals {
  # Use Secrets Manager if enabled, otherwise use variables
  service_pem_content = var.use_secrets_manager ? data.aws_secretsmanager_secret_version.piri_service_key[0].secret_string : var.service_pem_content
  wallet_hex_content  = var.use_secrets_manager ? data.aws_secretsmanager_secret_version.piri_wallet_key[0].secret_string : var.wallet_hex_content
}