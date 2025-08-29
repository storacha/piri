output "instance_id" {
  description = "ID of the EC2 instance"
  value       = aws_instance.piri.id
}

output "public_ip" {
  description = "Public IP address of the instance"
  value       = aws_instance.piri.public_ip
}

output "public_dns" {
  description = "Public DNS name of the instance"
  value       = aws_instance.piri.public_dns
}

output "domain_name" {
  description = "Full domain name of the instance"
  value       = var.domain_name
}

output "ebs_volume_id" {
  description = "ID of the EBS data volume"
  value       = var.protect_volume ? aws_ebs_volume.piri_data_protected[0].id : aws_ebs_volume.piri_data[0].id
}