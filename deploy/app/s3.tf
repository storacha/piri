resource "aws_s3_bucket" "ipni_store_bucket" {
  bucket = "${terraform.workspace}-${var.app}-ipni-store-bucket"
}

resource "aws_s3_bucket_public_access_block" "ipni_store_bucket" {
  bucket = aws_s3_bucket.ipni_store_bucket.id

  block_public_acls       = true
  block_public_policy     = false
  ignore_public_acls      = true
  restrict_public_buckets = false
}

resource "aws_s3_bucket_cors_configuration" "ipni_store_cors" {
  bucket = aws_s3_bucket.ipni_store_bucket.bucket

  cors_rule {
    allowed_headers = ["*"]
    allowed_methods = ["GET", "HEAD"]
    allowed_origins = ["*"]
    expose_headers  = ["Content-Length", "Content-Type", "Content-MD5", "ETag"]
    max_age_seconds = 86400
  }
}

resource "aws_s3_bucket_policy" "ipni_store_policy" {
  depends_on = [aws_s3_bucket_public_access_block.ipni_store_bucket]
  bucket     = aws_s3_bucket.ipni_store_bucket.id

  policy = jsonencode({
    "Version" : "2012-10-17",
    "Statement" : [
      {
        "Sid" : "PublicRead",
        "Effect" : "Allow",
        "Principal" : "*",
        "Action" : ["s3:GetObject", "s3:GetObjectVersion"],
        "Resource" : ["${aws_s3_bucket.ipni_store_bucket.arn}/*"]
      }
    ]
  })
}

resource "aws_s3_bucket" "blob_store_bucket" {
  bucket = "${terraform.workspace}-${var.app}-blob-store-bucket"
}

resource "aws_s3_bucket_public_access_block" "blob_store_bucket" {
  bucket = aws_s3_bucket.blob_store_bucket.id

  block_public_acls       = false
  block_public_policy     = false
  ignore_public_acls      = false
  restrict_public_buckets = false
}

resource "aws_s3_bucket_cors_configuration" "blob_store_cors" {
  bucket = aws_s3_bucket.blob_store_bucket.bucket

  cors_rule {
    allowed_headers = ["*"]
    allowed_methods = ["GET", "HEAD"]
    allowed_origins = ["*"]
    expose_headers  = ["Content-Length", "Content-Type", "Content-MD5", "ETag"]
    max_age_seconds = 86400
  }
}

resource "aws_s3_bucket_policy" "blob_store_policy" {
  depends_on = [aws_s3_bucket_public_access_block.blob_store_bucket]
  bucket     = aws_s3_bucket.blob_store_bucket.id

  policy = jsonencode({
    "Version" : "2012-10-17",
    "Statement" : [
      {
        "Sid" : "PublicRead",
        "Effect" : "Allow",
        "Principal" : "*",
        "Action" : ["s3:GetObject", "s3:GetObjectVersion"],
        "Resource" : ["${aws_s3_bucket.blob_store_bucket.arn}/*"]
      }
    ]
  })
}

resource "aws_s3_bucket" "receipt_store_bucket" {
  bucket = "${terraform.workspace}-${var.app}-receipt-store-bucket"
}

resource "aws_s3_bucket" "claim_store_bucket" {
  bucket = "${terraform.workspace}-${var.app}-claim-store-bucket"
}

resource "aws_s3_bucket" "buffer_bucket" {
  count  = var.use_pdp ? 1 : 0
  bucket = "${terraform.workspace}-${var.app}-buffer-bucket"
}

resource "aws_s3_bucket" "aggregates_bucket" {
  count  = var.use_pdp ? 1 : 0
  bucket = "${terraform.workspace}-${var.app}-aggregates-bucket"
}