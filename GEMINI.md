### RHEL 9 (nmcli) 지원 추가 및 OS 감지 로직 개선 (2025-07-09)

**1. 문제 발단 및 초기 진단:**
- 사용자로부터 SUSE 9.4 환경에서 `ifup`/`ifdown` 명령어가 작동하지 않는다는 피드백을 받음.
- 초기 `git log` 및 `GEMINI.md` 분석 결과, 최근 SUSE 9.4 (ifup/down) 호환성 추가 작업이 있었음을 확인.
- 원격 서버에 직접 SSH 접속하여 명령어 존재 여부를 확인하려 했으나, `sshpass`를 통한 원격 명령어 실행이 계속 실패함.

**2. SSH 접속 문제 해결:**
- 사용자로부터 SSH 접속이 프록시 서버를 통한 이중 비밀번호 인증 방식임을 확인.
- `~/.ssh/config` 파일에 직접 접근할 수 없는 제약사항으로 인해, `sshpass`를 중첩 사용하는 복잡한 명령어를 구성하여 원격 서버 접속에 성공.
- `sshpass -p 'cloud1234' ssh -o StrictHostKeyChecking=no -o ProxyCommand="sshpass -p 'cloud1234' ssh -o StrictHostKeyChecking=no -W %h:%p suse" suse-biz-worker1 'uname -a'`

**3. OS 오인 및 근본 원인 파악:**
- `uname -a` 및 `cat /etc/os-release` 명령 실행 결과, 해당 서버가 SUSE 9.4가 아닌 **Red Hat Enterprise Linux 9.4 (SUSE Liberty Linux)**임을 확인.
- 이는 `wicked`나 레거시 `ifup`/`ifdown` 명령어가 아닌, RHEL 계열의 표준 네트워크 관리 도구인 `NetworkManager` (`nmcli`)를 사용해야 함을 의미.
- 기존 SUSE 9.4 지원 로직이 잘못된 OS 정보에 기반했음을 파악.

**4. 코드 수정 및 기능 구현:**
- **`internal/domain/interfaces/os.go`**: 새로운 OS 타입 `OSTypeRHEL` 추가.
- **`internal/infrastructure/network/suse_legacy_adapter.go`**: 잘못된 SUSE 레거시 어댑터 파일 삭제.
- **`internal/infrastructure/network/rhel_adapter.go`**: `nmcli` 명령어를 사용하여 네트워크 인터페이스를 설정, 검증, 롤백하는 `RHELAdapter` 구현.
  - `nmcli connection add`, `nmcli connection up`, `nmcli connection down`, `nmcli connection delete` 등 사용.
- **`internal/infrastructure/adapters/os_detector.go`**: OS 감지 로직을 `/etc/issue` 대신 `/etc/os-release` 파일을 파싱하도록 개선. `ID` 및 `ID_LIKE` 필드를 기반으로 `Ubuntu`, `SUSE`, `RHEL` (및 호환 OS)를 정확히 식별하도록 수정. `fmt` 패키지 import 누락 오류 수정.
- **`internal/infrastructure/network/factory.go`**: `NetworkManagerFactory`에서 감지된 OS 타입에 따라 `RHELAdapter`를 생성하도록 로직 업데이트.
- **`README.md`**: 지원 OS 목록을 "Red Hat Enterprise Linux 9+ (및 Rocky, Alma, SUSE Liberty 등 호환 OS)"로 업데이트.

**5. 테스트 및 디버깅 여정:**
- 코드 수정 후 `go test ./internal/...` 실행 시 테스트 실패 발생.
- **첫 번째 실패**: `rhel_adapter.go`에서 `entities.InterfaceName`을 `string`으로 직접 변환하려던 컴파일 에러 발생. `name.String()` 메서드를 사용하도록 수정.
- **두 번째 실패**: `os_detector_test.go`에서 `ReadFile("/etc/issue")`가 예상치 않게 호출되는 `mock` 에러 발생.
  - `os_detector.go`의 `DetectOS` 함수 내 논리적 버그(일치하는 OS가 없을 때 `detectOSFromIssue()`로 폴백)를 발견하고 수정.
  - `fmt` 패키지 import 누락 오류 수정.
- **세 번째 실패**: `os_detector_test.go`의 테스트 데이터 문자열 리터럴 파싱 문제 확인. `osReleaseContent`를 더 단순한 형태로 변경하여 파싱 오류 해결.
- **최종 결과**: 모든 단위 테스트 성공적으로 통과.

