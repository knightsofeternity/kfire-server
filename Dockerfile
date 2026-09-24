# Both build stages run on the native build platform: the SPA is
# architecture-independent and Go cross-compiles, so a multi-arch build never
# runs node or the Go toolchain under QEMU emulation.

# ---- frontend: build the admin SPA ----
FROM --platform=$BUILDPLATFORM node:22-alpine AS web
WORKDIR /web
ENV COREPACK_ENABLE_DOWNLOAD_PROMPT=0
RUN corepack enable
COPY web/package.json web/pnpm-lock.yaml web/.npmrc ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

# ---- backend: build the static Go binary (embeds the SPA) ----
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS build
ARG TARGETOS=linux
ARG TARGETARCH
WORKDIR /src

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
# Bring in the freshly built SPA so //go:embed picks it up.
COPY --from=web /web/build ./web/build
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" \
    -o /out/kfire-server ./cmd/kfire-server

# ---- runtime ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/kfire-server /kfire-server
COPY migrations /migrations
EXPOSE 8080
USER nonroot
ENTRYPOINT ["/kfire-server"]
