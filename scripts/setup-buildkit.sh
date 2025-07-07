#!/bin/bash

# 색상 정의
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${GREEN}BuildKit 설치 스크립트${NC}"

# 1. 시스템 정보 확인
echo -e "\n${YELLOW}1. 시스템 정보 확인${NC}"
echo "OS: $(uname -s)"
echo "Architecture: $(uname -m)"
echo "Kernel: $(uname -r)"

# 2. BuildKit 설치 확인
echo -e "\n${YELLOW}2. 기존 설치 확인${NC}"
if command -v buildctl &> /dev/null; then
    echo -e "${GREEN}✓ buildctl이 이미 설치되어 있습니다${NC}"
    buildctl --version
    exit 0
fi

# 3. containerd 확인
echo -e "\n${YELLOW}3. containerd 확인${NC}"
if ! command -v containerd &> /dev/null; then
    echo -e "${RED}✗ containerd가 설치되어 있지 않습니다${NC}"
    echo "containerd를 먼저 설치해주세요"
    exit 1
fi
echo -e "${GREEN}✓ containerd 발견${NC}"

# 4. BuildKit 다운로드 및 설치
echo -e "\n${YELLOW}4. BuildKit 설치${NC}"

BUILDKIT_VERSION="v0.12.5"
ARCH=$(uname -m)
case $ARCH in
    x86_64) ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    armv7l) ARCH="armv7" ;;
    *) echo -e "${RED}지원하지 않는 아키텍처: $ARCH${NC}"; exit 1 ;;
esac

DOWNLOAD_URL="https://github.com/moby/buildkit/releases/download/${BUILDKIT_VERSION}/buildkit-${BUILDKIT_VERSION}.linux-${ARCH}.tar.gz"

echo -e "${BLUE}다운로드 URL: $DOWNLOAD_URL${NC}"

# 임시 디렉토리 생성
TMP_DIR=$(mktemp -d)
cd $TMP_DIR

# BuildKit 다운로드
echo -e "${YELLOW}BuildKit 다운로드 중...${NC}"
if ! curl -L -o buildkit.tar.gz "$DOWNLOAD_URL"; then
    echo -e "${RED}✗ 다운로드 실패${NC}"
    rm -rf $TMP_DIR
    exit 1
fi

# 압축 해제 및 설치
echo -e "${YELLOW}압축 해제 및 설치 중...${NC}"
tar -xzf buildkit.tar.gz

# 바이너리 설치
sudo cp bin/* /usr/local/bin/

# 설치 확인
if command -v buildctl &> /dev/null && command -v buildkitd &> /dev/null; then
    echo -e "${GREEN}✓ BuildKit 설치 성공${NC}"
    buildctl --version
else
    echo -e "${RED}✗ BuildKit 설치 실패${NC}"
    rm -rf $TMP_DIR
    exit 1
fi

# 5. containerd 확인 및 시작
echo -e "\n${YELLOW}5. containerd 설정 확인${NC}"
if systemctl is-active --quiet containerd; then
    echo -e "${GREEN}✓ containerd 서비스 실행 중${NC}"
else
    echo -e "${YELLOW}⚠ containerd 서비스가 실행되지 않고 있습니다${NC}"
    echo "containerd를 시작합니다..."
    sudo systemctl start containerd
    sleep 2
    if systemctl is-active --quiet containerd; then
        echo -e "${GREEN}✓ containerd 시작 완료${NC}"
    else
        echo -e "${RED}✗ containerd 시작 실패${NC}"
    fi
fi

# 6. buildkitd 서비스 설정 및 자동 시작
echo -e "\n${YELLOW}6. buildkitd 서비스 설정${NC}"
cat > /tmp/buildkitd.service << 'EOF'
[Unit]
Description=BuildKit daemon
After=containerd.service
Requires=containerd.service

[Service]
Type=notify
ExecStart=/usr/local/bin/buildkitd --containerd-worker=true --containerd-worker-namespace=k8s.io
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

echo -e "${BLUE}buildkitd 서비스 파일을 설치합니다...${NC}"
sudo cp /tmp/buildkitd.service /etc/systemd/system/
sudo systemctl daemon-reload

echo -e "${YELLOW}buildkitd 서비스를 활성화하고 시작합니다...${NC}"
sudo systemctl enable buildkitd
sudo systemctl start buildkitd

# 서비스 상태 확인
sleep 3
if systemctl is-active --quiet buildkitd; then
    echo -e "${GREEN}✓ buildkitd 서비스 시작 완료${NC}"
else
    echo -e "${RED}✗ buildkitd 서비스 시작 실패${NC}"
    echo "로그 확인: sudo journalctl -u buildkitd -f"
fi

# 7. 연결 테스트
echo -e "\n${YELLOW}7. buildkitd 연결 테스트${NC}"
for i in {1..10}; do
    if buildctl debug workers &>/dev/null; then
        echo -e "${GREEN}✓ buildkitd 연결 성공${NC}"
        break
    elif [ $i -eq 10 ]; then
        echo -e "${RED}✗ buildkitd 연결 실패${NC}"
        echo "수동으로 확인해보세요: sudo systemctl status buildkitd"
    else
        echo "연결 시도 $i/10..."
        sleep 2
    fi
done

# 정리
rm -rf $TMP_DIR

echo -e "\n${GREEN}🎉 BuildKit 설치 및 설정 완료!${NC}"
echo -e "\n${BLUE}📋 확인 명령어:${NC}"
echo -e "• BuildKit 버전: ${YELLOW}buildctl --version${NC}"
echo -e "• buildkitd 상태: ${YELLOW}sudo systemctl status buildkitd${NC}"
echo -e "• 연결 테스트: ${YELLOW}buildctl debug workers${NC}"
echo -e "• nerdctl 빌드 테스트: ${YELLOW}nerdctl --namespace=k8s.io build --help${NC}"

echo -e "\n${BLUE}🔧 사용법:${NC}"
echo -e "이제 ./simple-deploy.sh 또는 ./nerdctl-deploy.sh를 사용할 수 있습니다"

echo -e "\n${YELLOW}⚠️ 참고:${NC}"
echo -e "• buildkitd는 systemd 서비스로 등록되어 시스템 부팅 시 자동 시작됩니다"
echo -e "• 서비스 로그 확인: ${YELLOW}sudo journalctl -u buildkitd -f${NC}"
echo -e "• 서비스 중지: ${YELLOW}sudo systemctl stop buildkitd${NC}"
echo -e "• 서비스 비활성화: ${YELLOW}sudo systemctl disable buildkitd${NC}"