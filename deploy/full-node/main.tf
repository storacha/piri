locals {
  config_toml_content = templatefile("${path.module}/files/config.toml.tpl", {
    pdp_contract_address = var.pdp_contract_address
    pdp_lotus_endpoint   = var.pdp_lotus_endpoint
    pdp_owner_address    = var.pdp_owner_address
    server_public_url    = "https://${local.domain_name}"
    indexer_proof        = var.indexer_proof
    proof_set            = var.proof_set
  })

  nginx_conf_content = templatefile("${path.module}/files/nginx.conf.tpl", {
    server_name = local.domain_name
  })

  systemd_service_content = file("${path.module}/files/piri.service")
  
  install_from_release_script = file("${path.module}/scripts/install-from-release.sh")
  install_from_branch_script  = file("${path.module}/scripts/install-from-branch.sh")
}