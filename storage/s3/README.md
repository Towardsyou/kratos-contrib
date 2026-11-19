# storage/s3

```go
import "github.com/towardsyou/kratos-contrib/storage/s3"
```

Create a client with `s3.New` and inject it into your application. The client implements `PresignPut`, `PresignGet`, and `HeadObject`.

```go
client, err := s3.New(s3.Config{
    Endpoint:       "http://minio:9000",
    PublicEndpoint: "http://127.0.0.1:9000",
    Region:         "us-east-1",
    AccessKeyID:    accessKey,
    SecretKey:      secretKey,
    UsePathStyle:   true,
})
// Handle err before using the client.
request, err := client.PresignPut(ctx, "uploads", "photo.jpg",
    s3.PutOptions{ContentType: "image/jpeg", IfAbsent: true}, 5*time.Minute)
```

Use the returned URL, method and headers on the HTTP request. The URL supplies the signed Host header. `ExpiresAt` is derived from the actual signature. When `IfAbsent` is set, the signed PUT carries `If-None-Match: *` for create-only uploads. The lifetime must be between one second and seven days, in whole seconds.

`HeadObject` uses `Endpoint`. The presigner uses `PublicEndpoint`, which defaults to `Endpoint`. Changing the resulting URL's hostname after signing invalidates the signature. For Cloudflare R2 use the account's S3 API endpoint, region `auto` and R2 S3 credentials. Path-style addressing can be enabled or disabled as needed. Configure the bucket's CORS policy when a browser uses signed URLs from a different origin. [R2 presigned URL documentation](https://developers.cloudflare.com/r2/api/s3/presigned-urls/)

See the [upload example](../../examples/upload/README.md) for a private MinIO bucket and browser uploads with synchronous confirmation.
