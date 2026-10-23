FROM golang:1.26.10-alpine@sha256:27f2d3b83cfc5803c8078f8e730af0f7c431bf067afaec8f60a36fb763187cba AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG GIT_REF=unknown
ARG BUILD_TAG=container
ARG BUILD_DATE

RUN CGO_ENABLED=0 go build \
    -ldflags "-s -w \
      -X 'github.com/ivy/kb/internal/cli.version=${VERSION}' \
      -X 'github.com/ivy/kb/internal/cli.gitRef=${GIT_REF}' \
      -X 'github.com/ivy/kb/internal/cli.buildTag=${BUILD_TAG}' \
      -X 'github.com/ivy/kb/internal/cli.buildDate=${BUILD_DATE}'" \
    -o /kb ./cmd/kb

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /kb /kb

ENTRYPOINT ["/kb"]
CMD ["serve", "--config", "/etc/kb/kb.yaml"]
