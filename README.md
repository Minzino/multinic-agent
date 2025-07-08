# MultiNIC Agent v0.5.0

MultiNIC 네트워크 설정 관리를 위한 Kubernetes DaemonSet 에이전트

## 개요

이 에이전트는 Kubernetes 클러스터에 조인된 노드들의 네트워크 인터페이스 설정을 자동으로 관리합니다. 
데이터베이스를 모니터링하여 실패한 네트워크 설정을 감지하고 자동으로 재시도합니다.

## 버전 정보

- **버전**: 0.5.0
- **Go 버전**: 1.21+
- **Kubernetes**: 1.20+
- **지원 OS**: Ubuntu 18.04+, SUSE Linux Enterprise 15+

## 주요 기능

- **데이터베이스 기반 설정 관리**: MySQL/MariaDB 연동
- **다중 OS 지원**: Ubuntu (Netplan) 및 SUSE (Wicked)
- **자동 롤백 기능**: 실패 시 설정 파일 제거
- **multinic0 ~ multinic9 인터페이스 관리**: 최대 10개 지원
- **기존 네트워크 보호**: eth0, ens* 등 기존 인터페이스 보호
- **클린 아키텍처**: 도림 주도 설계 및 전체 노드 지원

## 아키텍처

```
┌─────────────────┐
│   Controller    │
│  (DB: MariaDB)  │
└────────┬────────┘
         │
    ┌────▼────┐
    │ Agent   │ (DaemonSet)
    │ - DB 모니터링 (30초 주기)
    │ - 설정 적용 (Netplan/Wicked)
    │ - 자동 롤백
    │ - 헬스체크 (포트 8080)
    └─────────┘
```

## 설치 방법

### 원클릭 배포 (권장)

```bash
# 모든 노드에 이미지 자동 배포 및 Helm 설치
./scripts/deploy.sh

# 환경 변수로 설정 변경
NAMESPACE=multinic-dev IMAGE_TAG=0.5.0 ./scripts/deploy.sh
```

### Helm을 사용한 설치

```bash
# 기본 설치
helm install multinic-agent ./deployments/helm

# 커스텀 값 사용
helm install multinic-agent ./deployments/helm \
  --set database.host=YOUR_DB_HOST \
  --set database.port=YOUR_DB_PORT \
  --set database.password=YOUR_DB_PASSWORD
```

### values.yaml 설정

```yaml
database:
  host: "YOUR_DB_HOST"
  port: "YOUR_DB_PORT"
  user: "YOUR_DB_USER"
  password: "YOUR_DB_PASSWORD"
  name: "YOUR_DB_NAME"

agent:
  pollInterval: "30s"
```

## 동작 방식

1. 에이전트는 주기적으로 데이터베이스의 `multi_interface` 테이블을 확인
2. `netplan_success = 0`이고 `attached_node_name`이 본인인 항목 발견
3. MAC 주소를 기반으로 네트워크 설정 자동 생성 (IP 설정 없이 단순 인터페이스만)
4. 인터페이스 이름을 multinic0~9 형식으로 자동 할당 (최대 10개)
5. OS에 따라 적절한 네트워크 관리자 사용 (Netplan/Wicked)
6. 설정 적용 및 테스트 후 성공 시 `netplan_success = 1`로 업데이트
7. 실패 시 설정 파일 제거 및 롤백

## 데이터베이스 스키마

### multi_interface 테이블
```sql
CREATE TABLE multi_interface (
    id INT PRIMARY KEY AUTO_INCREMENT,
    port_id VARCHAR(36) NOT NULL,
    subnet_id VARCHAR(36) NOT NULL,
    macaddress VARCHAR(17) NOT NULL,
    attached_node_id VARCHAR(36),
    attached_node_name VARCHAR(255),
    cr_namespace VARCHAR(255) NOT NULL,
    cr_name VARCHAR(255) NOT NULL,
    status VARCHAR(50) DEFAULT 'active',
    netplan_success TINYINT(1) DEFAULT 0,
    created_at TIMESTAMP,
    modified_at TIMESTAMP,
    deleted_at TIMESTAMP
);
```

## 개발

### 빌드
```bash
docker build -t multinic-agent:latest .
```

### 로컬 테스트
```bash
# 단위 테스트
go test ./internal/...

# 로컬 실행 (환경 변수 설정 필요)
export DB_HOST=YOUR_DB_HOST
export DB_PASSWORD=YOUR_DB_PASSWORD
go run cmd/agent/main.go
```

## 모니터링 및 로깅

### 로깅 형식
에이전트는 JSON 형식으로 구조화된 로그를 출력합니다:
- 설정 적용 시작/완료
- 오류 발생 시 상세 정보
- 롤백 수행 시 알림

### 헬스체크
- **엔드포인트**: `GET /` (포트 8080)
- **상태**: healthy/degraded/unhealthy
- **정보**: 데이터베이스 연결, 처리된 VM 수, 실패 수

## 문제 해결

### 에이전트가 설정을 적용하지 않음
1. 데이터베이스 연결 확인
2. 노드의 호스트네임과 DB의 attached_node_name 일치 여부 확인
3. 에이전트 로그 확인

### 네트워크 설정 실패
1. 에이전트 로그 확인: `kubectl logs -l app.kubernetes.io/name=multinic-agent`
2. OS별 로그 확인:
   - Ubuntu: `journalctl -u systemd-networkd`
   - SUSE: `journalctl -u wicked`