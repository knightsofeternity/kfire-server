# ---- build ----
FROM golang:1.23-alpine AS build
WORKDIR /src

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/kfire-server ./cmd/kfire-server

# ---- runtime (~20 MB) ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/kfire-server /kfire-server
COPY migrations /migrations
EXPOSE 8080
USER nonroot
ENTRYPOINT ["/kfire-server"]
