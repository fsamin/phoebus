FROM node:24-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci --no-audit
COPY frontend/ .
RUN NODE_OPTIONS="--max-old-space-size=4096" npm run build

FROM golang:1.26-alpine AS builder
ARG VERSION=dev
ARG COMMIT=none
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/frontend/dist /app/internal/ui/dist
RUN CGO_ENABLED=0 go build -ldflags "-s -w \
    -X github.com/fsamin/phoebus/internal/version.Version=${VERSION} \
    -X github.com/fsamin/phoebus/internal/version.Commit=${COMMIT} \
    -X github.com/fsamin/phoebus/internal/version.Date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o /phoebus ./cmd/phoebus

FROM alpine:3.21
RUN apk add --no-cache ca-certificates git
COPY --from=builder /phoebus /usr/local/bin/phoebus
EXPOSE 8080
CMD ["phoebus"]
