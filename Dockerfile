FROM node:22-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci --no-audit
COPY frontend/ .
RUN npm run build

FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/frontend/dist /app/internal/ui/dist
RUN CGO_ENABLED=0 go build -o /phoebus ./cmd/phoebus

FROM alpine:3.21
RUN apk add --no-cache ca-certificates git
COPY --from=builder /phoebus /usr/local/bin/phoebus
EXPOSE 8080
CMD ["phoebus"]
