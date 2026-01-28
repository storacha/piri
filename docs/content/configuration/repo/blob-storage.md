# blob-storage

Configure external S3-compatible storage for blobs. This section is optional.

```toml
[repo.blob_storage.minio]
endpoint = "s3.example.com:9000"
bucket = "piri-blobs"
insecure = false

[repo.blob_storage.minio.credentials]
access_key_id = "..."
secret_access_key = "..."
```
