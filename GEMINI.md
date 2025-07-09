
### 린터 지적사항 수정
- `golangci-lint` (errcheck)가 보고한 문제를 해결함.
- `internal/infrastructure/network/netplan_adapter.go`: 설정 테스트 실패 시 롤백을 위해 설정 파일을 삭제하는 과정에서 발생하는 에러를 로그에 기록하도록 수정하여, 잠재적인 자동 롤백 실패를 추적할 수 있도록 개선함.
