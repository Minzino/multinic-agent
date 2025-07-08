# MultiNIC Agent v2 프로젝트 분석

## 프로젝트 개요

MultiNIC Agent v2는 Kubernetes 클러스터에 조인된 노드들의 네트워크 인터페이스 설정을 자동으로 관리하는 Go 기반의 DaemonSet 에이전트입니다.

### 주요 특징
- 데이터베이스 기반 설정 관리 (MySQL/MariaDB)
- Ubuntu (Netplan) 및 SUSE (Wicked) 지원
- 자동 롤백 기능
- multinic0 ~ multinic9 인터페이스 관리 (최대 10개)
- 기존 네트워크 인터페이스 (eth0, ens* 등) 보호

## 아키텍처

```
┌─────────────────┐
│   Controller    │
│  (DB: MariaDB)  │ → 네트워크 설정 정보 저장
└────────┬────────┘
         │
    ┌────▼────┐
    │ Agent   │ (DaemonSet)
    │ - DB 모니터링 (30초 주기)
    │ - 설정 적용
    │ - 롤백 관리
    │ - 헬스체크
    └─────────┘
```

## 기술 스택

### 핵심 기술
- **언어**: Go 1.21
- **데이터베이스**: MySQL/MariaDB
- **배포**: Kubernetes DaemonSet
- **패키징**: Helm Chart

### 주요 의존성
- `github.com/go-sql-driver/mysql v1.7.1` - MySQL 드라이버
- `github.com/sirupsen/logrus v1.9.3` - 구조화된 로깅
- `gopkg.in/yaml.v3 v3.0.1` - YAML 파싱 (netplan 설정)

### 빌드 환경
- **Multi-stage Docker 빌드**
  - 빌드: `golang:1.21-alpine`
  - 실행: `alpine:3.18`
- **최소 이미지 크기**: Alpine 기반으로 경량화

## 프로젝트 구조

### 현재 구조 (리팩터링 진행 중)
```
multinic-agent-v2/
├── cmd/agent/          # 메인 애플리케이션
│   └── main.go         # 진입점
├── internal/           # 클린 아키텍처 구조 (NEW)
│   ├── domain/         # 비즈니스 로직 계층
│   │   ├── entities/   # 도메인 엔티티
│   │   ├── errors/     # 도메인 에러 정의
│   │   ├── interfaces/ # 도메인 인터페이스
│   │   └── services/   # 도메인 서비스
│   ├── application/    # 애플리케이션 계층
│   │   └── usecases/   # 유스케이스
│   ├── infrastructure/ # 인프라스트럭처 계층
│   │   ├── persistence/# 데이터베이스 구현
│   │   ├── network/    # 네트워크 관리 구현
│   │   ├── health/     # 헬스체크 구현
│   │   └── config/     # 설정 관리
│   └── interfaces/     # 인터페이스 어댑터
│       ├── http/       # HTTP 핸들러
│       └── cli/        # CLI 인터페이스
├── pkg/                # 기존 패키지 (마이그레이션 예정)
│   ├── db/             # 데이터베이스 연동
│   ├── health/         # 헬스체크 시스템
│   ├── netplan/        # Netplan 설정 관리
│   ├── network/        # 네트워크 관리 추상화
│   └── utils/          # 유틸리티 함수
├── deployments/helm/   # Helm 차트
├── scripts/            # 배포 및 테스트 스크립트
└── test/               # 통합 테스트
```

## 핵심 기능 분석

### 1. 메인 프로세스 (cmd/agent/main.go)

#### 초기화 과정
1. 환경 변수 기반 설정 로드
2. 데이터베이스 연결 (재시도 로직 포함)
3. OS 감지 및 NetworkManager 생성
4. 헬스체크 서버 시작 (포트 8080)
5. 30초 주기로 폴링 시작

