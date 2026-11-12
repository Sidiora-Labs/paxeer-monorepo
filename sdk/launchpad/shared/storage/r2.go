// Package storage is the shared object-storage layer for the media services.
// It targets Cloudflare R2 (and any S3-compatible endpoint) through
// aws-sdk-go-v2 with path-style addressing. [L9]
//
// The media platform is BUCKET-PRIMARY: services write user-uploaded assets
// (token logos/banners, user pfps/banners) straight to a bucket and read them
// back, while the media/gateway edge fronts reads with CDN + Redis caching. A
// Railway volume is never the source of truth (it pins to a single instance and
// is a single block device — see the 2026-06-26 storage decision).
//
// Each media service owns ITS OWN bucket (a distinct S3_BUCKET value in its
// docker-env) but shares the same R2 account/endpoint, so one Client wraps one
// bucket. Construct one Client per bucket.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// ErrNotFound is returned by Get/GetBytes/Head when the key is absent.
var ErrNotFound = errors.New("storage: object not found")

// ErrObjectTooLarge means a buffered download exceeded its caller's byte budget.
var ErrObjectTooLarge = errors.New("storage: object exceeds buffered read limit")

// ErrInvalidBufferLimit means the budget is negative or cannot accommodate the
// extra byte used to detect an oversized stream.
var ErrInvalidBufferLimit = errors.New("storage: invalid buffered read limit")

// Config configures a Client. All fields are required except Region, which
// defaults to "auto" (R2's region token). Field names mirror the TS S3_* env
// vars (invariant i8) so deploy configs port across unchanged.
type Config struct {
	Endpoint        string // S3 API endpoint, e.g. https://<account>.r2.cloudflarestorage.com
	Region          string // "auto" for R2
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
}

// Client is a thin, bucket-scoped wrapper over an S3-compatible API.
type Client struct {
	s3     *s3.Client
	bucket string
}

// Uploader is the write surface of Client (interface for clean wiring/testing;
// the concrete Client is exercised end-to-end against a real MinIO container in
// tests — no fakes).
type Uploader interface {
	Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error
	Delete(ctx context.Context, key string) error
}

// Reader is the read surface of Client.
type Reader interface {
	Get(ctx context.Context, key string) (*Object, error)
	GetBytes(ctx context.Context, key string, maxBytes int64) ([]byte, string, error)
	Head(ctx context.Context, key string) (*ObjectInfo, error)
}

// Object is a streamed object read. Callers MUST close Body.
type Object struct {
	Body        io.ReadCloser
	ContentType string
	Size        int64 // -1 when Content-Length is unknown
	ETag        string
}

// ObjectInfo is object metadata without the body (a HEAD result).
type ObjectInfo struct {
	ContentType string
	Size        int64
	ETag        string
}

// New builds a bucket-scoped Client. It validates the config and constructs an
// aws-sdk-go-v2 S3 client pinned to the given endpoint with path-style URLs
// (required by R2 and MinIO).
func New(ctx context.Context, cfg Config) (*Client, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	region := cfg.Region
	if region == "" {
		region = "auto"
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("storage: load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		o.UsePathStyle = true
	})
	return &Client{s3: client, bucket: cfg.Bucket}, nil
}

// NewFromS3 wraps an already-constructed *s3.Client against a bucket. It exists
// so tests can point at a MinIO container's client and so callers can share one
// underlying S3 client across multiple buckets.
func NewFromS3(client *s3.Client, bucket string) *Client {
	return &Client{s3: client, bucket: bucket}
}

func (c Config) validate() error {
	var missing []string
	if strings.TrimSpace(c.Endpoint) == "" {
		missing = append(missing, "Endpoint")
	}
	if strings.TrimSpace(c.AccessKeyID) == "" {
		missing = append(missing, "AccessKeyID")
	}
	if strings.TrimSpace(c.SecretAccessKey) == "" {
		missing = append(missing, "SecretAccessKey")
	}
	if strings.TrimSpace(c.Bucket) == "" {
		missing = append(missing, "Bucket")
	}
	if len(missing) > 0 {
		return fmt.Errorf("storage: missing required config: %s", strings.Join(missing, ", "))
	}
	return nil
}

// Bucket returns the bucket this Client is scoped to.
func (c *Client) Bucket() string { return c.bucket }

