# 빌드 스테이지
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o multinic-agent cmd/agent/main.go

# 실행 스테이지
FROM alpine:3.18

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /app/multinic-agent .

ENTRYPOINT ["./multinic-agent"]