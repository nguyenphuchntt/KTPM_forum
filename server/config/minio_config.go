package config

import (
	"fmt"
	"os"
)

// Defaults used when the matching MINIO_* variable is unset. The quarantine
// bucket stays private and only ever holds objects whose content has not been
// validated yet; the media bucket is public-read so <img src> can point straight
// at it.
const (
	DefaultQuarantineBucket = "forum-quarantine"
	DefaultMediaBucket      = "forum-media"

	// DefaultRegion is the region every request is signed for. A single MinIO
	// deployment has no regions, so the value only has to be consistent between
	// the two clients — but it must be non-empty, see MinIOConfig.Region.
	DefaultRegion = "us-east-1"
)

// MinIOConfig describes the object storage the upload pipeline talks to.
type MinIOConfig struct {
	// Endpoint is the address the server itself dials, e.g. "minio:9000"
	// inside docker compose or "localhost:9000" for a local run.
	Endpoint string
	// PublicEndpoint is the address the browser dials, e.g. "localhost:9000".
	// It defaults to Endpoint, but the two differ under docker compose, and
	// they must not be conflated: presigned URLs are signed over the Host
	// header, so a URL signed for the in-cluster host is rejected by MinIO
	// when the browser rewrites it to the published one.
	PublicEndpoint   string
	AccessKey        string
	SecretKey        string
	UseSSL           bool
	QuarantineBucket string
	MediaBucket      string
	// Region is what requests are signed for. It has to be set explicitly:
	// minio-go only looks up a bucket's region over the network when its client
	// has none, and that lookup cannot work for the presign client, which is
	// deliberately pointed at a host only the browser can reach.
	Region string
}

// Enabled reports whether object storage is configured at all. With no endpoint
// the app boots with uploads switched off, the way the Azure integration used
// to degrade when its connection string was absent.
func (c *MinIOConfig) Enabled() bool { return c.Endpoint != "" }

// ObjectURL builds the browser-facing URL of an object in the public bucket.
func (c *MinIOConfig) ObjectURL(bucket, objectKey string) string {
	scheme := "http"
	if c.UseSSL {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s/%s/%s", scheme, c.PublicEndpoint, bucket, objectKey)
}

// LoadMinIOConfigFromEnv reads the MINIO_* variables. Missing credentials are
// left empty rather than defaulted, so a half-configured environment fails
// loudly at EnsureBucketsWithRetry instead of silently falling back to a guess.
func LoadMinIOConfigFromEnv() *MinIOConfig {
	cfg := &MinIOConfig{
		Endpoint:         os.Getenv("MINIO_ENDPOINT"),
		PublicEndpoint:   os.Getenv("MINIO_PUBLIC_ENDPOINT"),
		AccessKey:        os.Getenv("MINIO_ACCESS_KEY"),
		SecretKey:        os.Getenv("MINIO_SECRET_KEY"),
		UseSSL:           getEnvBool("MINIO_USE_SSL", false),
		QuarantineBucket: getEnvString("MINIO_QUARANTINE_BUCKET", DefaultQuarantineBucket),
		MediaBucket:      getEnvString("MINIO_MEDIA_BUCKET", DefaultMediaBucket),
		Region:           getEnvString("MINIO_REGION", DefaultRegion),
	}

	if cfg.PublicEndpoint == "" {
		cfg.PublicEndpoint = cfg.Endpoint
	}
	return cfg
}

func getEnvString(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
