data "aws_route53_zone" "primary" {
  name = var.root_domain
}

resource "aws_route53_record" "piri" {
  zone_id = data.aws_route53_zone.primary.zone_id
  name    = local.domain_name
  type    = "A"
  ttl     = 300
  records = [aws_instance.piri.public_ip]
}

locals {
  domain_name = "${var.environment}.${var.app}.${var.root_domain}"
}