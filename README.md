# MultiNIC Agent v2

MultiNIC 네트워크 설정 관리를 위한 Kubernetes DaemonSet 에이전트

## 개요

이 에이전트는 Kubernetes 클러스터에 조인된 노드들의 네트워크 인터페이스 설정을 자동으로 관리합니다. 
데이터베이스를 모니터링하여 실패한 네트워크 설정을 감지하고 자동으로 재시도합니다.

## 주요 기능

- 데이터베이스 기반 설정 관리
- Ubuntu (Netplan) 및 SUSE (Wicked) 지원
- 자동 롤백 기능
- multinic0 ~ multinic9 인터페이스 관리
- 기존 네트워크 인터페이스 (eth0, ens* 등) 보호

## 아키텍처

```
┌─────────────────┐
│   Controller    │
│  (DB: MariaDB)  │
└────────┬────────┘
         │
    ┌────▼────┐
    │ Agent   │ (DaemonSet)
    │ - DB 모니터링
    │ - 설정 적용
    │ - 롤백 관리
    └─────────┘
```

## 설치 방법

### 빠른 배포 (권장)

```bash
# 자동으로 사용 가능한 도구 감지하여 배포
./scripts/simple-deploy.sh

# 환경 변수로 설정 변경
NAMESPACE=multinic-dev IMAGE_TAG=dev ./scripts/simple-deploy.sh
```

### containerd + nerdctl 환경

```bash
# buildkit이 없는 경우 설치
./scripts/setup-buildkit.sh

# buildkit 설치 후 배포
./scripts/nerdctl-deploy.sh
```

### Helm을 사용한 설치

```bash
# 기본 설치
helm install multinic-agent ./deployments/helm

# 커스텀 값 사용
helm install multinic-agent ./deployments/helm \
  --set database.host=192.168.34.79 \
  --set database.port=30305 \
  --set database.password=yourpassword
```

### values.yaml 설정

```yaml
database:
  host: "192.168.34.79"
  port: "30305"
  user: "root"
  password: "cloud1234"
  name: "multinic"

agent:
  pollInterval: "30s"
```

## 동작 방식

1. 에이전트는 주기적으로 데이터베이스의 `multi_interface` 테이블을 확인
2. `netplan_success = 0`이고 `attached_node_name`이 본인인 항목 발견
3. MAC 주소를 기반으로 네트워크 설정 자동 생성
4. 인터페이스 이름을 multinic0~9 형식으로 자동 할당 (최대 10개)
5. OS에 따라 적절한 네트워크 관리자 사용 (Netplan/Wicked)
6. 설정 적용 및 테스트 후 성공 시 `netplan_success = 1`로 업데이트
7. 실패 시 이전 설정으로 자동 롤백

## 데이터베이스 스키마

### multi_interface 테이블
```sql
CREATE TABLE multi_interface (
    id INT PRIMARY KEY AUTO_INCREMENT,
    attached_node_name VARCHAR(255),
    mac_address VARCHAR(17),
    netplan_success INT DEFAULT 0,
    ip_address VARCHAR(15),
    subnet_mask VARCHAR(15),
    gateway VARCHAR(15),
    dns VARCHAR(255),
    vlan INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);
```

## 개발

### 빌드
```bash
docker build -t multinic-agent:latest .
```

### 로컬 테스트
```bash
go run cmd/agent/main.go
```

## 로깅

에이전트는 JSON 형식으로 로그를 출력합니다:
- 설정 적용 시작/완료
- 오류 발생 시 상세 정보
- 롤백 수행 시 알림

## 문제 해결

### 에이전트가 설정을 적용하지 않음
1. 데이터베이스 연결 확인
2. 노드의 호스트네임과 DB의 vm_id 일치 여부 확인
3. 에이전트 로그 확인

### 네트워크 설정 실패
1. 백업 디렉토리 확인: `/var/lib/multinic/backups`
2. OS별 로그 확인:
   - Ubuntu: `journalctl -u systemd-networkd`
   - SUSE: `journalctl -u wicked`