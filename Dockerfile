# ---- frontend: build the admin SPA ----
FROM node:22-alpine AS web
WORKDIR /web
RUN corepack enable
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

# ---- backend: build the static Go binary (embeds the SPA) ----
FROM golang:1.23-alpine AS build
WORKDIR /src

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
# Bring in the freshly built SPA so //go:embed picks it up.
COPY --from=web /web/build ./web/build
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/kfire-server ./cmd/kfire-server

# ---- runtime ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/kfire-server /kfire-server
COPY migrations /migrations
EXPOSE 8080
USER nonroot
ENTRYPOINT ["/kfire-server"]
