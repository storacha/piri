output "instance_id" {
  description = "ID of the EC2 instance"
  value       = aws_instance.piri.id
}

output "public_ip" {
  description = "Public IP address of the EC2 instance"
  value       = aws_instance.piri.public_ip
}

output "domain_name" {
  description = "Domain name for the Piri instance"
  value       = local.domain_name
}

output "ssh_command" {
  description = "SSH command to connect to the instance"
  value       = "ssh -i <path-to-${var.key_name}.pem> ubuntu@${aws_instance.piri.public_ip}"
}

output "service_url" {
  description = "HTTPS URL for the Piri service"
  value       = "https://${local.domain_name}"
}

output "ebs_volume_id" {
  description = "ID of the EBS data volume"
  value       = aws_ebs_volume.piri_data.id
}