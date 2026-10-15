# Build context: this module root (sdk/go). Example:
#   docker build -f example/Dockerfile -t <registry>/pamone-example/go:0.1.0 .
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
ENV PAMONE_SINK_DIR=/var/log/pamone
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/app"]