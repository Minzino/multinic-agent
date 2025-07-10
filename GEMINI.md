### Netplan 동기화 및 로그 최적화 디버깅 (2025-07-10)

**1. 문제 발단:**
- Netplan 설정 파일(특히 구형 포맷)이 DB에 IP 정보가 있음에도 불구하고 최신 포맷으로 업데이트되지 않는 문제 발생.
- 에이전트 로그에 "고아 인터페이스 삭제 프로세스 시작"과 같은 불필요한 `debug` 레벨 로그가 과도하게 출력되어 실제 문제 파악이 어려움.

**2. 초기 진단 및 오해:**
- `netplan_success`가 1인 경우에도 동기화 로직이 작동해야 함을 인지.
- `syncConfiguredInterfaces` 함수 내의 `isDrifted` 로직이 구형 포맷을 제대로 감지하지 못하는 것으로 추정.
- `LOG_LEVEL=debug` 환경 변수가 불필요한 로그를 발생시키는 원인으로 파악.

**3. 근본 원인 재파악 및 디버깅 여정:**
- `isDrifted` 함수가 예상과 달리 `false`를 반환하여 동기화가 건너뛰어지는 현상 확인.
- `internal/application/usecases/configure_network.go` 파일의 `isDrifted` 함수 내부에 각 비교 조건의 결과와 최종 `isDrifted` 값을 상세하게 출력하는 디버그 로그 추가.
- `isDrifted` 함수 내의 괄호 구문 오류 수정.
- `internal/infrastructure/persistence/mysql_repository.go` 파일에서 `GetActiveInterfaces` 메서드가 중복 정의되어 있고, `GetAllNodeInterfaces` 메서드가 `NetworkInterfaceRepository` 인터페이스에 정의되어 있으나 구현체에 없다는 컴파일 오류 발생.
- `mysql_repository.go` 파일에서 중복된 `GetActiveInterfaces` 메서드를 제거하고, 기존 `GetActiveInterfaces` 메서드의 이름을 `GetAllNodeInterfaces`로 변경하며 쿼리에서 `LIMIT 10`을 제거하여 해당 노드의 모든 인터페이스를 조회하도록 수정.
- `internal/application/usecases/configure_network.go` 파일의 `Execute` 함수에서 `GetPendingInterfaces` 대신 `GetAllNodeInterfaces`를 사용하도록 수정.

**4. 테스트 및 완료:**
- 모든 코드 수정 후 `go test ./...` 실행하여 모든 단위 테스트가 성공적으로 통과함을 확인.
- `GEMINI.md`에 상세 작업 내용 기록.
- Git 저장소에 변경 사항 푸시 완료.

**5. 다음 단계:**
- 배포 후 에이전트 로그에서 `isDrifted` 함수의 상세 디버그 로그를 확인하여 Netplan 동기화가 왜 제대로 이루어지지 않는지 정확한 원인 분석 필요.
- `LOG_LEVEL` 환경 변수 설정이 `values.yaml`에 따라 `info`로 제대로 적용되는지 추가 확인 필요.
