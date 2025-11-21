locals {
  functions = {
    getclaim = {
      name  = "GETclaim"
      route = "GET /claim/{cid}"
    }
    getroot = {
      name  = "GETroot"
      route = "GET /"
    }
    publisher = {
      name = "publisher"
    }
    advertisementpublisher = {
      name        = "advertisementpublisher"
      concurrency = 1
    }
    postad = {
      name  = "POSTad"
      route = "POST /ad"
    }
    postroot = {
      name  = "POSTroot"
      route = "POST /"
    }
    putblob = {
      name  = "PUTblob"
      route = "PUT /blob/{blob}"
    }
  }
}

// zip the binary, as we can use only zip files to AWS lambda
data "archive_file" "function_archive" {
  for_each = local.functions

  type        = "zip"
  source_file = "${path.root}/../build/${each.key}/bootstrap"
  output_path = "${path.root}/../build/${each.key}/${each.key}.zip"
}

# Define functions

resource "aws_lambda_function" "lambda" {
  depends_on = [aws_cloudwatch_log_group.lambda_log_group]
  for_each   = local.functions

  function_name                  = "${terraform.workspace}-${var.app}-lambda-${each.value.name}"
  handler                        = "bootstrap"
  runtime                        = "provided.al2023"
  architectures                  = ["arm64"]
  role                           = aws_iam_role.lambda_exec.arn
  timeout                        = try(each.value.timeout, 60)
  memory_size                    = try(each.value.memory_size, 128)
  reserved_concurrent_executions = try(each.value.concurrency, -1)
  source_code_hash               = data.archive_file.function_archive[each.key].output_base64sha256
  filename                       = data.archive_file.function_archive[each.key].output_path # Path to your Lambda zip files

  environment {
    variables = {
      SENTRY_DSN                             = var.sentry_dsn
      SENTRY_ENVIRONMENT                     = var.sentry_environment == "" ? terraform.workspace : var.sentry_environment
      CHUNK_LINKS_TABLE_NAME                 = aws_dynamodb_table.chunk_links.id
      METADATA_TABLE_NAME                    = aws_dynamodb_table.metadata.id
      IPNI_STORE_BUCKET_NAME                 = aws_s3_bucket.ipni_store_bucket.bucket
      IPNI_ANNOUNCE_URLS                     = var.ipni_announce_urls
      PRIVATE_KEY                            = aws_ssm_parameter.private_key.name
      PUBLIC_URL                             = "https://${aws_apigatewayv2_domain_name.custom_domain.domain_name}"
      IPNI_STORE_BUCKET_REGIONAL_DOMAIN      = aws_s3_bucket.ipni_store_bucket.bucket_regional_domain_name
      CLAIM_STORE_BUCKET_NAME                = aws_s3_bucket.claim_store_bucket.bucket
      ALLOCATIONS_TABLE_NAME                 = aws_dynamodb_table.allocation_store.id
      ACCEPTANCE_TABLE_NAME                  = aws_dynamodb_table.acceptance_store.id
      BLOB_STORE_BUCKET_ENDPOINT             = var.use_external_blob_bucket ? var.external_blob_bucket_endpoint : ""
      BLOB_STORE_BUCKET_REGION               = var.use_external_blob_bucket ? var.external_blob_bucket_region : aws_s3_bucket.blob_store_bucket.region
      BLOB_STORE_BUCKET_ACCESS_KEY_ID        = var.use_external_blob_bucket ? aws_ssm_parameter.external_blob_bucket_access_key_id[0].name : ""
      BLOB_STORE_BUCKET_SECRET_ACCESS_KEY    = var.use_external_blob_bucket ? aws_ssm_parameter.external_blob_bucket_secret_access_key[0].name : ""
      BLOB_STORE_BUCKET_REGIONAL_DOMAIN      = var.use_external_blob_bucket ? var.external_blob_bucket_domain : aws_s3_bucket.blob_store_bucket.bucket_regional_domain_name
      BLOB_STORE_BUCKET_NAME                 = var.use_external_blob_bucket ? var.external_blob_bucket_name : aws_s3_bucket.blob_store_bucket.bucket
      BLOB_STORE_BUCKET_KEY_PATTERN          = var.blob_bucket_key_pattern
      INDEXING_SERVICE_DID                   = var.indexing_service_did
      INDEXING_SERVICE_URL                   = var.indexing_service_url
      INDEXING_SERVICE_PROOF                 = var.indexing_service_proof
      RAN_LINK_INDEX_TABLE_NAME              = aws_dynamodb_table.ran_link_index.id
      RECEIPT_STORE_BUCKET_NAME              = aws_s3_bucket.receipt_store_bucket.id
      IPNI_PUBLISHER_QUEUE_ID                = aws_sqs_queue.ipni_publisher.id
      IPNI_PUBLISHER_BUCKET_NAME             = aws_s3_bucket.ipni_publisher.bucket
      IPNI_ADVERTISEMENT_PUBLISHING_QUEUE_ID = aws_sqs_queue.ipni_advertisement_publishing.id
      PRINCIPAL_MAPPING                      = var.principal_mapping,
      PIRI_PRESETS                           = var.presets,
    }
  }
}

