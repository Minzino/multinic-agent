#!/bin/bash

# 색상 정의
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 변수
IMAGE_NAME=${IMAGE_NAME:-"multinic-agent"}
IMAGE_TAG=${IMAGE_TAG:-"latest"}
NAMESPACE=${NAMESPACE:-"default"}
RELEASE_NAME=${RELEASE_NAME:-"multinic-agent"}

echo -e "${GREEN}MultiNIC Agent 간단 배포 스크립트${NC}"
echo -e "이미지: ${BLUE}${IMAGE_NAME}:${IMAGE_TAG}${NC}"
echo -e "네임스페이스: ${BLUE}${NAMESPACE}${NC}"
echo -e "릴리즈명: ${BLUE}${RELEASE_NAME}${NC}"

# 1. 필수 명령어 확인
echo -e "\n${YELLOW}1. 필수 명령어 확인${NC}"

if ! command -v helm &> /dev/null; then
    echo -e "${RED}✗ Helm이 설치되어 있지 않습니다${NC}"
    echo "Helm 설치: https://helm.sh/docs/intro/install/"
    exit 1
fi

if ! command -v kubectl &> /dev/null; then
    echo -e "${RED}✗ kubectl이 설치되어 있지 않습니다${NC}"
    exit 1
fi

echo -e "${GREEN}✓ 필수 명령어 확인 완료${NC}"

# 2. 빌드 도구 확인 및 선택
echo -e "\n${YELLOW}2. 컨테이너 빌드 도구 확인${NC}"

BUILD_TOOL=""
if command -v docker &> /dev/null; then
    BUILD_TOOL="docker"
    echo -e "${BLUE}Docker 발견: docker를 사용합니다${NC}"
elif command -v nerdctl &> /dev/null; then
    # buildkit 확인
    if nerdctl version | grep -q "buildkitd"; then
        BUILD_TOOL="nerdctl"
        echo -e "${BLUE}nerdctl + buildkit 발견: nerdctl을 사용합니다${NC}"
    else
        echo -e "${YELLOW}nerdctl이 있지만 buildkit이 없습니다${NC}"
    fi
elif command -v podman &> /dev/null; then
    BUILD_TOOL="podman"
    echo -e "${BLUE}Podman 발견: podman을 사용합니다${NC}"
fi

if [ -z "$BUILD_TOOL" ]; then
    echo -e "${RED}✗ 컨테이너 빌드 도구를 찾을 수 없습니다${NC}"
    echo -e "${YELLOW}다음 중 하나를 설치해주세요:${NC}"
    echo "1. Docker: https://docs.docker.com/engine/install/"
    echo "2. nerdctl + buildkit: https://github.com/containerd/nerdctl#install"
    echo "3. Podman: https://podman.io/getting-started/installation"
    echo ""
    echo -e "${BLUE}또는 다른 환경에서 이미지를 빌드해서 가져오세요:${NC}"
    echo "docker build -t ${IMAGE_NAME}:${IMAGE_TAG} ."
    echo "docker save ${IMAGE_NAME}:${IMAGE_TAG} | gzip > multinic-agent.tar.gz"
    echo "# 이 서버로 파일 복사 후:"
    if command -v nerdctl &> /dev/null; then
        echo "nerdctl --namespace=k8s.io load < multinic-agent.tar.gz"
    else
        echo "docker load < multinic-agent.tar.gz"
    fi
    exit 1
fi

# 3. 이미지 빌드
echo -e "\n${YELLOW}3. ${BUILD_TOOL}로 이미지 빌드${NC}"
cd "$(dirname "$0")/.."

case $BUILD_TOOL in
    "docker")
        docker build -t ${IMAGE_NAME}:${IMAGE_TAG} .
        ;;
    "nerdctl")
        CONTAINERD_NAMESPACE=${CONTAINERD_NAMESPACE:-"k8s.io"}
        nerdctl --namespace=${CONTAINERD_NAMESPACE} build -t ${IMAGE_NAME}:${IMAGE_TAG} .
        ;;
    "podman")
        podman build -t ${IMAGE_NAME}:${IMAGE_TAG} .
        ;;
esac

if [ $? -ne 0 ]; then
    echo -e "${RED}✗ 이미지 빌드 실패${NC}"
    exit 1
fi
echo -e "${GREEN}✓ 이미지 빌드 성공${NC}"

# 4. 빌드된 이미지 확인
echo -e "\n${YELLOW}4. 빌드된 이미지 확인${NC}"
case $BUILD_TOOL in
    "docker")
        docker images | grep ${IMAGE_NAME}
        ;;
    "nerdctl")
        nerdctl --namespace=${CONTAINERD_NAMESPACE} images | grep ${IMAGE_NAME}
        ;;
    "podman")
        podman images | grep ${IMAGE_NAME}
        ;;
esac
echo -e "${GREEN}✓ 이미지 확인 완료${NC}"

# 5. Helm 차트 검증
echo -e "\n${YELLOW}5. Helm 차트 검증${NC}"
helm lint ./deployments/helm

if [ $? -ne 0 ]; then
    echo -e "${RED}✗ Helm 차트 검증 실패${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Helm 차트 검증 성공${NC}"