#### 처리 흐름
```go
processConfigurations() {
    1. 호스트네임 가져오기 및 검증
    2. DB에서 대기 중인 인터페이스 조회 (netplan_success=0)
    3. 각 인터페이스별로:
       - 인터페이스 이름 생성 (multinic0~9)
       - 네트워크 설정 적용 (재시도 2회)
       - 성공 시 DB 상태 업데이트
       - 실패 시 헬스체크에 기록
}
```

### 2. 네트워크 관리 시스템 (pkg/network/)

#### 설계 패턴
- **Strategy Pattern**: OS별 구현 분리
- **Factory Pattern**: OS 자동 감지 및 적절한 관리자 생성

#### NetworkManager 인터페이스
```go
type NetworkManager interface {
    ApplyConfiguration(configData []byte, interfaceName string) error
    Rollback(interfaceName string) error
    ValidateInterface(interfaceName string) bool
    GetType() string
    ConfigureInterface(iface db.MultiInterface, interfaceName string) error
}
```

#### OS별 구현

**Ubuntu (Netplan)**
- 설정 파일: `/etc/netplan/9{index}-{interface}.yaml`
- 백업 경로: `/var/lib/multinic/backups`
- 특징:
  - `netplan try --timeout=120`으로 안전한 테스트
  - 컨테이너 환경에서 `nsenter` 사용
  - YAML 기반 설정

**SUSE (Wicked)**
- 설정 파일: `/etc/sysconfig/network/ifcfg-{interface}`
- 백업 경로: `/var/lib/multinic/wicked-backups`
- 특징:
  - `wicked ifup` 명령으로 개별 인터페이스 제어
  - 키-값 쌍 기반 설정

### 3. 데이터베이스 연동 (pkg/db/)

#### 연결 풀 설정
- 최대 동시 연결: 10개
- 최대 유휴 연결: 5개
- 연결 수명: 5분

#### 주요 쿼리
1. **GetPendingInterfaces**: 처리 대기 인터페이스 조회
   ```sql
   SELECT id, mac_address, attached_node_name, netplan_success 
   FROM multi_interface 
   WHERE netplan_success = 0 
   AND attached_node_name = ? 
   AND deleted_at IS NULL 
   LIMIT 10
   ```

2. **UpdateInterfaceStatus**: 처리 결과 업데이트
   ```sql
   UPDATE multi_interface 
   SET netplan_success = ?, modified_at = NOW() 
   WHERE id = ?
   ```

### 4. 헬스체크 시스템 (pkg/health/)

#### HTTP 엔드포인트
- URL: `GET /`
- 상태 코드:
  - 200: healthy/degraded
  - 503: unhealthy

#### 모니터링 항목
- 데이터베이스 연결 상태
- 처리된 VM 수
- 실패한 설정 수
- 서비스 가동 시간

## 보안 및 안정성 기능

### 1. 입력 검증
- 호스트네임 검증
- MAC 주소 형식 검증
- 인터페이스 이름 패턴 검증 (multinic[0-9])

### 2. 백업 및 롤백
- 설정 변경 전 자동 백업
- 타임스탬프 기반 백업 파일 관리
- 실패 시 최신 백업으로 자동 복원

### 3. 재시도 로직
- 지수 백오프를 사용한 재시도
- DB 연결: 최대 5회, 초기 지연 1초
- 네트워크 설정: 최대 2회, 초기 지연 2초

### 4. 타임아웃 처리
- 명령 실행: 30초 타임아웃
- Netplan 테스트: 120초 타임아웃
- 프로세스 강제 종료 지원

## 배포 방식

### 1. Helm Chart
- 환경별 설정 파일 지원 (dev/prod)
- RBAC 설정 포함
- DaemonSet으로 모든 노드에 배포

### 2. 배포 스크립트
- `deploy.sh`: 원클릭 배포
- `test-deployment.sh`: 배포 검증
- `test-functionality.sh`: 기능 테스트

