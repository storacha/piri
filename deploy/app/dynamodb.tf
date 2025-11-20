resource "aws_dynamodb_table" "metadata" {
  name         = "${terraform.workspace}-${var.app}-metadata"
  billing_mode = "PAY_PER_REQUEST"

  attribute {
    name = "provider"
    type = "S"
  }

  attribute {
    name = "contextID"
    type = "B"
  }

  hash_key  = "provider"
  range_key = "contextID"

  tags = {
    Name = "${terraform.workspace}-${var.app}-metadata"
  }

  point_in_time_recovery {
    enabled = terraform.workspace == "prod"
  }

  deletion_protection_enabled = terraform.workspace == "prod"
}

resource "aws_dynamodb_table" "chunk_links" {
  name         = "${terraform.workspace}-${var.app}-chunk-links"
  billing_mode = "PAY_PER_REQUEST"

  attribute {
    name = "provider"
    type = "S"
  }

  attribute {
    name = "contextID"
    type = "B"
  }

  hash_key  = "provider"
  range_key = "contextID"

  tags = {
    Name = "${terraform.workspace}-${var.app}-chunk-links"
  }

  point_in_time_recovery {
    enabled = terraform.workspace == "prod"
  }

  deletion_protection_enabled = terraform.workspace == "prod"
}

resource "aws_dynamodb_table" "ran_link_index" {
  name         = "${terraform.workspace}-${var.app}-ran-link-index"
  billing_mode = "PAY_PER_REQUEST"

  attribute {
    name = "ran"
    type = "S"
  }

  attribute {
    name = "link"
    type = "S"
  }

  hash_key  = "ran"
  range_key = "link"

  tags = {
    Name = "${terraform.workspace}-${var.app}-ran-link-index"
  }

  point_in_time_recovery {
    enabled = terraform.workspace == "prod"
  }

  deletion_protection_enabled = terraform.workspace == "prod"
}


resource "aws_dynamodb_table" "allocation_store" {
  name         = "${terraform.workspace}-${var.app}-allocation-store"
  billing_mode = "PAY_PER_REQUEST"

  attribute {
    name = "hash"
    type = "S"
  }

  # note: now contains a space DID not invocation CID
  attribute {
    name = "cause"
    type = "S"
  }

  hash_key  = "hash"
  range_key = "cause"

  tags = {
    Name = "${terraform.workspace}-${var.app}-allocation-store"
  }

  point_in_time_recovery {
    enabled = terraform.workspace == "prod"
  }

  deletion_protection_enabled = terraform.workspace == "prod"
}

resource "aws_dynamodb_table" "acceptance_store" {
  name         = "${terraform.workspace}-${var.app}-acceptance-store"
  billing_mode = "PAY_PER_REQUEST"

  attribute {
    name = "hash"
    type = "S"
  }

  attribute {
    name = "space"
    type = "S"
  }

  hash_key  = "hash"
  range_key = "space"

  tags = {
    Name = "${terraform.workspace}-${var.app}-acceptance-store"
  }

  point_in_time_recovery {
    enabled = terraform.workspace == "prod"
  }

  deletion_protection_enabled = terraform.workspace == "prod"
}