# 6. 네임스페이스 생성 (필요한 경우)
echo -e "\n${YELLOW}6. 네임스페이스 확인${NC}"
kubectl get namespace $NAMESPACE 2>/dev/null || kubectl create namespace $NAMESPACE
echo -e "${GREEN}✓ 네임스페이스 준비 완료${NC}"

# 7. 기존 배포 확인
echo -e "\n${YELLOW}7. 기존 배포 확인${NC}"
if helm list -n $NAMESPACE | grep -q $RELEASE_NAME; then
    echo -e "${BLUE}기존 릴리즈 발견. 업그레이드를 수행합니다.${NC}"
    DEPLOY_CMD="upgrade"
else
    echo -e "${BLUE}새로운 릴리즈를 설치합니다.${NC}"
    DEPLOY_CMD="install"
fi

# 8. Helm 배포 실행
echo -e "\n${YELLOW}8. Helm 배포 실행${NC}"
if [ "$DEPLOY_CMD" = "upgrade" ]; then
    helm upgrade $RELEASE_NAME ./deployments/helm \
        --namespace $NAMESPACE \
        --set image.repository=$IMAGE_NAME \
        --set image.tag=$IMAGE_TAG \
        --set image.pullPolicy=Never \
        --wait --timeout=5m
else
    helm install $RELEASE_NAME ./deployments/helm \
        --namespace $NAMESPACE \
        --set image.repository=$IMAGE_NAME \
        --set image.tag=$IMAGE_TAG \
        --set image.pullPolicy=Never \
        --wait --timeout=5m
fi

if [ $? -ne 0 ]; then
    echo -e "${RED}✗ 배포 실패${NC}"
    echo -e "\n${YELLOW}배포 상태 확인:${NC}"
    kubectl get pods -n $NAMESPACE -l app.kubernetes.io/name=multinic-agent
    echo -e "\n${YELLOW}Pod 상세 정보:${NC}"
    kubectl describe pods -n $NAMESPACE -l app.kubernetes.io/name=multinic-agent | tail -20
    exit 1
fi
echo -e "${GREEN}✓ 배포 성공${NC}"

# 9. 배포 상태 확인
echo -e "\n${YELLOW}9. 배포 상태 확인${NC}"

echo -e "\n${BLUE}DaemonSet 상태:${NC}"
kubectl get daemonset -n $NAMESPACE -l app.kubernetes.io/name=multinic-agent

echo -e "\n${BLUE}Pod 상태:${NC}"
kubectl get pods -n $NAMESPACE -l app.kubernetes.io/name=multinic-agent -o wide

# 10. 애플리케이션 로그 확인
echo -e "\n${YELLOW}10. 애플리케이션 로그 확인${NC}"
POD=$(kubectl get pods -n $NAMESPACE -l app.kubernetes.io/name=multinic-agent -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ ! -z "$POD" ]; then
    echo -e "${BLUE}Pod: $POD 로그 (최근 20줄):${NC}"
    kubectl logs $POD -n $NAMESPACE --tail=20
else
    echo -e "${YELLOW}실행 중인 Pod가 없습니다.${NC}"
fi

echo -e "\n${GREEN}🎉 MultiNIC Agent 배포 완료!${NC}"
echo -e "\n${BLUE}📋 사용된 빌드 도구: ${BUILD_TOOL}${NC}"
echo -e "\n${BLUE}📋 유용한 명령어:${NC}"
echo -e "• 실시간 로그: ${YELLOW}kubectl logs -f daemonset/$RELEASE_NAME -n $NAMESPACE${NC}"
echo -e "• Pod 상태: ${YELLOW}kubectl get pods -n $NAMESPACE -l app.kubernetes.io/name=multinic-agent${NC}"
echo -e "• 헬스체크: ${YELLOW}kubectl port-forward <pod-name> 8080:8080 -n $NAMESPACE${NC}"
echo -e "• 삭제: ${YELLOW}helm uninstall $RELEASE_NAME -n $NAMESPACE${NC}"

echo -e "\n${BLUE}🔍 문제 해결:${NC}"
case $BUILD_TOOL in
    "docker")
        echo -e "• 이미지 재빌드: ${YELLOW}docker build -t ${IMAGE_NAME}:${IMAGE_TAG} .${NC}"
        echo -e "• 이미지 확인: ${YELLOW}docker images | grep ${IMAGE_NAME}${NC}"
        ;;
    "nerdctl")
        echo -e "• 이미지 재빌드: ${YELLOW}nerdctl --namespace=${CONTAINERD_NAMESPACE} build -t ${IMAGE_NAME}:${IMAGE_TAG} .${NC}"
        echo -e "• 이미지 확인: ${YELLOW}nerdctl --namespace=${CONTAINERD_NAMESPACE} images${NC}"
        ;;
    "podman")
        echo -e "• 이미지 재빌드: ${YELLOW}podman build -t ${IMAGE_NAME}:${IMAGE_TAG} .${NC}"
        echo -e "• 이미지 확인: ${YELLOW}podman images | grep ${IMAGE_NAME}${NC}"
        ;;
esac
echo -e "• Pod 재시작: ${YELLOW}kubectl rollout restart daemonset/$RELEASE_NAME -n $NAMESPACE${NC}"