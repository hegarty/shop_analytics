# Builds all three shop_analytics binaries into one small, non-root image —
# see shop_ingestor's Dockerfile for why one image, selected by command at
# deploy time, rather than three near-identical images.

FROM golang:1.24.13-bookworm AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/scheduler ./cmd/scheduler && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/scheduler /out/worker /out/migrate ./
USER nonroot:nonroot
ENTRYPOINT ["/app/worker"]
