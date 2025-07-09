### Dockerfile 최적화 및 문서 개선
- `Dockerfile`: `COPY . .` 명령을 `cmd`, `internal` 등 특정 디렉토리만 복사하도록 수정하여 Docker 이미지 빌드 캐시 효율을 개선함.
- `README_TEAM.md`: 배포 가이드를 대폭 수정하여, 불필요한 `export` 안내를 제거하고 `VAR=value ./script.sh` 방식의 실행을 안내하도록 단순화 및 명료화함.