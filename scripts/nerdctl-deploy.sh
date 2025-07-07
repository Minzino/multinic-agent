#!/bin/bash

set -e

# 색상 정의
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${GREEN}MultiNIC Agent nerdctl 배포 스크립트${NC}"

# 변수 설정
IMAGE_NAME=${IMAGE_NAME:-"multinic-agent"}
IMAGE_TAG=${IMAGE_TAG:-"latest"}
NAMESPACE=${NAMESPACE:-"default"}
RELEASE_NAME=${RELEASE_NAME:-"multinic-agent"}

echo -e "이미지: ${BLUE}${IMAGE_NAME}:${IMAGE_TAG}${NC}"
echo -e "네임스페이스: ${BLUE}${NAMESPACE}${NC}"
echo -e "릴리즈명: ${BLUE}${RELEASE_NAME}${NC}"

# 1. 필수 명령어 확인
echo -e "\n${BLUE}1. 필수 명령어 확인${NC}"
commands=("nerdctl" "helm" "kubectl")
for cmd in "${commands[@]}"; do
    if ! command -v $cmd &> /dev/null; then
        echo -e "${RED}✗ $cmd가 설치되어 있지 않습니다${NC}"
        exit 1
    fi
done
echo -e "${GREEN}✓ 필수 명령어 확인 완료${NC}"

# 2. nerdctl로 Docker 이미지 빌드
echo -e "\n${BLUE}2. nerdctl로 Docker 이미지 빌드${NC}"
cd "$(dirname "$0")/.."

echo -e "${YELLOW}containerd namespace: k8s.io${NC}"
if ! nerdctl --namespace=k8s.io build --no-cache -t ${IMAGE_NAME}:${IMAGE_TAG} .; then
    echo -e "${RED}✗ nerdctl 빌드 실패${NC}"
    echo -e "\n${YELLOW}buildkitd가 실행되지 않았을 수 있습니다. 다음을 시도해보세요:${NC}"
    echo "1. ./scripts/start-buildkit.sh (임시 시작)"
    echo "2. ./scripts/setup-buildkit.sh (완전 설치 및 자동 시작)"
    exit 1
fi

echo -e "${GREEN}✓ nerdctl로 이미지 빌드 완료${NC}"

# 3. Helm 차트 검증
echo -e "\n${BLUE}3. Helm 차트 검증${NC}"
if ! helm lint ./deployments/helm; then
    echo -e "${RED}✗ Helm 차트 검증 실패${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Helm 차트 검증 완료${NC}"

# 4. 네임스페이스 생성
echo -e "\n${BLUE}4. 네임스페이스 생성${NC}"
if ! kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -; then
    echo -e "${RED}✗ 네임스페이스 생성 실패${NC}"
    exit 1
fi
echo -e "${GREEN}✓ 네임스페이스 생성/확인 완료${NC}"

# 5. 기존 배포 확인 및 배포
echo -e "\n${BLUE}5. MultiNIC Agent 배포${NC}"
if helm list -n $NAMESPACE | grep -q $RELEASE_NAME; then
    echo -e "${YELLOW}기존 릴리즈 발견. 업그레이드를 수행합니다...${NC}"
    if ! helm upgrade $RELEASE_NAME ./deployments/helm \
        --namespace $NAMESPACE \
        --set image.repository=$IMAGE_NAME \
        --set image.tag=$IMAGE_TAG \
        --set image.pullPolicy=Never \
        --wait --timeout=5m; then
        echo -e "${RED}✗ Helm 업그레이드 실패${NC}"
        exit 1
    fi
else
    echo -e "${YELLOW}새로운 릴리즈를 설치합니다...${NC}"
    if ! helm install $RELEASE_NAME ./deployments/helm \
        --namespace $NAMESPACE \
        --set image.repository=$IMAGE_NAME \
        --set image.tag=$IMAGE_TAG \
        --set image.pullPolicy=Never \
        --wait --timeout=5m; then
        echo -e "${RED}✗ Helm 설치 실패${NC}"
        exit 1
    fi
fi

echo -e "${GREEN}✓ MultiNIC Agent 배포 완료${NC}"

