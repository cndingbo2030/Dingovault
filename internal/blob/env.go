package blob

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// NewProviderFromEnv picks filesystem or S3/MinIO storage from environment variables.
//
// DINGO_BLOB_BACKEND: "filesystem" (default), "s3", or "minio" (alias of s3).
//
// Filesystem (default when vaultRoot non-empty):
//   - Writes to vaultRoot/assets
//
// S3 / MinIO:
//   - DINGO_S3_BUCKET (required)
//   - DINGO_S3_REGION (default us-east-1)
//   - DINGO_S3_ENDPOINT (optional; e.g. http://localhost:9000 for MinIO)
//   - DINGO_S3_PREFIX (optional key prefix inside bucket)
//   - DINGO_S3_PUBLIC_BASE (required) base URL for markdown links, no trailing slash
//     e.g. https://mybucket.s3.us-east-1.amazonaws.com or https://cdn.example.com/mybucket
//   - DINGO_S3_USE_PATH_STYLE=1 for path-style addressing (typical MinIO)
//   - AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY (or set DINGO_S3_ACCESS_KEY / DINGO_S3_SECRET_KEY)
func NewProviderFromEnv(ctx context.Context, vaultRoot string) (Provider, error) {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("DINGO_BLOB_BACKEND")))
	if mode == "" {
		mode = "filesystem"
	}
	switch mode {
	case "s3", "minio":
		ak := strings.TrimSpace(os.Getenv("DINGO_S3_ACCESS_KEY"))
		sk := strings.TrimSpace(os.Getenv("DINGO_S3_SECRET_KEY"))
		if ak == "" {
			ak = strings.TrimSpace(os.Getenv("AWS_ACCESS_KEY_ID"))
		}
		if sk == "" {
			sk = strings.TrimSpace(os.Getenv("AWS_SECRET_ACCESS_KEY"))
		}
		pathStyle := strings.TrimSpace(os.Getenv("DINGO_S3_USE_PATH_STYLE")) == "1" ||
			strings.EqualFold(strings.TrimSpace(os.Getenv("DINGO_S3_USE_PATH_STYLE")), "true")

		cfg := S3Config{
			Region:          strings.TrimSpace(os.Getenv("DINGO_S3_REGION")),
			Bucket:          strings.TrimSpace(os.Getenv("DINGO_S3_BUCKET")),
			Endpoint:        strings.TrimSpace(os.Getenv("DINGO_S3_ENDPOINT")),
			AccessKeyID:     ak,
			SecretAccessKey: sk,
			KeyPrefix:       strings.TrimSpace(os.Getenv("DINGO_S3_PREFIX")),
			PublicBaseURL:   strings.TrimSpace(os.Getenv("DINGO_S3_PUBLIC_BASE")),
			UsePathStyle:    pathStyle,
		}
		if cfg.Region == "" {
			cfg.Region = "us-east-1"
		}
		return NewS3Store(ctx, cfg)

	case "filesystem", "fs", "local":
		vr := strings.TrimSpace(vaultRoot)
		if vr == "" {
			return nil, nil
		}
		return NewFileSystem(vr), nil

	default:
		return nil, fmt.Errorf("unknown DINGO_BLOB_BACKEND %q", mode)
	}
}
