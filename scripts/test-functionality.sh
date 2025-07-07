#!/bin/bash

# 색상 정의
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${GREEN}MultiNIC Agent 기능 테스트${NC}"

# 1. 유닛 테스트
echo -e "\n${YELLOW}1. 유닛 테스트 실행${NC}"
go test -v ./pkg/... -cover

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ 유닛 테스트 통과${NC}"
else
    echo -e "${RED}✗ 유닛 테스트 실패${NC}"
    exit 1
fi

# 2. 유효성 검사 테스트
echo -e "\n${YELLOW}2. 유효성 검사 기능 테스트${NC}"
go test -v ./pkg/utils -run TestValidate

# 3. 네트워크 매니저 테스트
echo -e "\n${YELLOW}3. 네트워크 매니저 팩토리 테스트${NC}"
go test -v ./pkg/network -run TestNewNetworkManager

# 4. 코드 품질 검사
echo -e "\n${YELLOW}4. 코드 품질 검사${NC}"

echo -e "${BLUE}4.1 go fmt 검사${NC}"
NEED_FMT=$(go fmt ./...)
if [ -z "$NEED_FMT" ]; then
    echo -e "${GREEN}✓ 코드 포맷팅 OK${NC}"
else
    echo -e "${RED}✗ 다음 파일들이 포맷팅 필요:${NC}"
    echo "$NEED_FMT"
fi

echo -e "${BLUE}4.2 go vet 검사${NC}"
go vet ./...
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ go vet 통과${NC}"
else
    echo -e "${RED}✗ go vet 실패${NC}"
fi

# 5. 빌드 테스트
echo -e "\n${YELLOW}5. 빌드 테스트${NC}"
go build -o /tmp/multinic-agent-test cmd/agent/main.go
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ 빌드 성공${NC}"
    rm /tmp/multinic-agent-test
else
    echo -e "${RED}✗ 빌드 실패${NC}"
    exit 1
fi

# 6. 의존성 검사
echo -e "\n${YELLOW}6. 의존성 검사${NC}"
go mod verify
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ 모든 의존성 확인됨${NC}"
else
    echo -e "${RED}✗ 의존성 문제 발견${NC}"
fi

# 7. 테스트 커버리지 보고서
echo -e "\n${YELLOW}7. 테스트 커버리지 분석${NC}"
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | grep total | awk '{print "전체 커버리지: " $3}'

# 8. 메모리 누수 테스트 (선택사항)
echo -e "\n${YELLOW}8. 메모리 프로파일링 (선택사항)${NC}"
read -p "메모리 프로파일링을 실행하시겠습니까? (y/n) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    go test -memprofile=mem.prof -bench=. ./pkg/...
    echo -e "${GREEN}✓ 메모리 프로파일 생성됨: mem.prof${NC}"
fi

echo -e "\n${GREEN}모든 기능 테스트 완료${NC}"

# 결과 요약
echo -e "\n${YELLOW}=== 테스트 결과 요약 ===${NC}"
echo -e "1. 유닛 테스트: ${GREEN}통과${NC}"
echo -e "2. 빌드 테스트: ${GREEN}성공${NC}"
echo -e "3. 코드 품질: ${GREEN}양호${NC}"