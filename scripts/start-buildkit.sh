#!/bin/bash

# 색상 정의
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${GREEN}BuildKit 데몬 시작 스크립트${NC}"

# 1. buildkitd 확인
echo -e "\n${YELLOW}1. buildkitd 확인${NC}"
if ! command -v buildkitd &> /dev/null; then
    echo -e "${RED}✗ buildkitd가 설치되어 있지 않습니다${NC}"
    echo "buildctl만 있고 buildkitd가 없는 경우 buildkit을 다시 설치해야 합니다"
    echo "./setup-buildkit.sh를 실행해주세요"
    exit 1
fi

echo -e "${GREEN}✓ buildkitd 발견${NC}"

# 2. 기존 프로세스 확인
echo -e "\n${YELLOW}2. 기존 buildkitd 프로세스 확인${NC}"
if pgrep -f buildkitd > /dev/null; then
    echo -e "${BLUE}buildkitd가 이미 실행 중입니다${NC}"
    echo "프로세스 목록:"
    ps aux | grep buildkitd | grep -v grep
    exit 0
fi

# 3. containerd 확인
echo -e "\n${YELLOW}3. containerd 상태 확인${NC}"
if ! systemctl is-active --quiet containerd; then
    echo -e "${YELLOW}containerd가 실행되지 않고 있습니다. 시작합니다...${NC}"
    sudo systemctl start containerd
    sleep 2
fi

if systemctl is-active --quiet containerd; then
    echo -e "${GREEN}✓ containerd 실행 중${NC}"
else
    echo -e "${RED}✗ containerd 시작 실패${NC}"
    exit 1
fi

# 4. buildkitd 수동 시작
echo -e "\n${YELLOW}4. buildkitd 데몬 시작${NC}"

# k8s.io namespace로 buildkitd 시작
echo -e "${BLUE}buildkitd를 k8s.io namespace로 시작합니다...${NC}"
sudo buildkitd --containerd-worker=true --containerd-worker-namespace=k8s.io &

# 프로세스 시작 대기
sleep 3

# 5. 연결 테스트
echo -e "\n${YELLOW}5. buildkitd 연결 테스트${NC}"
for i in {1..10}; do
    if buildctl --addr unix:///run/buildkit/buildkitd.sock debug workers &>/dev/null; then
        echo -e "${GREEN}✓ buildkitd 연결 성공${NC}"
        break
    elif [ $i -eq 10 ]; then
        echo -e "${RED}✗ buildkitd 연결 실패${NC}"
        echo "수동으로 다음 명령어를 시도해보세요:"
        echo "sudo buildkitd --containerd-worker=true --containerd-worker-namespace=k8s.io"
        exit 1
    else
        echo "연결 시도 $i/10..."
        sleep 2
    fi
done

# 6. 프로세스 확인
echo -e "\n${YELLOW}6. buildkitd 프로세스 확인${NC}"
ps aux | grep buildkitd | grep -v grep

echo -e "\n${GREEN}🎉 buildkitd 시작 완료!${NC}"
echo -e "\n${BLUE}📋 확인 명령어:${NC}"
echo -e "• 연결 테스트: ${YELLOW}buildctl debug workers${NC}"
echo -e "• 프로세스 확인: ${YELLOW}ps aux | grep buildkitd${NC}"
echo -e "• nerdctl 테스트: ${YELLOW}nerdctl --namespace=k8s.io build --help${NC}"

echo -e "\n${BLUE}🚀 이제 배포 스크립트를 실행할 수 있습니다:${NC}"
echo -e "${YELLOW}./nerdctl-deploy.sh${NC}"

echo -e "\n${YELLOW}⚠️  주의: buildkitd는 백그라운드에서 실행됩니다${NC}"
echo -e "시스템 재부팅 시 다시 시작해야 합니다"
echo -e "자동 시작을 원한다면: ${YELLOW}sudo systemctl enable buildkitd${NC}"