# 6. DaemonSet Pod 상태 확인
echo -e "\n${BLUE}6. DaemonSet Pod 상태 확인${NC}"
echo -e "${YELLOW}DaemonSet Pod들이 Ready 상태가 될 때까지 대기중...${NC}"
if ! kubectl wait --for=condition=Ready pod -l app.kubernetes.io/name=multinic-agent -n $NAMESPACE --timeout=300s; then
    echo -e "${YELLOW}⚠️  일부 Pod의 Ready 상태 확인 타임아웃. 수동으로 확인해주세요.${NC}"
else
    echo -e "${GREEN}✓ 모든 Agent Pod가 성공적으로 실행중입니다${NC}"
fi

# 7. 전체 상태 확인
echo -e "\n${BLUE}7. 전체 시스템 상태 확인${NC}"
echo "=================================================="
echo "📋 MultiNIC Agent DaemonSet 상태:"
kubectl get daemonset -n $NAMESPACE -l app.kubernetes.io/name=multinic-agent

echo ""
echo "📋 MultiNIC Agent Pod 상태 (모든 노드):"
kubectl get pods -n $NAMESPACE -l app.kubernetes.io/name=multinic-agent -o wide

echo ""
echo "📋 노드별 Pod 분포:"
kubectl get pods -n $NAMESPACE -l app.kubernetes.io/name=multinic-agent -o jsonpath='{range .items[*]}{.spec.nodeName}{"\t"}{.metadata.name}{"\t"}{.status.phase}{"\n"}{end}' | column -t

echo ""
echo "📋 서비스 및 기타 리소스:"
kubectl get svc,secret,serviceaccount,clusterrole,clusterrolebinding -n $NAMESPACE -l app.kubernetes.io/name=multinic-agent
echo "=================================================="

# 8. 헬스체크
echo -e "\n${BLUE}8. 헬스체크${NC}"
POD=$(kubectl get pods -n $NAMESPACE -l app.kubernetes.io/name=multinic-agent -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ ! -z "$POD" ]; then
    echo -e "${YELLOW}첫 번째 Pod ($POD) 로그 확인 (최근 10줄):${NC}"
    kubectl logs $POD -n $NAMESPACE --tail=10
    
    echo ""
    echo -e "${YELLOW}헬스체크 엔드포인트 테스트 (포트 포워딩):${NC}"
    echo -e "${BLUE}다음 명령어로 헬스체크를 확인할 수 있습니다:${NC}"
    echo "kubectl port-forward $POD 8080:8080 -n $NAMESPACE"
    echo "curl http://localhost:8080/"
else
    echo -e "${YELLOW}실행 중인 Pod가 없습니다.${NC}"
fi

echo -e "\n${GREEN}🎉 MultiNIC Agent 배포가 완료되었습니다!${NC}"

echo -e "\n${YELLOW}📖 사용법:${NC}"
echo -e "  • Agent 로그 확인: ${BLUE}kubectl logs -f daemonset/$RELEASE_NAME -n $NAMESPACE${NC}"
echo -e "  • Pod 상태 확인: ${BLUE}kubectl get pods -n $NAMESPACE -l app.kubernetes.io/name=multinic-agent${NC}"
echo -e "  • 특정 노드 Pod 로그: ${BLUE}kubectl logs <pod-name> -n $NAMESPACE${NC}"
echo -e "  • 헬스체크: ${BLUE}kubectl port-forward <pod-name> 8080:8080 -n $NAMESPACE${NC}"

echo -e "\n${BLUE}🔧 다음 단계:${NC}"
echo -e "  1. 데이터베이스 연결 테스트"
echo -e "  2. multi_interface 테이블에 테스트 데이터 추가"
echo -e "  3. 네트워크 인터페이스 생성 모니터링"
echo -e "  4. Agent 로그에서 처리 상황 확인"

echo -e "\n${BLUE}🗑️  삭제 방법:${NC}"
echo -e "  ${YELLOW}helm uninstall $RELEASE_NAME -n $NAMESPACE${NC}"

echo -e "\n${YELLOW}⚠️  참고사항:${NC}"
echo -e "  • Agent는 DaemonSet으로 모든 노드에서 실행됩니다"
echo -e "  • 네트워크 설정 변경을 위해 privileged 권한이 필요합니다"
echo -e "  • 실패한 설정은 자동으로 롤백됩니다"