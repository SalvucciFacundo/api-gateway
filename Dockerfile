# Build the frontend separately so web/embed.go includes the production assets.
FROM node:22-alpine AS frontend-build

WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.26.5-bookworm AS go-build

WORKDIR /src
ARG TARGETOS
ARG TARGETARCH
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend-build /src/web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/gateway ./cmd/gateway

# Alpine supplies a small non-root runtime and wget for the Compose healthcheck.
FROM alpine:3.22

RUN addgroup -S -g 65532 gateway \
    && adduser -S -D -H -u 65532 -G gateway gateway
COPY --from=go-build /out/gateway /gateway

USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/gateway"]
