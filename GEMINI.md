### SUSE 9.4 호환성 개선
- `wicked_adapter.go` 파일을 `suse_legacy_adapter.go`로 이름 변경하고, 내부 코드(구조체, 함수명, 주석)를 `SuseLegacyAdapter`로 업데이트함.
- `activateInterface` 및 `deactivateInterface` 함수에서 `wicked` 명령어 대신 `ifup` 및 `ifdown` 명령어를 사용하도록 수정하여 SUSE 9.4 환경에 맞춤.
- `factory.go`에서 `NewWickedAdapter` 대신 `NewSuseLegacyAdapter`를 호출하도록 업데이트함.
- `README.md` 및 `README_TEAM.md`의 지원 OS 목록에서 'SUSE Linux Enterprise 15+'를 'SUSE Linux 9.4'로 변경하여 문서의 정확성을 높임.