#!/bin/bash

set -e

echo "🚀 MultiNIC Agent v2 배포를 시작합니다..."

# 색상 정의
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 변수 설정
IMAGE_NAME=${IMAGE_NAME:-"multinic-agent"}
IMAGE_TAG=${IMAGE_TAG:-"latest"}
NAMESPACE=${NAMESPACE:-"default"}
RELEASE_NAME=${RELEASE_NAME:-"multinic-agent"}

# 워커 노드 목록 (환경에 맞게 수정)
WORKER_NODES=(viola2-biz-worker01 viola2-biz-worker02 viola2-biz-worker03)

echo "이미지: ${BLUE}${IMAGE_NAME}:${IMAGE_TAG}${NC}"
echo "네임스페이스: ${BLUE}${NAMESPACE}${NC}"
echo "릴리즈명: ${BLUE}${RELEASE_NAME}${NC}"

# 1. nerdctl을 사용한 이미지 빌드 (기존 방식과 동일)
echo -e "\n${BLUE}📦 1단계: nerdctl로 이미지 빌드${NC}"
cd "$(dirname "$0")/.."

echo "nerdctl을 사용하여 이미지를 빌드합니다..."
nerdctl --namespace=k8s.io build --no-cache -t ${IMAGE_NAME}:${IMAGE_TAG} .

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✅ nerdctl로 이미지 빌드 완료${NC}"
else
    echo -e "${RED}❌ nerdctl 이미지 빌드 실패${NC}"
    echo -e "\n${YELLOW}buildkit 문제일 수 있습니다. 다음을 시도해보세요:${NC}"
    echo "1. sudo buildkitd --containerd-worker=true --containerd-worker-namespace=k8s.io &"
    echo "2. 또는 다른 환경에서 이미지를 빌드해서 가져오기"
    exit 1
fi

# 1.5. 이미지를 모든 워커 노드에 배포
echo -e "\n${BLUE}🚚 1.5단계: 이미지 배포${NC}"
echo "이미지를 모든 워커 노드에 배포..."
nerdctl --namespace=k8s.io save ${IMAGE_NAME}:${IMAGE_TAG} -o ${IMAGE_NAME}-${IMAGE_TAG}.tar

for node in "${WORKER_NODES[@]}"; do
    echo "📦 $node 노드에 이미지 전송 중..."
    scp ${IMAGE_NAME}-${IMAGE_TAG}.tar $node:/tmp/
    
    echo "🔧 $node 노드에 이미지 로드 중..."
    ssh $node "sudo nerdctl --namespace=k8s.io load -i /tmp/${IMAGE_NAME}-${IMAGE_TAG}.tar && rm /tmp/${IMAGE_NAME}-${IMAGE_TAG}.tar"
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✅ $node 노드 완료${NC}"
    else
        echo -e "${YELLOW}⚠️  $node 노드에서 오류 발생 (계속 진행)${NC}"
    fi
done

echo "🗑️ 로컬 tar 파일 정리..."
rm -f ${IMAGE_NAME}-${IMAGE_TAG}.tar
echo -e "${GREEN}✅ 모든 노드에 이미지 배포 완료${NC}"

# 2. 필수 도구 확인
echo -e "\n${BLUE}🔧 2단계: 필수 도구 확인${NC}"
if ! command -v helm &> /dev/null; then
    echo -e "${RED}❌ Helm이 설치되어 있지 않습니다${NC}"
    exit 1
fi

if ! command -v kubectl &> /dev/null; then
    echo -e "${RED}❌ kubectl이 설치되어 있지 않습니다${NC}"
    exit 1
fi
echo -e "${GREEN}✅ 필수 도구 확인 완료${NC}"

# 3. Helm 차트 검증
echo -e "\n${BLUE}📋 3단계: Helm 차트 검증${NC}"
helm lint ./deployments/helm
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✅ Helm 차트 검증 완료${NC}"
else
    echo -e "${RED}❌ Helm 차트 검증 실패${NC}"
    exit 1
fi

# 4. 네임스페이스 생성
echo -e "\n${BLUE}📁 4단계: 네임스페이스 생성${NC}"
kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✅ 네임스페이스 생성/확인 완료${NC}"
else
    echo -e "${RED}❌ 네임스페이스 생성 실패${NC}"
    exit 1
