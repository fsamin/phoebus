FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /phoebus ./cmd/phoebus

FROM alpine:3.21
RUN apk add --no-cache ca-certificates git
COPY --from=builder /phoebus /usr/local/bin/phoebus
EXPOSE 8080
CMD ["phoebus"]
