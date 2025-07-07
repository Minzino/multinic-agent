#!/bin/bash

# 색상 정의
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 변수
NAMESPACE="multinic-test"
RELEASE_NAME="multinic-agent-test"
DB_HOST=${DB_HOST:-"192.168.34.79"}
DB_PORT=${DB_PORT:-"30305"}

echo -e "${GREEN}MultiNIC Agent 배포 테스트 시작${NC}"

# 1. 네임스페이스 생성
echo -e "\n${YELLOW}1. 네임스페이스 생성${NC}"
kubectl create namespace $NAMESPACE 2>/dev/null || echo "네임스페이스가 이미 존재합니다"

# 2. 데이터베이스 연결 테스트
echo -e "\n${YELLOW}2. 데이터베이스 연결 테스트${NC}"
kubectl run db-test --rm -it --image=mysql:8.0 --namespace=$NAMESPACE --restart=Never -- \
    mysql -h$DB_HOST -P$DB_PORT -uroot -pcloud1234 -e "SELECT 1" 2>/dev/null

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ 데이터베이스 연결 성공${NC}"
else
    echo -e "${RED}✗ 데이터베이스 연결 실패${NC}"
    exit 1
fi

# 3. Docker 이미지 빌드
echo -e "\n${YELLOW}3. Docker 이미지 빌드${NC}"
make docker-build VERSION=test

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Docker 이미지 빌드 성공${NC}"
else
    echo -e "${RED}✗ Docker 이미지 빌드 실패${NC}"
    exit 1
fi

# 4. Helm 차트 검증
echo -e "\n${YELLOW}4. Helm 차트 검증${NC}"
helm lint ./deployments/helm

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Helm 차트 검증 성공${NC}"
else
    echo -e "${RED}✗ Helm 차트 검증 실패${NC}"
    exit 1
fi

# 5. Dry-run 테스트
echo -e "\n${YELLOW}5. Helm 설치 시뮬레이션 (dry-run)${NC}"
helm install $RELEASE_NAME ./deployments/helm \
    --namespace $NAMESPACE \
    --set image.tag=test \
    --set database.host=$DB_HOST \
    --set database.port=$DB_PORT \
    --dry-run --debug

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Helm dry-run 성공${NC}"
else
    echo -e "${RED}✗ Helm dry-run 실패${NC}"
    exit 1
fi

# 6. 실제 배포
echo -e "\n${YELLOW}6. 실제 배포 진행${NC}"
read -p "실제로 배포하시겠습니까? (y/n) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    helm install $RELEASE_NAME ./deployments/helm \
        --namespace $NAMESPACE \
        --set image.tag=test \
        --set database.host=$DB_HOST \
        --set database.port=$DB_PORT \
        --wait --timeout=2m

    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ 배포 성공${NC}"
        
        # 7. 배포 상태 확인
        echo -e "\n${YELLOW}7. 배포 상태 확인${NC}"
        kubectl get daemonset -n $NAMESPACE
        kubectl get pods -n $NAMESPACE
        
        # 8. 로그 확인
        echo -e "\n${YELLOW}8. 에이전트 로그 확인 (첫 번째 Pod)${NC}"
        POD=$(kubectl get pods -n $NAMESPACE -l app.kubernetes.io/name=multinic-agent -o jsonpath='{.items[0].metadata.name}')
        kubectl logs $POD -n $NAMESPACE --tail=20
        
    else
        echo -e "${RED}✗ 배포 실패${NC}"
        exit 1
    fi
fi

# 9. 정리
echo -e "\n${YELLOW}9. 테스트 환경 정리${NC}"
read -p "테스트 환경을 정리하시겠습니까? (y/n) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    helm uninstall $RELEASE_NAME --namespace $NAMESPACE
    kubectl delete namespace $NAMESPACE
    echo -e "${GREEN}✓ 정리 완료${NC}"
fi

echo -e "\n${GREEN}배포 테스트 완료${NC}"