# Access for the gateway

resource "aws_lambda_permission" "api_gateway" {
  for_each = aws_lambda_function.lambda

  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = each.value.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.api.execution_arn}/*/*"
}

# Logging

resource "aws_cloudwatch_log_group" "lambda_log_group" {
  for_each          = local.functions
  name              = "/aws/lambda/${terraform.workspace}-${var.app}-lambda-${each.value.name}"
  retention_in_days = 7
  lifecycle {
    prevent_destroy = false
  }
}

# Role policies and access to resources

resource "aws_iam_role" "lambda_exec" {
  name = "${terraform.workspace}-${var.app}-lambda-exec-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "lambda.amazonaws.com"
        }
      }
    ]
  })
}

data "aws_iam_policy_document" "lambda_dynamodb_put_get_document" {
  statement {
    actions = [
      "dynamodb:GetItem",
      "dynamodb:PutItem",
      "dynamodb:Query"
    ]
    resources = [
      aws_dynamodb_table.chunk_links.arn,
      aws_dynamodb_table.metadata.arn,
      aws_dynamodb_table.ran_link_index.arn,
      aws_dynamodb_table.allocation_store.arn,
      aws_dynamodb_table.acceptance_store.arn
    ]
  }
}

resource "aws_iam_policy" "lambda_dynamodb_put_get" {
  name        = "${terraform.workspace}-${var.app}-lambda-dynamodb-put-get"
  description = "This policy will be used by the lambda to put and get data from DynamoDB"
  policy      = data.aws_iam_policy_document.lambda_dynamodb_put_get_document.json
}

resource "aws_iam_role_policy_attachment" "lambda_dynamodb_put_get" {
  role       = aws_iam_role.lambda_exec.name
  policy_arn = aws_iam_policy.lambda_dynamodb_put_get.arn
}


data "aws_iam_policy_document" "lambda_s3_put_get_document" {
  statement {
    actions = [
      "s3:GetObject",
      "s3:PutObject",
      "s3:HeadObject",
    ]
    resources = [
      "${aws_s3_bucket.blob_store_bucket.arn}/*",
      "${aws_s3_bucket.ipni_store_bucket.arn}/*",
      "${aws_s3_bucket.receipt_store_bucket.arn}/*",
      "${aws_s3_bucket.claim_store_bucket.arn}/*",
      "${aws_s3_bucket.ipni_publisher.arn}/*",
    ]
  }

  statement {
    actions = [
      "s3:ListBucket", "s3:GetBucketLocation"
    ]
    resources = [
      aws_s3_bucket.blob_store_bucket.arn,
      aws_s3_bucket.ipni_store_bucket.arn,
      aws_s3_bucket.receipt_store_bucket.arn,
      aws_s3_bucket.claim_store_bucket.arn,
      aws_s3_bucket.ipni_publisher.arn,
    ]
  }
}

resource "aws_iam_policy" "lambda_s3_put_get" {
  name        = "${terraform.workspace}-${var.app}-lambda-s3-put-get"
  description = "This policy will be used by the lambda to put and get objects from S3"
  policy      = data.aws_iam_policy_document.lambda_s3_put_get_document.json
}

resource "aws_iam_role_policy_attachment" "lambda_s3_put_get" {
  role       = aws_iam_role.lambda_exec.name
  policy_arn = aws_iam_policy.lambda_s3_put_get.arn
}

data "aws_iam_policy_document" "lambda_logs_document" {
  statement {
    actions = [
      "logs:CreateLogStream",
      "logs:PutLogEvents",
    ]
    resources = [
      "arn:aws:logs:*:*:*"
    ]
  }
}

resource "aws_iam_policy" "lambda_logs" {
  name        = "${terraform.workspace}-${var.app}-lambda-logs"
  description = "This policy will be used by the lambda to write logs"
  policy      = data.aws_iam_policy_document.lambda_logs_document.json
}

resource "aws_iam_role_policy_attachment" "lambda_logs" {
  role       = aws_iam_role.lambda_exec.name
  policy_arn = aws_iam_policy.lambda_logs.arn
}

data "aws_iam_policy_document" "lambda_ssm_document" {
  statement {

    effect = "Allow"

    actions = [
      "ssm:GetParameters",
    ]

    resources = var.use_external_blob_bucket ? [
      aws_ssm_parameter.private_key.arn,
      aws_ssm_parameter.external_blob_bucket_access_key_id[0].arn,
      aws_ssm_parameter.external_blob_bucket_secret_access_key[0].arn,
    ] : [aws_ssm_parameter.private_key.arn]
  }
}

resource "aws_iam_policy" "lambda_ssm" {
  name        = "${terraform.workspace}-${var.app}-lambda-ssm"
  description = "This policy will be used by the lambda to access the parameter store"
  policy      = data.aws_iam_policy_document.lambda_ssm_document.json
}

resource "aws_iam_role_policy_attachment" "lambda_ssm" {
  role       = aws_iam_role.lambda_exec.name
  policy_arn = aws_iam_policy.lambda_ssm.arn
}

data "aws_iam_policy_document" "lambda_sqs_document" {
  statement {

    effect = "Allow"

    actions = [
      "sqs:SendMessage*",
      "sqs:ReceiveMessage",
      "sqs:DeleteMessage",
      "sqs:GetQueueAttributes"
    ]

    resources = [
      aws_sqs_queue.ipni_publisher.arn,
      aws_sqs_queue.ipni_advertisement_publishing.arn
    ]
  }
}

resource "aws_iam_policy" "lambda_sqs" {
  name        = "${terraform.workspace}-${var.app}-lambda-sqs"
  description = "This policy will be used by the lambda to send messages to an SQS queue"
  policy      = data.aws_iam_policy_document.lambda_sqs_document.json
}

resource "aws_iam_role_policy_attachment" "lambda_sqs" {
  role       = aws_iam_role.lambda_exec.name
  policy_arn = aws_iam_policy.lambda_sqs.arn
}

# event source mappings


resource "aws_lambda_event_source_mapping" "ipni_publisher_source_mapping" {
  event_source_arn = aws_sqs_queue.ipni_publisher.arn
  enabled          = true
  function_name    = aws_lambda_function.lambda["publisher"].arn
  batch_size       = terraform.workspace == "prod" ? 10 : 1
}

resource "aws_lambda_event_source_mapping" "ipni_advertisement_publishing_source_mapping" {
  event_source_arn = aws_sqs_queue.ipni_advertisement_publishing.arn
  enabled          = true
  function_name    = aws_lambda_function.lambda["advertisementpublisher"].arn
  batch_size       = terraform.workspace == "prod" ? 10 : 1
}
