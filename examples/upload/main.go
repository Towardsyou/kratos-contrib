package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/towardsyou/kratos-contrib/auth/supabase"
	"github.com/towardsyou/kratos-contrib/examples/upload/internal/upload"
	"github.com/towardsyou/kratos-contrib/otel/grafana"
	"github.com/towardsyou/kratos-contrib/storage/s3"
)

func main() {
	if err := run(); err != nil {
		log.Error(err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	demoLogin, err := boolEnv("UPLOAD_DEMO_LOGIN", false)
	if err != nil {
		return err
	}
	auth := supabase.JWTConfig{
		Algorithm: env("UPLOAD_JWT_ALGORITHM", "HS256"),
		Issuer:    os.Getenv("UPLOAD_JWT_ISSUER"),
		Audience:  env("UPLOAD_JWT_AUDIENCE", "authenticated"),
		Secret:    os.Getenv("UPLOAD_JWT_SECRET"),
		JWKSURL:   os.Getenv("UPLOAD_JWKS_URL"),
	}
	if demoLogin && auth.Algorithm != "HS256" {
		return fmt.Errorf("demo login requires HS256")
	}
	verify, err := supabase.NewJWTVerifier(auth)
	if err != nil {
		return err
	}
	pathStyle, err := boolEnv("UPLOAD_S3_PATH_STYLE", true)
	if err != nil {
		return err
	}
	objects, err := s3.New(s3.Config{
		Endpoint:       env("UPLOAD_S3_ENDPOINT", "http://127.0.0.1:9000"),
		PublicEndpoint: os.Getenv("UPLOAD_S3_PUBLIC_ENDPOINT"),
		Region:         env("UPLOAD_S3_REGION", "us-east-1"),
		AccessKeyID:    os.Getenv("UPLOAD_S3_ACCESS_KEY_ID"),
		SecretKey:      os.Getenv("UPLOAD_S3_SECRET_KEY"),
		SessionToken:   os.Getenv("UPLOAD_S3_SESSION_TOKEN"),
		UsePathStyle:   pathStyle,
	})
	if err != nil {
		return err
	}
	store, err := upload.OpenStore(ctx, env("UPLOAD_DB_PATH", "uploads.db"))
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	insecure, err := boolEnv("UPLOAD_OTLP_LOG_INSECURE", false)
	if err != nil {
		return err
	}
	logs, shutdown, err := grafana.InitLoggerProvider(ctx, "upload-example", "dev", grafana.LoggerConfig{
		Endpoint:   os.Getenv("UPLOAD_OTLP_LOG_ENDPOINT"),
		Insecure:   insecure,
		InstanceID: os.Getenv("UPLOAD_OTLP_LOG_INSTANCE_ID"),
		APIKey:     os.Getenv("UPLOAD_OTLP_LOG_API_KEY"),
	})
	if err != nil {
		return err
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdown(ctx); err != nil {
			log.Error(err)
		}
	}()
	logger := grafana.NewLogger(log.NewStdLogger(os.Stdout), logs)

	service := upload.NewService(store, objects, env("UPLOAD_S3_BUCKET", "uploads"))
	server, err := newHTTPServer(env("UPLOAD_HTTP_ADDR", "127.0.0.1:8080"), service, verify, demoLogin, auth)
	if err != nil {
		return err
	}
	return kratos.New(kratos.Name("upload-example"), kratos.Server(server), kratos.Logger(logger)).Run()
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func boolEnv(name string, fallback bool) (bool, error) {
	if value := os.Getenv(name); value != "" {
		result, err := strconv.ParseBool(value)
		if err != nil {
			return false, fmt.Errorf("%s must be a boolean: %w", name, err)
		}
		return result, nil
	}
	return fallback, nil
}
