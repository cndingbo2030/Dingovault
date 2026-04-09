package blob

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

// S3Store uploads to AWS S3 or S3-compatible APIs (e.g. MinIO).
type S3Store struct {
	client     *s3.Client
	bucket     string
	keyPrefix  string
	publicBase string // e.g. https://cdn.example.com or https://minio:9000/mybucket
}

// S3Config configures an S3Store.
type S3Config struct {
	Region          string
	Bucket          string
	Endpoint        string // optional; set for MinIO/custom
	AccessKeyID     string
	SecretAccessKey string
	KeyPrefix       string // optional logical prefix inside the bucket
	PublicBaseURL   string // required for meaningful markdown links (no trailing slash)
	UsePathStyle    bool   // typical for MinIO
}

// NewS3Store builds an S3 client and store.
func NewS3Store(ctx context.Context, cfg S3Config) (*S3Store, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("S3 bucket required")
	}
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	if strings.TrimSpace(cfg.PublicBaseURL) == "" {
		return nil, fmt.Errorf("S3 public base URL required for markdown links (set DINGO_S3_PUBLIC_BASE)")
	}

	loadOpts := []func(*config.LoadOptions) error{
		config.WithRegion(cfg.Region),
	}
	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		loadOpts = append(loadOpts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = cfg.UsePathStyle
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
	})

	prefix := strings.Trim(cfg.KeyPrefix, "/")
	return &S3Store{
		client:     client,
		bucket:     cfg.Bucket,
		keyPrefix:  prefix,
		publicBase: strings.TrimRight(strings.TrimSpace(cfg.PublicBaseURL), "/"),
	}, nil
}

// Put implements Provider.
func (s *S3Store) Put(ctx context.Context, in PutInput) (PutResult, error) {
	ext := strings.ToLower(path.Ext(in.FileName))
	if ext == "" {
		ext = ".bin"
	}
	tenant := strings.TrimSpace(in.TenantID)
	if tenant == "" {
		tenant = "anonymous"
	}
	tenant = sanitizePathSegment(tenant)
	keyUUID := uuid.NewString()
	objectKey := path.Join(s.keyPrefix, "assets", tenant, keyUUID+ext)
	objectKey = strings.ReplaceAll(objectKey, "\\", "/")

	body, err := io.ReadAll(io.LimitReader(in.Body, in.Limit))
	if err != nil {
		return PutResult{}, fmt.Errorf("read body: %w", err)
	}
	if len(body) == 0 {
		return PutResult{}, fmt.Errorf("empty upload")
	}

	ct := strings.TrimSpace(in.ContentType)
	input := &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(objectKey),
		Body:        bytes.NewReader(body),
		ContentType: awsStringOrNil(ct),
	}
	_, err = s.client.PutObject(ctx, input)
	if err != nil {
		return PutResult{}, fmt.Errorf("s3 put: %w", err)
	}

	publicURL := s.publicBase + "/" + objectKey
	md := MarkdownLink(in.FileName, publicURL, ext) + "\n"
	return PutResult{
		Ref:      publicURL,
		Bytes:    int64(len(body)),
		Markdown: md,
	}, nil
}

func sanitizePathSegment(s string) string {
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "..", "_")
	if s == "" {
		return "_"
	}
	return s
}

func awsStringOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return aws.String(s)
}
