variable "app" {
  description = "name of the application"
  type        = string
  default     = "storage"
}

variable "owner" {
  description = "owner of the resources"
  type        = string
  default     = "storacha"
}

variable "team" {
  description = "name of team managing working on the project"
  type        = string
  default     = "Storacha Engineering"
}

variable "org" {
  description = "name of the organization managing the project"
  type        = string
  default     = "Storacha"
}

variable "domain" {
  description = "domain name to use for the deployment (will be prefixed with app name)"
  type        = string
  default     = "storacha.network"
}

variable "use_pdp" {
  description = "is this a deployment that uses pdp"
  type        = bool
  default     = false
}

variable "region" {
  description = "aws region for all services"
  type        = string
  default     = "us-west-2"
}

variable "allowed_account_ids" {
  description = "account IDs used for AWS"
  type        = list(string)
  default     = ["0"]
}

variable "private_key" {
  description = "private_key for the peer for this deployment"
  type        = string
}

variable "presets" {
  description = "presets to use for the deployment"
  type        = string
  default     = "prod"
}

variable "indexing_service_did" {
  description = "did to use for the indexer"
  type        = string
  default     = "did:web:indexer.storacha.network"
}

variable "indexing_service_url" {
  description = "url to use for the indexer"
  type        = string
  default     = "https://indexer.storacha.network/claims"
}

variable "indexing_service_proof" {
  description = "UCAN delegation to prove this storage node can access the indexer"
  type        = string
}

variable "pdp_proofset" {
  description = "proofset used with pdp"
  type        = number
  default     = 0
}

variable "curio_url" {
  description = "url for the curio SP to communicate with"
  type        = string
  default     = ""
}

variable "access_logging_log_format" {
  type        = string
  description = "The log format to use for access logging."
  default     = "{\"apiId\": \"$context.apiId\", \"requestId\": \"$context.requestId\", \"extendedRequestId\": \"$context.extendedRequestId\", \"httpMethod\": \"$context.httpMethod\", \"path\": \"$context.path\", \"protocol\": \"$context.protocol\", \"requestTime\": \"$context.requestTime\", \"requestTimeEpoch\": \"$context.requestTimeEpoch\", \"status\": $context.status, \"responseLatency\": $context.responseLatency, \"responseLength\": $context.responseLength}"
}

variable "principal_mapping" {
  type        = string
  description = "JSON encoded mapping of did:web to did:key"
  default     = ""
}

variable "blob_bucket_key_pattern" {
  type        = string
  description = "Optional key pattern (with {blob} specifier) for blob bucket"
  default     = "blob/{blob}"
}

variable "sentry_dsn" {
  type        = string
  description = "DSN for Sentry (get it from your Sentry project's properties). Leave unset to disable error reporting."
  default     = ""
}

variable "sentry_environment" {
  type        = string
  description = "Environment name for Sentry"
  default     = ""
}

variable "ipni_announce_urls" {
  type        = string
  description = "Optional JSON array of IPNI node URLs to announce chain updates to."
  default     = "[\"https://cid.contact\"]"
}

// Externally hosted, S3 compatible blob bucket? These variables are for you.
// Note: credentials MUST have s3:GetObject, s3:PutObject s3:ListBucket perms.

variable "use_external_blob_bucket" {
  type        = bool
  description = "Is the blob bucket externally hosted (but S3 compatible)?"
  default     = false
}

variable "external_blob_bucket_endpoint" {
  type        = string
  description = "Optional endpoint of an external blob bucket"
  default     = ""
}

variable "external_blob_bucket_region" {
  type        = string
  description = "Optional region of an external blob bucket"
  default     = ""
}

variable "external_blob_bucket_name" {
  type        = string
  description = "Optional name of an external blob bucket"
  default     = ""
}

variable "external_blob_bucket_domain" {
  type        = string
  description = "Optional domain name for the external blob bucket."
  default     = ""
}

variable "external_blob_bucket_access_key_id" {
  type        = string
  description = "Optional access key ID for external blob bucket"
  default     = ""
}

variable "external_blob_bucket_secret_access_key" {
  type        = string
  description = "Optional secret access key for external blob bucket"
  default     = ""
}