// Put writes body to key. When size > 0 it is sent as Content-Length, allowing
// a single-shot upload without buffering; a negative size buffers the reader.
// contentType is stored and returned verbatim on Get/Head.
func (c *Client) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	in := &s3.PutObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
		Body:   body,
	}
	if contentType != "" {
		in.ContentType = aws.String(contentType)
	}
	if size >= 0 {
		in.ContentLength = aws.Int64(size)
	}
	if _, err := c.s3.PutObject(ctx, in); err != nil {
		return fmt.Errorf("storage: put %q: %w", key, err)
	}
	return nil
}

// Get streams the object at key. The caller MUST close Object.Body. A missing
// key yields ErrNotFound.
func (c *Client) Get(ctx context.Context, key string) (*Object, error) {
	out, err := c.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("storage: get %q: %w", key, err)
	}
	size := int64(-1)
	if out.ContentLength != nil {
		size = *out.ContentLength
	}
	return &Object{
		Body:        out.Body,
		ContentType: aws.ToString(out.ContentType),
		Size:        size,
		ETag:        strings.Trim(aws.ToString(out.ETag), `"`),
	}, nil
}

// GetBytes buffers an object within an explicit caller-approved byte budget.
// maxBytes must be in [0, math.MaxInt64-1]; zero permits only empty objects.
// Known oversize objects are rejected before reading; all other bodies are read
// through a maxBytes+1 limit, even when metadata understates the size. Failure
// returns no partial content or content type. GetBytes always closes the body.
// Use Get and close its Body for intentionally larger streaming downloads.
func (c *Client) GetBytes(ctx context.Context, key string, maxBytes int64) (content []byte, contentType string, err error) {
	if maxBytes < 0 || maxBytes == math.MaxInt64 {
		return nil, "", ErrInvalidBufferLimit
	}
	obj, err := c.Get(ctx, key)
	if err != nil {
		return nil, "", err
	}
	return readBufferedObject(key, obj, maxBytes)
}

func readBufferedObject(key string, obj *Object, maxBytes int64) ([]byte, string, error) {
	defer func() {
		if cleanupErr := obj.Body.Close(); cleanupErr != nil {
			log.Printf("cleanup obj.Body.Close: %v", cleanupErr)
		}
	}()
	if obj.Size > maxBytes {
		return nil, "", fmt.Errorf("storage: read %q (limit %d bytes): %w", key, maxBytes, ErrObjectTooLarge)
	}
	data, err := io.ReadAll(io.LimitReader(obj.Body, maxBytes+1))
	if int64(len(data)) > maxBytes {
		return nil, "", fmt.Errorf("storage: read %q (limit %d bytes): %w", key, maxBytes, ErrObjectTooLarge)
	}
	if err != nil {
		return nil, "", fmt.Errorf("storage: read %q: %w", key, err)
	}
	return data, obj.ContentType, nil
}

// Head returns object metadata without the body. A missing key yields
// ErrNotFound. Useful for existence checks and conditional serving (ETag/304).
func (c *Client) Head(ctx context.Context, key string) (*ObjectInfo, error) {
	out, err := c.s3.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("storage: head %q: %w", key, err)
	}
	return &ObjectInfo{
		ContentType: aws.ToString(out.ContentType),
		Size:        aws.ToInt64(out.ContentLength),
		ETag:        strings.Trim(aws.ToString(out.ETag), `"`),
	}, nil
}

// Delete removes key. Deleting a non-existent key is a no-op (S3 semantics).
func (c *Client) Delete(ctx context.Context, key string) error {
	if _, err := c.s3.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}); err != nil {
		return fmt.Errorf("storage: delete %q: %w", key, err)
	}
	return nil
}

// isNotFound reports whether err is an S3 "key/bucket absent" error. It handles
// both the typed NoSuchKey/NotFound shapes and the generic smithy APIError code
// (HeadObject returns a bare 404 with code "NotFound" and no typed body).
func isNotFound(err error) bool {
	var nsk *types.NoSuchKey
	if errors.As(err, &nsk) {
		return true
	}
	var nf *types.NotFound
	if errors.As(err, &nf) {
		return true
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NoSuchKey", "NotFound", "404":
			return true
		}
	}
	return false
}
