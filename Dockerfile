# syntax=docker/dockerfile:1

# One image for production: the API serves the web app and, with
# RUN_CLICK_WORKER=true, also runs the click worker. See
# backend/docs/deployment.md.

FROM node:24-alpine AS web
WORKDIR /web
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.27.4-alpine AS go
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker

# Distroless: no shell or package manager, runs as a non-root user, and ships
# the CA certificates needed for TLS to Postgres, Redis, RabbitMQ and Google.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=go /out/api /app/api
COPY --from=go /out/worker /app/worker
COPY --from=web /web/dist /app/web
ENV HTTP_ADDR=:8080 \
    FRONTEND_DIST_DIR=/app/web
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/api"]