## 모니터링 및 디버깅

### 로그 형식
- JSON 구조화 로깅
- 필드 기반 컨텍스트 정보
- 예시:
  ```json
  {
    "level": "info",
    "msg": "인터페이스 설정 적용 중",
    "interface_id": 123,
    "interface_name": "multinic0",
    "time": "2025-01-08T..."
  }
  ```

### 디버깅 포인트
1. 에이전트 로그: `kubectl logs -n multinic-system`
2. DB 연결 상태: 헬스체크 엔드포인트
3. 백업 파일: 노드의 `/var/lib/multinic/backups`

## 개선 사항 및 고려사항

### 현재 제한사항
- 최대 10개 인터페이스 지원 (multinic0~9)
- 30초 고정 폴링 주기
- 단방향 동기화 (DB → 노드)

### 향후 개선 가능 영역
1. 동적 폴링 주기 조정
2. 인터페이스 수 제한 확장
3. 양방향 동기화 지원
4. Prometheus 메트릭 추가

## 리팩터링 진행 상황

### Phase 1: 기반 구조 개선 (완료)
1. ✅ **클린 아키텍처 디렉토리 구조 생성**
   - 도메인, 애플리케이션, 인프라스트럭처 레이어 분리
   
2. ✅ **도메인 레이어 구현**
   - NetworkInterface 엔티티 정의
   - Repository, Network, OS 관련 인터페이스 정의
   - InterfaceNamingService 도메인 서비스 구현
   
3. ✅ **에러 처리 체계 구축**
   - 타입별 도메인 에러 정의 (Validation, NotFound, System 등)
   - 일관된 에러 생성 및 처리 패턴
   
4. ✅ **Repository 패턴 구현**
   - MySQLRepository 구현
   - 도메인과 인프라 계층 분리

5. ✅ **OS 감지 로직 개선**
   - /etc/issue 파일 기반으로 단순화

### Phase 2: 핵심 로직 리팩터링 (완료)
1. ✅ **인프라스트럭처 어댑터 구현**
   - OS 감지기 (RealOSDetector)
   - 파일 시스템 어댑터 (RealFileSystem, RealClock)
   - 설정 로더 (EnvironmentConfigLoader)
   
2. ✅ **네트워크 관리 시스템 구현**
   - NetplanManager와 WickedManager 어댑터
   - 통합된 백업 서비스 (BackupService)
   - 헬스 체크 서비스 (HealthService)
   
3. ✅ **애플리케이션 유스케이스 구현**
   - ConfigureNetworkUseCase
   - 모든 비즈니스 로직 캡슐화
   
4. ✅ **의존성 주입 컨테이너**
   - 전체 시스템 조립 및 관리
   - 생명주기 관리 (graceful shutdown)
   
5. ✅ **main.go 완전 리팩터링**
   - 250줄 → 179줄로 코드 축소
   - Application 구조체로 관심사 분리
   - 의존성 주입을 통한 테스트 가능성 향상

### Phase 3: 테스트 및 검증 (완료)
1. ✅ **단위 테스트 작성**
   - 도메인 엔티티 테스트 (100% 커버리지)
     * NetworkInterface 유효성 검증 테스트
     * MAC 주소/인터페이스 이름 형식 검증
     * 상태 변경 메서드 테스트
   
   - 도메인 서비스 테스트 (100% 커버리지)
     * InterfaceNamingService 인터페이스 이름 생성 로직
     * 사용 중인 인터페이스 감지 로직
   
   - 유스케이스 테스트 (88.2% 커버리지)
     * 네트워크 설정 성공/실패 시나리오
     * 롤백 로직 및 에러 처리
     * Mock 의존성을 활용한 완전 격리 테스트
   
   - 인프라스트럭처 어댑터 테스트
     * OS 감지 어댑터 (다양한 OS 형식 테스트)
     * 설정 로더 (환경 변수 처리 테스트)

