locals {
  nginx_conf_content = templatefile("${path.module}/files/nginx.conf.tpl", {
    server_name = local.domain_name
  })

  systemd_service_content = templatefile("${path.module}/files/piri.service.tpl", {
    registrar_url  = var.registrar_url
    lotus_endpoint = var.pdp_lotus_endpoint
    operator_email = var.operator_email
    public_url     = "https://${local.domain_name}"
  })
  
  install_from_release_script = file("${path.module}/scripts/install-from-release.sh")
  install_from_branch_script  = file("${path.module}/scripts/install-from-branch.sh")
}