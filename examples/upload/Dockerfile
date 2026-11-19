FROM golang:1.25.3-alpine AS build
WORKDIR /src
ENV GOWORK=off
COPY auth/supabase/go.* auth/supabase/
COPY otel/grafana/go.* otel/grafana/
COPY storage/s3/go.* storage/s3/
COPY examples/upload/go.* examples/upload/
WORKDIR /src/examples/upload
RUN go mod download
COPY auth/supabase/*.go /src/auth/supabase/
COPY otel/grafana/*.go /src/otel/grafana/
COPY storage/s3/*.go /src/storage/s3/
COPY examples/upload/*.go ./
COPY examples/upload/api api
COPY examples/upload/internal internal
COPY examples/upload/web web
RUN CGO_ENABLED=0 go build -trimpath -o /upload .

FROM alpine:3.22
RUN apk add --no-cache ca-certificates && mkdir /data && chown 65534:65534 /data
COPY --from=build /upload /usr/local/bin/upload
USER 65534:65534
ENTRYPOINT ["/usr/local/bin/upload"]
