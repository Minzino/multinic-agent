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

# 5. containerd-rootless 확인 (옵션)
echo -e "\n${YELLOW}5. containerd 설정 확인${NC}"
if systemctl is-active --quiet containerd; then
    echo -e "${GREEN}✓ containerd 서비스 실행 중${NC}"
else
    echo -e "${YELLOW}⚠ containerd 서비스가 실행되지 않고 있습니다${NC}"
    echo "sudo systemctl start containerd"
fi

# 6. buildkitd 서비스 설정 (선택사항)
echo -e "\n${YELLOW}6. buildkitd 서비스 설정${NC}"
cat > /tmp/buildkitd.service << 'EOF'
[Unit]
Description=BuildKit daemon
After=containerd.service

[Service]
Type=notify
ExecStart=/usr/local/bin/buildkitd --containerd-worker=true --containerd-worker-namespace=k8s.io
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

echo -e "${BLUE}buildkitd 서비스 파일이 준비되었습니다${NC}"
echo -e "${YELLOW}다음 명령어로 서비스를 설치하고 시작할 수 있습니다:${NC}"
echo "sudo cp /tmp/buildkitd.service /etc/systemd/system/"
echo "sudo systemctl daemon-reload"
echo "sudo systemctl enable buildkitd"
echo "sudo systemctl start buildkitd"

# 정리
rm -rf $TMP_DIR

echo -e "\n${GREEN}🎉 BuildKit 설치 완료!${NC}"
echo -e "\n${BLUE}📋 확인 명령어:${NC}"
echo -e "• BuildKit 버전: ${YELLOW}buildctl --version${NC}"
echo -e "• buildkitd 실행: ${YELLOW}sudo systemctl status buildkitd${NC}"
echo -e "• nerdctl 빌드 테스트: ${YELLOW}nerdctl --namespace=k8s.io build --help${NC}"

echo -e "\n${BLUE}🔧 사용법:${NC}"
echo -e "이제 ./simple-deploy.sh 또는 ./nerdctl-deploy.sh를 사용할 수 있습니다"