2. ✅ **통합 테스트 구현**
   - 클린 아키텍처 구성 요소 간 통합 테스트
   - 실제 의존성과 Mock 의존성 혼합 테스트
   - 컨테이너 초기화 및 라이프사이클 테스트

3. ✅ **테스트 인프라스트럭처**
   - testify/mock 라이브러리 도입
   - 모든 도메인 인터페이스에 대한 Mock 구현
   - 일관된 테스트 패턴 및 구조

## 포스트모템

### 프로젝트 강점
1. **확장성**: Strategy/Factory 패턴으로 새로운 OS 지원 용이
2. **안정성**: 백업/롤백, 재시도, 타임아웃 등 다중 안전장치
3. **운영성**: 구조화된 로깅, 헬스체크, Helm 차트 제공
4. **보안성**: 입력 검증, 기존 인터페이스 보호
5. **유지보수성**: 클린 아키텍처 도입으로 관심사 분리 개선

### 기술적 의사결정
1. **Go 선택**: 경량 바이너리, 효율적인 동시성 처리
2. **DaemonSet 사용**: 모든 노드에 자동 배포 및 관리
3. **DB 기반 설정**: 중앙 집중식 관리, 상태 추적 용이
4. **클린 아키텍처 도입**: 테스트 가능성과 확장성 개선

### 리팩터링 성과 및 개선 사항

#### 정량적 개선 지표
1. **코드 복잡도 감소**: main.go 코드 250줄 → 179줄 (28% 감소)
2. **테스트 커버리지**: 핵심 도메인 로직 90%+ 커버리지 달성
3. **아키텍처 레이어 분리**: 단일 파일 → 4개 계층으로 구조화
4. **의존성 관리**: 순환 참조 제거, 인터페이스 기반 느슨한 결합

#### 품질 개선 사항
1. **테스트 가능성**: Mock을 통한 완전 격리 테스트 환경 구축
2. **유지보수성**: 도메인 중심 설계로 비즈니스 로직 명확화
3. **확장성**: 새로운 OS 지원, 기능 추가가 용이한 구조
4. **타입 안전성**: 강타입 도메인 엔티티와 에러 처리

#### 클린 아키텍처 적용 효과
- **의존성 역전**: 인프라스트럭처가 도메인에 의존하는 구조
- **관심사 분리**: 각 레이어별 명확한 책임 정의
- **비즈니스 로직 보호**: 외부 의존성으로부터 도메인 로직 격리
- **테스트 전략**: 단위/통합/E2E 테스트 레벨별 명확한 구분

### 기술적 도전과 해결책

#### 도전 1: 기존 레거시 코드와의 통합
**문제**: 기존 pkg/ 패키지와 새로운 internal/ 구조 간 충돌
**해결**: 어댑터 패턴을 통한 점진적 마이그레이션

#### 도전 2: Mock 타입 호환성 문제
**문제**: Mock 객체의 메서드 시그니처 불일치
**해결**: testify/mock 표준 사용, 인터페이스 정확한 구현

#### 도전 3: 테스트 간 격리 문제
**문제**: 파일 시스템 의존성으로 인한 테스트 간 간섭
**해결**: Mock FileSystem 도입, 실제 파일 시스템과 완전 격리

### 학습 포인트
1. **컨테이너 환경 네트워크 제어**: nsenter를 통한 호스트 네임스페이스 접근
2. **OS별 네트워크 도구 차이**: Ubuntu Netplan vs SUSE Wicked의 설정 방식 및 명령어
3. **안전한 네트워크 설정**: 백업/롤백, 타임아웃, 검증을 통한 무중단 변경
4. **점진적 리팩터링**: 기존 시스템을 유지하면서 새 아키텍처로 전환하는 전략
5. **도메인 주도 설계**: 비즈니스 로직을 중심으로 한 계층 구조 설계
6. **테스트 주도 개발**: Mock을 활용한 격리된 단위 테스트 작성법

