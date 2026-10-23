# Build context: this module root (sdk/go). Example:
#   docker build -f example/Dockerfile -t <registry>/pamone-example/go:0.2.1 .
FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /out/app ./example

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 app
COPY --from=build /out/app /usr/local/bin/app
USER app
# No sink directory and no sidecar: the app exports OTLP/HTTP itself.
ENV OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/app"]