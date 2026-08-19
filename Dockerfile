# Dockerfile for ssh-mcp.
#
# Multi-stage, multi-arch build producing a minimal distroless image.
# The Go binary is statically linked (CGO_ENABLED=0) and stripped (-s -w),
# then copied onto gcr.io/distroless/static-debian12 which provides CA
# certificates (needed for HTTPS proxies) without a shell or package
# manager. Final image is typically 15-18 MB.
#
# Build args:
#   VERSION  - injected into the binary via -ldflags (defaults to "dev").
#
# Buildx sets TARGETOS and TARGETARCH automatically for multi-platform
# builds (linux/amd64, linux/arm64).

ARG VERSION=dev

# --- build stage ---
FROM golang:1.26-alpine AS builder

ARG VERSION
ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

# Cache module downloads by copying manifests first.
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source and build a stripped static binary.
COPY . .

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build \
      -ldflags "-s -w -X main.version=${VERSION}" \
      -o /out/ssh-mcp \
      ./cmd/ssh-mcp

# --- runtime stage ---
FROM gcr.io/distroless/static-debian12:nonroot

# Copy the stripped static binary into a standard PATH location.
COPY --from=builder /out/ssh-mcp /usr/local/bin/ssh-mcp

# ssh-mcp reads/writes local files for SFTP upload/download, so expose a
# working directory that host directories can be mounted onto.
WORKDIR /work

ENTRYPOINT ["ssh-mcp"]
