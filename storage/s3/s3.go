// Package s3 presigns object requests and reads metadata from S3-compatible storage.
package s3

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/credentials"
	s3sdk "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// Config configures the S3 API endpoints and credentials.
type Config struct {
	Endpoint       string // Used for server-side requests.
	PublicEndpoint string // Used for presigned requests. Defaults to Endpoint.
	Region         string
	AccessKeyID    string
	SecretKey      string
	SessionToken   string // Optional for temporary credentials.
	UsePathStyle   bool
}

// Client uses separate endpoints for metadata requests and signing.
type Client struct {
	internal *s3sdk.Client
	signer   *s3sdk.PresignClient
}

// New constructs a client without making a network request.
func New(cfg Config) (*Client, error) {
	if cfg.PublicEndpoint == "" {
		cfg.PublicEndpoint = cfg.Endpoint
	}
	for _, endpoint := range []string{cfg.Endpoint, cfg.PublicEndpoint} {
		u, err := url.Parse(endpoint)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" ||
			u.User != nil || u.ForceQuery || u.RawQuery != "" || u.Fragment != "" {
			return nil, errors.New("S3 endpoints must be HTTP(S) URLs without credentials, query or fragment")
		}
	}
	if cfg.Region == "" || cfg.AccessKeyID == "" || cfg.SecretKey == "" {
		return nil, errors.New("S3 region, access key ID and secret key are required")
	}
	awsCfg := aws.Config{
		Region:                     cfg.Region,
		Credentials:                credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretKey, cfg.SessionToken),
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
		ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,
	}
	newClient := func(endpoint string) *s3sdk.Client {
		return s3sdk.NewFromConfig(awsCfg, func(o *s3sdk.Options) {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = cfg.UsePathStyle
		})
	}
	internal := newClient(cfg.Endpoint)
	public := internal
	if cfg.PublicEndpoint != cfg.Endpoint {
		public = newClient(cfg.PublicEndpoint)
	}
	return &Client{internal: internal, signer: s3sdk.NewPresignClient(public)}, nil
}

// SignedRequest describes an HTTP request. The URL supplies the signed Host header.
type SignedRequest struct {
	URL       string
	Method    string
	Headers   http.Header
	ExpiresAt time.Time
}

// PutOptions configures the headers for an upload.
type PutOptions struct {
	ContentType string
	IfAbsent    bool // Sign If-None-Match: * so the request cannot overwrite an existing object.
}

// PresignPut signs a PUT request for a single object.
func (c *Client) PresignPut(ctx context.Context, bucket, key string, opts PutOptions, ttl time.Duration) (SignedRequest, error) {
	if err := validateObject(bucket, key, ttl); err != nil {
		return SignedRequest{}, err
	}
	in := &s3sdk.PutObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)}
	if opts.ContentType != "" {
		in.ContentType = aws.String(opts.ContentType)
	}
	if opts.IfAbsent {
		in.IfNoneMatch = aws.String("*")
	}
	req, err := c.signer.PresignPutObject(ctx, in, func(o *s3sdk.PresignOptions) {
		o.Expires = ttl
		if opts.ContentType != "" {
			o.ClientOptions = append(o.ClientOptions, func(client *s3sdk.Options) {
				client.APIOptions = append(client.APIOptions, func(stack *middleware.Stack) error {
					// Presigning has no request body. Keep the caller's Content-Type
					// even though the SDK sees a zero-length body while signing.
					_, err := stack.Build.Remove("RemoveContentTypeHeader")
					return err
				})
			})
		}
	})
	if err != nil {
		return SignedRequest{}, err
	}
	return signedRequest(req)
}

// PresignGet signs a GET request for a single object.
func (c *Client) PresignGet(ctx context.Context, bucket, key string, ttl time.Duration) (SignedRequest, error) {
	if err := validateObject(bucket, key, ttl); err != nil {
		return SignedRequest{}, err
	}
	req, err := c.signer.PresignGetObject(ctx, &s3sdk.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, s3sdk.WithPresignExpires(ttl))
	if err != nil {
		return SignedRequest{}, err
	}
	return signedRequest(req)
}

func validateObject(bucket, key string, ttl time.Duration) error {
	if bucket == "" || key == "" {
		return errors.New("S3 bucket and object key are required")
	}
	if ttl < time.Second || ttl > 7*24*time.Hour || ttl%time.Second != 0 {
		return errors.New("S3 presign lifetime must be between 1 second and 7 days in whole seconds")
	}
	return nil
}

func signedRequest(req *v4.PresignedHTTPRequest) (SignedRequest, error) {
	u, err := url.Parse(req.URL)
	if err != nil {
		return SignedRequest{}, fmt.Errorf("parse S3 presigned URL: %w", err)
	}
	query := u.Query()
	signedAt, err := time.Parse("20060102T150405Z", query.Get("X-Amz-Date"))
	if err != nil {
		return SignedRequest{}, fmt.Errorf("read S3 signature timestamp: %w", err)
	}
	seconds, err := strconv.ParseInt(query.Get("X-Amz-Expires"), 10, 32)
	if err != nil {
		return SignedRequest{}, fmt.Errorf("read S3 signature lifetime: %w", err)
	}
	headers := req.SignedHeader.Clone()
	headers.Del("Host")
	return SignedRequest{
		URL: req.URL, Method: req.Method, Headers: headers,
		ExpiresAt: signedAt.Add(time.Duration(seconds) * time.Second),
	}, nil
}

// ErrObjectNotFound means the object is missing or has a delete marker.
var ErrObjectNotFound = errors.New("S3 object not found")

// Metadata describes a stored object.
type Metadata struct {
	Size         int64
	ETag         string
	ContentType  string
	LastModified time.Time
}

// HeadObject reads object metadata through the internal endpoint.
func (c *Client) HeadObject(ctx context.Context, bucket, key string) (Metadata, error) {
	if bucket == "" || key == "" {
		return Metadata{}, errors.New("S3 bucket and object key are required")
	}
	out, err := c.internal.HeadObject(ctx, &s3sdk.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var response *smithyhttp.ResponseError
		if errors.As(err, &response) && response.HTTPStatusCode() == http.StatusNotFound {
			return Metadata{}, ErrObjectNotFound
		}
		return Metadata{}, err
	}
	if aws.ToBool(out.DeleteMarker) {
		return Metadata{}, ErrObjectNotFound
	}
	if out.ContentLength == nil || *out.ContentLength < 0 {
		return Metadata{}, errors.New("S3 HEAD response has no valid object size")
	}
	return Metadata{
		Size: *out.ContentLength, ETag: aws.ToString(out.ETag),
		ContentType: aws.ToString(out.ContentType), LastModified: aws.ToTime(out.LastModified),
	}, nil
}
