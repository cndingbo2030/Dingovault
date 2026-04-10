package vaultsync

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Config targets a bucket prefix for bidirectional Markdown sync (compatible with AWS S3 and MinIO-style endpoints).
type S3Config struct {
	Region    string
	Bucket    string
	Prefix    string
	AccessKey string
	SecretKey string
	// Endpoint is optional (e.g. https://play.min.io for MinIO); empty uses AWS default for the region.
	Endpoint string
}

// SyncMarkdownVaultS3 mirrors .md files under localRoot with objects under cfg.Prefix in the bucket.
func SyncMarkdownVaultS3(ctx context.Context, localRoot string, cfg S3Config) error {
	localRoot = filepath.Clean(localRoot)
	if localRoot == "" {
		return fmt.Errorf("empty local root")
	}
	bucket := strings.TrimSpace(cfg.Bucket)
	if bucket == "" {
		return fmt.Errorf("empty S3 bucket")
	}
	client, err := dialS3(ctx, cfg)
	if err != nil {
		return err
	}
	prefix := normalizeS3Prefix(cfg.Prefix)

	localFiles, err := listLocalMarkdown(localRoot)
	if err != nil {
		return err
	}
	remoteFiles, err := listRemoteS3(ctx, client, bucket, prefix)
	if err != nil {
		return fmt.Errorf("list s3: %w", err)
	}

	for rel := range mergeRelKeys(localFiles, remoteFiles) {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := syncOneS3(ctx, client, bucket, prefix, localRoot, localFiles, remoteFiles, rel); err != nil {
			return err
		}
	}
	return nil
}

func dialS3(ctx context.Context, cfg S3Config) (*s3.Client, error) {
	region := strings.TrimSpace(cfg.Region)
	if region == "" {
		region = "us-east-1"
	}
	opts := []func(*awscfg.LoadOptions) error{
		awscfg.WithRegion(region),
	}
	if strings.TrimSpace(cfg.AccessKey) != "" && strings.TrimSpace(cfg.SecretKey) != "" {
		opts = append(opts, awscfg.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(strings.TrimSpace(cfg.AccessKey), strings.TrimSpace(cfg.SecretKey), ""),
		))
	}
	ac, err := awscfg.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}
	ep := strings.TrimSpace(cfg.Endpoint)
	if ep != "" {
		return s3.NewFromConfig(ac, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(ep)
			o.UsePathStyle = true
		}), nil
	}
	return s3.NewFromConfig(ac), nil
}

func normalizeS3Prefix(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimPrefix(p, "/")
	if p == "" {
		return ""
	}
	if !strings.HasSuffix(p, "/") {
		p += "/"
	}
	return p
}

func s3Key(prefix, rel string) string {
	rel = strings.TrimPrefix(filepath.ToSlash(rel), "/")
	if prefix == "" {
		return rel
	}
	return prefix + rel
}

func listRemoteS3(ctx context.Context, client *s3.Client, bucket, prefix string) (map[string]remoteFile, error) {
	out := make(map[string]remoteFile)
	paginator := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})
	for paginator.HasMorePages() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, obj := range page.Contents {
			key := aws.ToString(obj.Key)
			if key == "" || strings.HasSuffix(key, "/") {
				continue
			}
			if !strings.EqualFold(filepath.Ext(key), ".md") {
				continue
			}
			rel := strings.TrimPrefix(key, prefix)
			if rel == "" {
				continue
			}
			mod := time.Time{}
			if obj.LastModified != nil {
				mod = *obj.LastModified
			}
			sz := int64(0)
			if obj.Size != nil {
				sz = *obj.Size
			}
			out[rel] = remoteFile{
				path: key,
				snap: fileSnapshot{modTime: mod, size: sz},
			}
		}
	}
	return out, nil
}

func syncOneS3(ctx context.Context, client *s3.Client, bucket, prefix, localRoot string, localFiles map[string]localFile, remoteFiles map[string]remoteFile, rel string) error {
	var lp, rp *fileSnapshot
	if v, ok := localFiles[rel]; ok {
		s := v.snap
		lp = &s
	}
	if v, ok := remoteFiles[rel]; ok {
		s := v.snap
		rp = &s
	}
	switch classifySync(lp, rp) {
	case syncSkip:
		return nil
	case syncPush:
		loc := localFiles[rel]
		return pushS3(ctx, client, bucket, prefix, rel, loc.abs)
	case syncPull:
		rem := remoteFiles[rel]
		return pullS3(ctx, client, bucket, rem.path, rel, localRoot)
	case syncConflict:
		loc := localFiles[rel]
		rem := remoteFiles[rel]
		return resolveS3Conflict(ctx, client, bucket, rem.path, rel, loc.abs, localRoot)
	default:
		return fmt.Errorf("internal: unknown sync action for %s", rel)
	}
}

func pushS3(ctx context.Context, client *s3.Client, bucket, prefix, rel, localAbs string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := os.ReadFile(localAbs)
	if err != nil {
		return err
	}
	key := s3Key(prefix, rel)
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	})
	if err != nil {
		return fmt.Errorf("put s3 %s: %w", key, err)
	}
	return nil
}

func pullS3(ctx context.Context, client *s3.Client, bucket, objectKey, rel, localRoot string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	out, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return err
	}
	defer out.Body.Close()
	data, err := io.ReadAll(out.Body)
	if err != nil {
		return err
	}
	localAbs := filepath.Join(localRoot, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(localAbs), 0o755); err != nil {
		return err
	}
	return atomicWriteFile(localAbs, data)
}

func resolveS3Conflict(ctx context.Context, client *s3.Client, bucket, objectKey, rel, localAbs, localRoot string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	localData, err := os.ReadFile(localAbs)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	conflictAbs := conflictSiblingPath(localAbs)
	if len(localData) > 0 {
		if err := atomicWriteFile(conflictAbs, localData); err != nil {
			return err
		}
	}
	out, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return err
	}
	defer out.Body.Close()
	remoteData, err := io.ReadAll(out.Body)
	if err != nil {
		return err
	}
	dest := filepath.Join(localRoot, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	return atomicWriteFile(dest, remoteData)
}
