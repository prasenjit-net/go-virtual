# Dockerfile — multi-arch CI/release build
#
# Requires Docker Buildx. Used by CI and release workflows to build
# linux/amd64 + linux/arm64 images and push to Docker Hub.
# BUILDPLATFORM and TARGETPLATFORM are injected by buildx automatically.
#
# For local single-arch builds see Dockerfile.dev.

# ── Stage 1: build UI (always runs natively on the builder, not under QEMU) ──
# Running on $BUILDPLATFORM ensures npm runs at native speed on the builder
# host regardless of the target arch, avoiding slow QEMU-emulated arm64 builds.
ARG BUILDPLATFORM
ARG TARGETPLATFORM
FROM --platform=$BUILDPLATFORM node:20-alpine AS ui-builder
WORKDIR /app/ui
COPY ui/package.json ui/package-lock.json ./
RUN npm ci
COPY ui/ ./
RUN npm run build

# ── Stage 2: build Go binary (cross-compiled natively, no QEMU needed) ───────
# Build runs on $BUILDPLATFORM and cross-compiles for $TARGETPLATFORM via
# GOOS/GOARCH, so no emulation is required even for arm64 targets.
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS go-builder
WORKDIR /app

ARG TARGETPLATFORM
ARG BUILDPLATFORM
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=ui-builder /app/ui/dist ./ui/dist

# Derive GOOS/GOARCH from $TARGETPLATFORM (e.g. "linux/arm64" → GOOS=linux GOARCH=arm64)
RUN --mount=type=cache,target=/root/.cache/go-build \
    export GOOS=$(echo "$TARGETPLATFORM" | cut -d/ -f1) && \
    export GOARCH=$(echo "$TARGETPLATFORM" | cut -d/ -f2) && \
    CGO_ENABLED=0 go build \
      -ldflags="-s -w \
        -X github.com/prasenjit/go-virtual/internal/version.Version=${VERSION} \
        -X github.com/prasenjit/go-virtual/internal/version.Commit=${COMMIT} \
        -X github.com/prasenjit/go-virtual/internal/version.BuildDate=${BUILD_DATE}" \
      -o /go-virtual ./cmd/server

# ── Stage 3: minimal runtime image ───────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=go-builder /go-virtual /go-virtual

EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=5s --start-period=30s --retries=3 \
  CMD ["/go-virtual", "healthcheck"]
ENTRYPOINT ["/go-virtual", "serve"]
