# MultiNIC Agent - Host Daemon Mode

이 문서는 MultiNIC Agent를 Kubernetes DaemonSet이 아닌 호스트의 systemd 데몬으로 실행하는 방법을 설명합니다.

## 개요

호스트 데몬 모드는 다음과 같은 장점이 있습니다:
- 컨테이너 권한 문제 해결 (`privileged: true` 불필요)
- 더 나은 성능 (컨테이너 오버헤드 없음)
- 시스템 통합 개선 (systemd 직접 관리)
- 보안 강화 (최소 권한으로 실행)

## 요구사항

- Linux 시스템 (Ubuntu 18.04+ 또는 RHEL/CentOS 7+)
- systemd
- Go 1.21+ (바이너리 빌드용)
- MySQL/MariaDB 접근 권한

## 설치

### 1. 자동 설치 (권장)

```bash
# 저장소 클론
git clone https://github.com/Minzino/multinic-agent.git
cd multinic-agent

# 설치 스크립트 실행
sudo ./scripts/install-daemon.sh
```

### 2. 수동 설치

#### 바이너리 빌드
```bash
go build -o multinic-agent ./cmd/agent
sudo cp multinic-agent /usr/local/bin/
sudo chmod +x /usr/local/bin/multinic-agent
```

#### 설정 파일 생성
```bash
sudo mkdir -p /etc/multinic-agent
sudo cp deployments/systemd/config.env.template /etc/multinic-agent/config.env
sudo chmod 600 /etc/multinic-agent/config.env
```

#### 설정 파일 편집
```bash
sudo nano /etc/multinic-agent/config.env
```

다음 값들을 실제 환경에 맞게 수정:
```bash
DB_HOST=your-db-host
DB_PORT=3306
DB_USER=multinic
DB_PASSWORD=your-secure-password
DB_NAME=multinic_db
```

#### systemd 서비스 설치
```bash
sudo cp deployments/systemd/multinic-agent.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable multinic-agent
```

## 실행

### 서비스 시작
```bash
sudo systemctl start multinic-agent
```

### 상태 확인
```bash
sudo systemctl status multinic-agent
```

### 로그 확인
```bash
# 실시간 로그
sudo journalctl -u multinic-agent -f

# 최근 100줄
sudo journalctl -u multinic-agent -n 100

# 특정 시간 이후 로그
sudo journalctl -u multinic-agent --since "2025-01-16 10:00:00"
```

## 모니터링

### 헬스체크
```bash
curl http://localhost:8080/
```

### Prometheus 메트릭
```bash
curl http://localhost:8080/metrics
```

## 문제 해결

### 서비스가 시작되지 않을 때
1. 로그 확인: `sudo journalctl -u multinic-agent -n 50`
2. 설정 파일 확인: `sudo cat /etc/multinic-agent/config.env`
3. 데이터베이스 연결 테스트: `nc -zv <DB_HOST> <DB_PORT>`

### 네트워크 인터페이스가 생성되지 않을 때
1. 권한 확인: 서비스가 root로 실행되는지 확인
2. OS 타입 확인: 로그에서 "Operating system detected" 메시지 확인
3. 데이터베이스 데이터 확인: 호스트네임이 일치하는지 확인

## 제거

### 자동 제거
```bash
sudo ./scripts/uninstall-daemon.sh
```

### 수동 제거
```bash
# 서비스 중지 및 비활성화
sudo systemctl stop multinic-agent
sudo systemctl disable multinic-agent

# 파일 제거
sudo rm -f /etc/systemd/system/multinic-agent.service
sudo rm -f /usr/local/bin/multinic-agent
sudo rm -rf /etc/multinic-agent

# systemd 리로드
sudo systemctl daemon-reload
```

## Kubernetes에서 데몬 모드로 마이그레이션

1. **기존 DaemonSet 백업**
   ```bash
   kubectl get daemonset -n multinic-system multinic-agent -o yaml > multinic-agent-backup.yaml
   ```

2. **DaemonSet 제거**
   ```bash
   kubectl delete daemonset -n multinic-system multinic-agent
   ```

3. **각 노드에 데몬 설치**
   ```bash
   # 각 노드에서 실행
   sudo ./scripts/install-daemon.sh
   ```

4. **설정 동기화**
   - Kubernetes ConfigMap/Secret의 DB 정보를 `/etc/multinic-agent/config.env`로 복사

## 보안 고려사항

데몬 모드에서는 다음과 같은 보안 개선이 적용됩니다:
- systemd의 보안 기능 활용 (권한 제한, 리소스 제한)
- 파일 시스템 직접 접근으로 컨테이너 탈출 위험 제거
- 네트워크 네임스페이스 격리 불필요

## 성능 튜닝

### 폴링 간격 조정
```bash
# /etc/multinic-agent/config.env
POLL_INTERVAL=15s  # 더 빠른 반응을 위해 감소
```

### 동시 작업 수 조정
```bash
# /etc/multinic-agent/config.env
MAX_CONCURRENT_TASKS=10  # 더 많은 인터페이스 동시 처리
```

### 시스템 리소스 제한
```bash
# /etc/systemd/system/multinic-agent.service 수정
[Service]
CPUQuota=50%
MemoryLimit=512M
```