# 빌드 스테이지
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

# 소스 코드 복사 (관련 디렉토리만 명시하여 캐시 효율성 증대)
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN go build -o multinic-agent cmd/agent/main.go

# 실행 스테이지
FROM alpine:3.18

RUN apk add --no-cache ca-certificates networkmanager

WORKDIR /app
COPY --from=builder /app/multinic-agent .

ENTRYPOINT ["./multinic-agent"]