### 향후 개선 방향
1. **통합 테스트 확장**: 실제 DB와 네트워크 환경을 사용한 E2E 테스트
2. **성능 최적화**: 폴링 주기 동적 조정, 배치 처리 최적화
3. **모니터링 강화**: Prometheus 메트릭, OpenTelemetry 추적 도입
4. **기능 확장**: IPv6 지원, 고급 네트워크 설정 옵션

## 배포 스크립트 개선 사항 (2025-01-08)

### 개선 내용
1. **전체 노드 지원**: 워커 노드만이 아닌 모든 노드에 이미지 배포
   - 기존: 하드코딩된 워커 노드 목록 (viola2-biz-worker01, worker02, worker03)
   - 개선: `kubectl get nodes`를 통한 동적 노드 목록 가져오기
   
2. **변경 사항**
   ```bash
   # 기존
   WORKER_NODES=(viola2-biz-worker01 viola2-biz-worker02 viola2-biz-worker03)
   
   # 개선
   ALL_NODES=($(kubectl get nodes -o jsonpath='{.items[*].metadata.name}'))
   ```

3. **영향 범위**
   - DaemonSet이 모든 노드에서 실행되도록 설계되어 있으므로, 이미지도 모든 노드에 배포되어야 함
   - OpenStack multi-interface 관리를 위해 컨트롤 플레인 노드에서도 실행 필요
   - 노드 추가/제거 시 자동으로 반영되어 유지보수성 향상

### Toleration 설정 추가
컨트롤 플레인 노드에도 DaemonSet이 배포되도록 toleration 설정 추가:

```yaml
tolerations:
  # 컨트롤 플레인 노드에도 배포되도록 설정
  - key: node-role.kubernetes.io/control-plane
    operator: Exists
    effect: NoSchedule
  - key: node-role.kubernetes.io/master
    operator: Exists
    effect: NoSchedule
```

이 설정으로 `node-role.kubernetes.io/control-plane:NoSchedule` taint가 있는 마스터 노드에도 Pod가 스케줄링됩니다.

## 데이터베이스 스키마 불일치 버그 수정 (2025-01-08)

### 문제 발생
배포 후 다음과 같은 에러 발생:
```
Error 1054 (42S22): Unknown column 'ip_address' in 'field list'
```

### 원인
- 코드에서 존재하지 않는 필드들을 조회 시도 (`ip_address`, `subnet_mask`, `gateway`, `dns`, `vlan`)
- 실제 테이블에는 `macaddress`, `attached_node_name` 등만 존재
- Netplan 설정은 MAC 주소와 인터페이스 이름만 필요

### 수정 내용
1. **엔티티 구조 단순화**
   ```go
   // 기존
   type NetworkInterface struct {
       ID, MacAddress, AttachedNodeName string
       IPAddress, SubnetMask, Gateway, DNS string  // 제거
       VLAN int                                     // 제거
   }
   
   // 변경
   type NetworkInterface struct {
       ID               int
       MacAddress       string
       AttachedNodeName string
       Status           InterfaceStatus
   }
   ```

2. **데이터베이스 쿼리 수정**
   ```sql
   -- 기존
   SELECT id, macaddress, attached_node_name, ip_address, 
          subnet_mask, gateway, dns, vlan
   
   -- 변경  
   SELECT id, macaddress, attached_node_name, netplan_success
   ```

3. **Netplan 설정 단순화**
   ```yaml
   # 실제 생성되는 설정
   network:
     ethernets:
       multinic0:
         dhcp4: false
         match:
           macaddress: fa:16:3e:b1:29:8f
         set-name: multinic0
         mtu: 1500
     version: 2
   ```

4. **Wicked 설정도 동일하게 단순화**
   - BOOTPROTO를 'static'에서 'none'으로 변경
   - IP 관련 설정 모두 제거
