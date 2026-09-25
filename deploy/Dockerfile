# The Explore backend: one image for serve, worker and migrate.
#
# Versions are pinned so the same commit builds the same image later, as in
# Kite's Dockerfile.

# Must match the toolchain directive in go.mod.
ARG GO_VERSION=1.26.8
ARG ALPINE_VERSION=3.22

# The compiler runs on the build machine's own platform and cross-compiles
# for the target, so an arm64 image builds without emulating arm64.
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
ARG TARGETOS
ARG TARGETARCH

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -buildvcs=false \
    -ldflags "-s -w \
      -X github.com/kite-plus/explore/internal/buildinfo.Version=${VERSION} \
      -X github.com/kite-plus/explore/internal/buildinfo.Commit=${COMMIT} \
      -X github.com/kite-plus/explore/internal/buildinfo.Date=${DATE}" \
    -o /out/explore ./cmd/explore

FROM alpine:${ALPINE_VERSION}

# ca-certificates for fetching feeds over https.
RUN apk add --no-cache ca-certificates && \
    adduser -D -H -u 1718 explore

COPY --from=build /out/explore /usr/local/bin/explore

USER explore
ENV EXPLORE_HTTP_ADDR=0.0.0.0:8080
EXPOSE 8080

ENTRYPOINT ["explore"]
CMD ["serve"]