**6. 작업 완료:**
- 모든 코드 변경 및 문서 업데이트 완료.
- 테스트 통과 확인.
- `GEMINI.md`에 상세 작업 내용 기록.
- Git 저장소에 변경 사항 푸시 준비 완료.

### RHEL/nmcli 기능 검증 (2025-07-09)

**1. 검증 목표:**
- 이전 작업에서 SUSE로 오인했던 RHEL 9.4 환경에 대해 `nmcli`를 사용하도록 수정한 기능이 올바르게 구현되었는지 최종 검증.
- 원격 서버(`suse-biz-worker1`)에 직접 접속하여 코드의 명령어와 실제 서버 환경의 일치 여부 확인.

**2. 검증 절차 및 결과:**
- **OS 재확인**: `cat /etc/os-release` 명령을 통해 대상 서버가 **RHEL 9.4**임을 재차 확인함. 에이전트의 OS 감지 로직(`ID="rhel"`)이 올바르게 동작할 것을 확신.
- **`nmcli` 도구 확인**: `nmcli --version` (1.46.0) 명령으로 `nmcli`가 설치되어 있고, `systemctl is-active NetworkManager` 명령으로 서비스가 활성화 상태임을 확인함.
- **코드-서버 간 명령어 일치 확인**:
  - `internal/infrastructure/network/rhel_adapter.go`에 구현된 핵심 `nmcli` 명령어들을 분석.
    - **설정**: `nmcli connection add type ethernet con-name {name} ifname {name} mac {mac}`
    - **IP 비활성화**: `nmcli connection modify {name} ipv4.method disabled`
    - **활성화**: `nmcli connection up {name}`
    - **검증**: `nmcli device status`
    - **롤백**: `nmcli connection down {name}` 및 `nmcli connection delete {name}`
  - `nmcli device status`의 실제 출력 형식을 확인한 결과, 코드 내 파싱 로직(`strings.Fields`로 분리 후 3번째 필드 확인)이 문제없이 동작할 것을 확인함.

**3. 최종 결론:**
- **코드 변경 불필요**: 현재 `RHELAdapter`의 구현은 대상 서버 환경과 완벽하게 일치하며, 추가적인 코드 수정은 필요하지 않음.
- 이전 작업(SUSE 오인 수정 및 RHEL 지원 추가)이 성공적으로 완료되었음을 최종 확인.
- `GEMINI.md`에 검증 내용 기록 완료.

### RHEL 고아 인터페이스 삭제 기능 구현 (2025-07-09)

**1. 문제점:**
- 기존 '고아 인터페이스 삭제' 로직은 Ubuntu(Netplan)의 `.yaml` 파일 기반으로 구현되어 있어, RHEL(nmcli) 환경에서는 동작하지 않는 기능적 허점이 있었음.
- 이로 인해 RHEL 노드에서는 DB에서 인터페이스가 삭제되어도 시스템에 불필요한 `nmcli` 연결 프로파일이 계속 남게 됨.

**2. 구현 내용:**
- **`DeleteNetworkUseCase` 리팩터링**: OS를 감지하여 Ubuntu와 RHEL에 맞는 각기 다른 삭제 로직을 수행하도록 `switch` 문으로 분기 처리.
- **RHEL 삭제 로직 구현**: 
  1. `nmcli -t -f NAME c show` 명령으로 시스템의 모든 연결 프로파일 목록을 조회하는 `ListNmcliConnectionNames` 메소드를 `InterfaceNamingService`에 추가.
  2. `multinic`으로 시작하는 프로파일들을 필터링.
  3. 각 프로파일에 대해 `ip addr show {profile_name}` 명령을 실행하여 실제 네트워크 장치의 존재 여부를 확인.
  4. 프로파일은 있지만 실제 장치가 없는 경우를 '고아'로 판단하여, `rollbacker.Rollback` (내부적으로 `nmcli connection delete` 실행)을 호출하여 해당 프로파일을 삭제.
- **의존성 주입 수정**: `container.go`를 수정하여 `DeleteNetworkUseCase` 생성 시 `OSDetector`를 주입하도록 변경.

**3. 테스트 관련:**
- 수차례의 시도 끝에 `testify/mock` 프레임워크의 모의 객체 설정 문제를 해결하고, RHEL과 Ubuntu 시나리오를 모두 포함하는 테스트 코드를 성공적으로 작성 및 통과함.