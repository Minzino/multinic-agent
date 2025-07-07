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

echo -e "${GREEN}MultiNIC Agent 배포 전용 스크립트${NC}"
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

if ! command -v docker &> /dev/null; then
    echo -e "${RED}✗ Docker가 설치되어 있지 않습니다${NC}"
    exit 1
fi

echo -e "${GREEN}✓ 필수 명령어 확인 완료${NC}"

# 2. Docker 이미지 빌드 (Go 없이)
echo -e "\n${YELLOW}2. Docker 이미지 빌드${NC}"
cd "$(dirname "$0")/.."

docker build -t ${IMAGE_NAME}:${IMAGE_TAG} .

if [ $? -ne 0 ]; then
    echo -e "${RED}✗ Docker 빌드 실패${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Docker 이미지 빌드 성공${NC}"

# 3. 모든 노드에 이미지 배포 (Kind 클러스터가 아닌 경우)
echo -e "\n${YELLOW}3. 클러스터 타입 확인${NC}"
CLUSTER_TYPE=$(kubectl config current-context)
if [[ $CLUSTER_TYPE == *"kind"* ]]; then
    echo -e "${BLUE}Kind 클러스터 감지: 이미지 로드 중...${NC}"
    kind load docker-image ${IMAGE_NAME}:${IMAGE_TAG}
else
    echo -e "${BLUE}일반 클러스터: 모든 노드에 이미지 복사 필요${NC}"
    echo -e "${YELLOW}다음 중 하나를 선택하세요:${NC}"
    echo "1. 컨테이너 레지스트리 사용 (권장)"
    echo "2. 모든 노드에 수동으로 이미지 복사"
    echo "3. 로컬 이미지로 계속 진행 (노드에 이미지가 있는 경우)"
    
    read -p "선택 (1-3): " choice
    case $choice in
        1)
            echo -e "${YELLOW}컨테이너 레지스트리에 푸시하세요:${NC}"
            echo "docker tag ${IMAGE_NAME}:${IMAGE_TAG} your-registry/${IMAGE_NAME}:${IMAGE_TAG}"
            echo "docker push your-registry/${IMAGE_NAME}:${IMAGE_TAG}"
            echo "그 후 values.yaml에서 image.repository를 수정하세요"
            exit 0
            ;;
        2)
            echo -e "${YELLOW}모든 워커 노드에서 다음 명령어를 실행하세요:${NC}"
            echo "docker save ${IMAGE_NAME}:${IMAGE_TAG} | ssh user@node 'docker load'"
            echo "완료 후 다시 스크립트를 실행하세요 (선택지 3)"
            exit 0
            ;;
        3)
            echo -e "${BLUE}로컬 이미지로 계속 진행합니다${NC}"
            ;;
    esac
fi

# 4. Helm 차트 검증
echo -e "\n${YELLOW}4. Helm 차트 검증${NC}"
helm lint ./deployments/helm

if [ $? -ne 0 ]; then
    echo -e "${RED}✗ Helm 차트 검증 실패${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Helm 차트 검증 성공${NC}"

# 5. 네임스페이스 생성 (필요한 경우)
echo -e "\n${YELLOW}5. 네임스페이스 확인${NC}"
kubectl get namespace $NAMESPACE 2>/dev/null || kubectl create namespace $NAMESPACE
echo -e "${GREEN}✓ 네임스페이스 준비 완료${NC}"

# 6. 배포 확인
echo -e "\n${YELLOW}6. 기존 배포 확인${NC}"
if helm list -n $NAMESPACE | grep -q $RELEASE_NAME; then
    echo -e "${BLUE}기존 릴리즈 발견. 업그레이드를 수행합니다.${NC}"
    DEPLOY_CMD="upgrade"
else
    echo -e "${BLUE}새로운 릴리즈를 설치합니다.${NC}"
    DEPLOY_CMD="install"
fi

# 7. 배포 실행
echo -e "\n${YELLOW}7. Helm 배포 실행${NC}"
if [ "$DEPLOY_CMD" = "upgrade" ]; then
    helm upgrade $RELEASE_NAME ./deployments/helm \
        --namespace $NAMESPACE \
        --set image.tag=$IMAGE_TAG \
        --set image.pullPolicy=Never \
        --wait --timeout=5m
else
    helm install $RELEASE_NAME ./deployments/helm \
        --namespace $NAMESPACE \
        --set image.tag=$IMAGE_TAG \
        --set image.pullPolicy=Never \
        --wait --timeout=5m
fi

if [ $? -ne 0 ]; then
    echo -e "${RED}✗ 배포 실패${NC}"
    echo -e "${YELLOW}배포 상태 확인:${NC}"
    kubectl get pods -n $NAMESPACE -l app.kubernetes.io/name=multinic-agent
    kubectl describe pods -n $NAMESPACE -l app.kubernetes.io/name=multinic-agent
    exit 1
fi
echo -e "${GREEN}✓ 배포 성공${NC}"

# 8. 배포 상태 확인
echo -e "\n${YELLOW}8. 배포 상태 확인${NC}"
echo -e "\n${BLUE}DaemonSet 상태:${NC}"
kubectl get daemonset -n $NAMESPACE -l app.kubernetes.io/name=multinic-agent

echo -e "\n${BLUE}Pod 상태:${NC}"
kubectl get pods -n $NAMESPACE -l app.kubernetes.io/name=multinic-agent

echo -e "\n${BLUE}첫 번째 Pod 로그 (최근 20줄):${NC}"
POD=$(kubectl get pods -n $NAMESPACE -l app.kubernetes.io/name=multinic-agent -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ ! -z "$POD" ]; then
    kubectl logs $POD -n $NAMESPACE --tail=20
else
    echo "실행 중인 Pod가 없습니다."
fi

echo -e "\n${GREEN}배포 완료!${NC}"
echo -e "\n${BLUE}유용한 명령어:${NC}"
echo -e "• 로그 확인: ${YELLOW}kubectl logs -f daemonset/$RELEASE_NAME -n $NAMESPACE${NC}"
echo -e "• 상태 확인: ${YELLOW}kubectl get pods -n $NAMESPACE -l app.kubernetes.io/name=multinic-agent${NC}"
echo -e "• 헬스체크: ${YELLOW}kubectl port-forward <pod-name> 8080:8080 -n $NAMESPACE${NC}"
echo -e "• 삭제: ${YELLOW}helm uninstall $RELEASE_NAME -n $NAMESPACE${NC}"