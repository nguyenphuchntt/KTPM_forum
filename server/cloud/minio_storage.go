package cloud

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"forum/server/config"
	"forum/server/logger"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// missingObjectCodes are the S3 error codes that mean "no such key" rather than
// a real failure.
var missingObjectCodes = map[string]bool{
	"NoSuchKey":    true,
	"NoSuchObject": true,
	"NotFound":     true,
}

// MinIOStorage implements Storage against a MinIO (or any S3-compatible)
// deployment.
//
// It deliberately holds two clients. Pre-signed URLs are signed over the Host
// header, so a URL signed against the in-cluster address (minio:9000) is
// rejected by MinIO when the browser rewrites it to the published one
// (localhost:9000). ops therefore talks to the internal endpoint for every
// server-side call, while presign is built from the endpoint the browser
// actually uses and is only ever asked to sign.
type MinIOStorage struct {
	ops     *minio.Client
	presign *minio.Client
	cfg     *config.MinIOConfig
}

// NewMinIOStorage builds both clients. It performs no I/O — call
// EnsureBucketsWithRetry once at boot to verify the endpoint is reachable and
// the topology is right.
func NewMinIOStorage(cfg *config.MinIOConfig) (*MinIOStorage, error) {
	ops, err := newMinIOClient(cfg.Endpoint, cfg)
	if err != nil {
		return nil, err
	}

	presign := ops
	if cfg.PublicEndpoint != cfg.Endpoint {
		presign, err = newMinIOClient(cfg.PublicEndpoint, cfg)
		if err != nil {
			return nil, err
		}
	}

	return &MinIOStorage{ops: ops, presign: presign, cfg: cfg}, nil
}

func newMinIOClient(endpoint string, cfg *config.MinIOConfig) (*minio.Client, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		// Region must be set. Left empty, minio-go resolves a bucket's region by
		// calling GET /<bucket>/?location= against *this client's own endpoint*
		// before it will sign anything. That call is fine for the ops client but
		// impossible for the presign client, whose endpoint is the host the
		// browser dials — inside docker compose that host resolves to the app
		// container itself, so signing a PUT fails with a connection refused.
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("minio client for %s: %w", endpoint, err)
	}
	return client, nil
}

// EnsureBuckets creates the quarantine and media buckets when missing and makes
// the media bucket public-read. It is idempotent, so it is safe to run on every
// boot.
func (s *MinIOStorage) EnsureBuckets(ctx context.Context) error {
	for _, bucket := range []string{s.cfg.QuarantineBucket, s.cfg.MediaBucket} {
		exists, err := s.ops.BucketExists(ctx, bucket)
		if err != nil {
			return fmt.Errorf("check bucket %s: %w", bucket, err)
		}
		if exists {
			continue
		}
		if err := s.ops.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("create bucket %s: %w", bucket, err)
		}
		logger.Log.Info().Str("bucket", bucket).Msg("created object storage bucket")
	}

	// A failed policy leaves the bucket private, which only breaks image
	// loading — worth failing the boot over, since uploads would look fine.
	if err := s.ops.SetBucketPolicy(ctx, s.cfg.MediaBucket, publicReadPolicy(s.cfg.MediaBucket)); err != nil {
		return fmt.Errorf("set public-read policy on %s: %w", s.cfg.MediaBucket, err)
	}
	return nil
}

// EnsureBucketsWithRetry calls EnsureBuckets until it succeeds or every attempt
// is spent.
//
// It exists because "the container has started" is not "the server is listening":
// AIStor validates its license before it accepts connections, and docker compose
// only orders on container start. Without this, an app that wins the race exits
// fatally on a connection refused that would have resolved a second later.
func (s *MinIOStorage) EnsureBucketsWithRetry(ctx context.Context, attempts int, interval time.Duration) error {
	if attempts < 1 {
		attempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if attempt > 1 {
			select {
			case <-ctx.Done():
				return fmt.Errorf("ensure buckets: %w", ctx.Err())
			case <-time.After(interval):
			}
		}

		if err := s.EnsureBuckets(ctx); err != nil {
			lastErr = err
			logger.Log.Warn().Err(err).Int("attempt", attempt).Int("attempts", attempts).
				Msg("Object storage not ready yet")
			continue
		}
		return nil
	}

	return fmt.Errorf("object storage unreachable after %d attempts: %w", attempts, lastErr)
}

// publicReadPolicy allows anonymous GET on every object of the bucket. MinIO
// keeps buckets private until a policy says otherwise.
func publicReadPolicy(bucket string) string {
	return fmt.Sprintf(`{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {"AWS": ["*"]},
      "Action": ["s3:GetObject"],
      "Resource": ["arn:aws:s3:::%s/*"]
    }
  ]
}`, bucket)
}

func (s *MinIOStorage) PresignPut(ctx context.Context, bucket, objectKey string, expiry time.Duration) (string, error) {
	url, err := s.presign.PresignedPutObject(ctx, bucket, objectKey, expiry)
	if err != nil {
		return "", fmt.Errorf("presign put %s/%s: %w", bucket, objectKey, err)
	}
	return url.String(), nil
}

func (s *MinIOStorage) StatObject(ctx context.Context, bucket, objectKey string) (ObjectInfo, error) {
	info, err := s.ops.StatObject(ctx, bucket, objectKey, minio.StatObjectOptions{})
	if err != nil {
		if isMissingObject(err) {
			return ObjectInfo{}, ErrObjectNotFound
		}
		return ObjectInfo{}, fmt.Errorf("stat %s/%s: %w", bucket, objectKey, err)
	}
	return ObjectInfo{Key: info.Key, Size: info.Size, ContentType: info.ContentType}, nil
}

func (s *MinIOStorage) GetObject(ctx context.Context, bucket, objectKey string) (io.ReadCloser, error) {
	object, err := s.ops.GetObject(ctx, bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		if isMissingObject(err) {
			return nil, ErrObjectNotFound
		}
		return nil, fmt.Errorf("get %s/%s: %w", bucket, objectKey, err)
	}
	return object, nil
}

func (s *MinIOStorage) Copy(ctx context.Context, srcBucket, srcObject, dstBucket, dstObject string) error {
	_, err := s.ops.CopyObject(ctx,
		minio.CopyDestOptions{Bucket: dstBucket, Object: dstObject},
		minio.CopySrcOptions{Bucket: srcBucket, Object: srcObject},
	)
	if err != nil {
		if isMissingObject(err) {
			return ErrObjectNotFound
		}
		return fmt.Errorf("copy %s/%s to %s/%s: %w", srcBucket, srcObject, dstBucket, dstObject, err)
	}
	return nil
}

func (s *MinIOStorage) Remove(ctx context.Context, bucket, objectKey string) error {
	if err := s.ops.RemoveObject(ctx, bucket, objectKey, minio.RemoveObjectOptions{}); err != nil {
		if isMissingObject(err) {
			return nil
		}
		return fmt.Errorf("remove %s/%s: %w", bucket, objectKey, err)
	}
	return nil
}

func (s *MinIOStorage) PublicURL(bucket, objectKey string) string {
	return s.cfg.ObjectURL(bucket, objectKey)
}

func isMissingObject(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrObjectNotFound) {
		return true
	}
	return missingObjectCodes[minio.ToErrorResponse(err).Code]
}
