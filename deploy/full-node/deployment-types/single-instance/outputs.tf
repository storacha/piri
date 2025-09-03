output "instance_id" {
  description = "ID of the EC2 instance"
  value       = module.piri_instance.instance_id
}

output "public_ip" {
  description = "Public IP address of the instance"
  value       = module.piri_instance.public_ip
}

output "url" {
  description = "URL to access the Piri service"
  value       = "https://${module.piri_instance.domain_name}"
}

output "ssh_command" {
  description = "SSH command to connect to the instance"
  value       = "ssh -i ~/.ssh/${var.key_name}.pem ubuntu@${module.piri_instance.public_ip}"
}