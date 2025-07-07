# MultiNIC Agent v2 배포 가이드

## 🚀 빠른 시작

### 1. 원클릭 배포
```bash
# 기본 배포
./scripts/build-and-deploy.sh

# 개발환경 배포
NAMESPACE=multinic-dev IMAGE_TAG=dev ./scripts/build-and-deploy.sh
```

### 2. 수동 배포

#### Docker 이미지 빌드
```bash
docker build -t multinic-agent:latest .
```

#### Helm으로 배포
```bash
# 기본 배포
helm install multinic-agent ./deployments/helm

# 개발환경
helm install multinic-agent ./deployments/helm -f ./deployments/helm/values-dev.yaml

# 프로덕션
helm install multinic-agent ./deployments/helm -f ./deployments/helm/values-prod.yaml
```

## 📋 사전 요구사항

### 시스템 요구사항
- Kubernetes 1.20+
- Helm 3.0+
- Docker 20.0+
- Go 1.21+ (개발용)

### 권한 요구사항
- 클러스터 관리자 권한 (DaemonSet 배포용)
- 각 노드의 네트워크 설정 파일 접근 권한

### 데이터베이스 요구사항
- MultiNIC Controller DB 접근 가능
- `multi_interface` 테이블 존재

## 🔧 설정 옵션

### 데이터베이스 설정
```yaml
database:
  host: "192.168.34.79"    # DB 호스트
  port: "30305"            # DB 포트
  user: "root"             # DB 사용자
  password: "cloud1234"    # DB 패스워드
  name: "multinic"         # DB 이름
```

### 에이전트 설정
```yaml
agent:
  pollInterval: "30s"      # 폴링 주기
  logLevel: "info"         # 로그 레벨
```

### 리소스 설정
```yaml
resources:
  limits:
    cpu: 500m
    memory: 256Mi
  requests:
    cpu: 100m
    memory: 128Mi
```

## 📊 모니터링 및 확인

### 배포 상태 확인
```bash
# DaemonSet 상태
kubectl get daemonset -n default -l app.kubernetes.io/name=multinic-agent

# Pod 상태
kubectl get pods -n default -l app.kubernetes.io/name=multinic-agent

# 로그 확인
kubectl logs -f daemonset/multinic-agent -n default
```

### 헬스체크
```bash
# 포트 포워딩
kubectl port-forward <pod-name> 8080:8080

# 헬스체크 API 호출
curl http://localhost:8080/
```

### 메트릭 확인
헬스체크 엔드포인트에서 다음 정보 제공:
- 처리된 인터페이스 수
- 실패한 설정 수
- 데이터베이스 연결 상태
- 에이전트 가동 시간

## 🐛 문제 해결

### 일반적인 문제들

#### 1. 권한 오류
```
Error: cannot create resource "daemonsets" in API group "apps"
```
**해결**: 클러스터 관리자 권한 필요
```bash
kubectl auth can-i create daemonsets --all-namespaces
```

#### 2. 데이터베이스 연결 실패
```
데이터베이스 연결 실패: dial tcp 192.168.34.79:30305: connect: connection refused
```
**해결**: 
- DB 호스트/포트 확인
- 네트워크 연결성 확인
- 방화벽 설정 확인

#### 3. 인터페이스 설정 실패
```
netplan try 실패: Invalid YAML
```
**해결**:
- MAC 주소 형식 확인
- 네트워크 설정 백업 복원
- 로그에서 상세 오류 확인

### 로그 레벨별 디버깅

#### DEBUG 레벨
```yaml
agent:
  logLevel: "debug"
```
모든 설정 변경사항과 상세한 실행 과정 로그

#### INFO 레벨 (기본)
주요 작업과 상태 변화만 로그

#### ERROR 레벨
오류와 경고만 로그

## 🔄 업그레이드

### Helm을 통한 업그레이드
```bash
# 새 이미지로 업그레이드
helm upgrade multinic-agent ./deployments/helm --set image.tag=v1.1.0

# 설정 변경과 함께 업그레이드
helm upgrade multinic-agent ./deployments/helm -f values-new.yaml
```

### 롤링 업데이트
DaemonSet은 기본적으로 롤링 업데이트를 지원하여 무중단 업그레이드 가능

## 🗑️ 제거

### Helm으로 제거
```bash
helm uninstall multinic-agent -n default
```

### 수동 제거
```bash
kubectl delete daemonset multinic-agent
kubectl delete clusterrole multinic-agent
kubectl delete clusterrolebinding multinic-agent
kubectl delete serviceaccount multinic-agent
kubectl delete secret multinic-agent-db
```

## 📈 성능 최적화

### 리소스 튜닝
- CPU: 인터페이스 수에 따라 조정
- 메모리: 로그 레벨과 폴링 주기에 따라 조정

### 폴링 주기 최적화
- 개발환경: 10-30초
- 프로덕션: 30-60초
- 대규모 클러스터: 60-120초

## 🔐 보안 고려사항

### 필요한 권한
- NET_ADMIN: 네트워크 인터페이스 관리
- SYS_ADMIN: 시스템 설정 파일 접근
- hostNetwork: 호스트 네트워크 직접 접근

### 보안 강화 방안
- 최소 권한 원칙 적용
- 네트워크 정책으로 트래픽 제한
- RBAC으로 클러스터 권한 제한