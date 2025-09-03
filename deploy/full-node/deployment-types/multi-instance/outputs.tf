output "instances" {
  description = "Details of all Piri instances"
  value = {
    for k, v in module.piri_instances : k => {
      instance_id = v.instance_id
      public_ip   = v.public_ip
      domain_name = v.domain_name
      url         = "https://${v.domain_name}"
    }
  }
}

output "ssh_commands" {
  description = "SSH commands to connect to each instance"
  value = {
    for k, v in module.piri_instances : k => "ssh -i ~/.ssh/${var.key_name}.pem ubuntu@${v.public_ip}"
  }
}

output "urls" {
  description = "URLs to access each Piri service"
  value = {
    for k, v in module.piri_instances : k => "https://${v.domain_name}"
  }
}