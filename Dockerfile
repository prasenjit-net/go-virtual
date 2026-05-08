# ── Stage 1: build UI (always runs natively on the builder, not under QEMU) ──
# $BUILDPLATFORM = the host machine arch (e.g. linux/amd64).
# The UI output is pure static files — architecture-independent — so there is
# no reason to emulate arm64 here. This eliminates the QEMU npm timeout.
FROM --platform=$BUILDPLATFORM node:20-alpine AS ui-builder
WORKDIR /app/ui
COPY ui/package.json ui/package-lock.json ./
RUN npm ci
COPY ui/ ./
RUN npm run build

# ── Stage 2: build Go binary (cross-compiled, no QEMU needed) ─────────────────
# $BUILDPLATFORM = builder host arch; $TARGETPLATFORM = linux/amd64 or linux/arm64.
# We build on the host and cross-compile via GOARCH so this stage also runs
# natively — no emulation required.
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