fi

# 5. 기존 배포 확인 및 배포
echo -e "\n${BLUE}🚀 5단계: MultiNIC Agent 배포${NC}"
if helm list -n $NAMESPACE | grep -q $RELEASE_NAME; then
    echo "기존 릴리즈 발견. 업그레이드를 수행합니다..."
    helm upgrade $RELEASE_NAME ./deployments/helm \
        --namespace $NAMESPACE \
        --set image.repository=$IMAGE_NAME \
        --set image.tag=$IMAGE_TAG \
        --set image.pullPolicy=Never \
        --wait --timeout=5m
else
    echo "새로운 릴리즈를 설치합니다..."
    helm install $RELEASE_NAME ./deployments/helm \
        --namespace $NAMESPACE \
        --set image.repository=$IMAGE_NAME \
        --set image.tag=$IMAGE_TAG \
        --set image.pullPolicy=Never \
        --wait --timeout=5m
fi

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✅ MultiNIC Agent 배포 완료${NC}"
else
    echo -e "${RED}❌ MultiNIC Agent 배포 실패${NC}"
    exit 1
fi

# 6. DaemonSet Pod 상태 확인
echo -e "\n${BLUE}🔍 6단계: DaemonSet Pod 상태 확인${NC}"
echo "DaemonSet Pod들이 Ready 상태가 될 때까지 대기중..."
kubectl wait --for=condition=Ready pod -l app.kubernetes.io/name=multinic-agent -n $NAMESPACE --timeout=300s
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✅ 모든 Agent Pod가 성공적으로 실행중입니다${NC}"
else
    echo -e "${YELLOW}⚠️  일부 Pod의 Ready 상태 확인 타임아웃. 수동으로 확인해주세요.${NC}"
fi

# 7. 전체 상태 확인
echo -e "\n${BLUE}📊 7단계: 전체 시스템 상태 확인${NC}"
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
echo -e "\n${BLUE}🩺 8단계: 헬스체크${NC}"
POD=$(kubectl get pods -n $NAMESPACE -l app.kubernetes.io/name=multinic-agent -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ ! -z "$POD" ]; then
    echo "첫 번째 Pod ($POD) 로그 확인 (최근 10줄):"
    kubectl logs $POD -n $NAMESPACE --tail=10
    
    echo ""
    echo "헬스체크 엔드포인트 테스트 (포트 포워딩):"
    echo "다음 명령어로 헬스체크를 확인할 수 있습니다:"
    echo "kubectl port-forward $POD 8080:8080 -n $NAMESPACE"
    echo "curl http://localhost:8080/"
else
    echo -e "${YELLOW}실행 중인 Pod가 없습니다.${NC}"
fi

echo -e "\n${GREEN}🎉 MultiNIC Agent v2 배포가 완료되었습니다!${NC}"

echo -e "\n${YELLOW}📖 사용법:${NC}"
echo "  • Agent 로그 확인: kubectl logs -f daemonset/$RELEASE_NAME -n $NAMESPACE"
echo "  • Pod 상태 확인: kubectl get pods -n $NAMESPACE -l app.kubernetes.io/name=multinic-agent"
echo "  • 특정 노드 Pod 로그: kubectl logs <pod-name> -n $NAMESPACE"
echo "  • 헬스체크: kubectl port-forward <pod-name> 8080:8080 -n $NAMESPACE"

echo -e "\n${BLUE}🔧 다음 단계:${NC}"
echo "  1. 데이터베이스 연결 테스트"
echo "  2. multi_interface 테이블에 테스트 데이터 추가"
echo "  3. 네트워크 인터페이스 생성 모니터링"
echo "  4. Agent 로그에서 처리 상황 확인"

echo -e "\n${BLUE}🗑️  삭제 방법:${NC}"
echo "  helm uninstall $RELEASE_NAME -n $NAMESPACE"

echo -e "\n${YELLOW}⚠️  참고사항:${NC}"
echo "  • Agent는 DaemonSet으로 모든 노드에서 실행됩니다"
echo "  • 네트워크 설정 변경을 위해 privileged 권한이 필요합니다"
echo "  • 실패한 설정은 자동으로 롤백됩니다"