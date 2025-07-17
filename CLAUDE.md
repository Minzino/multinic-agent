# CLAUDE.md

This file contains essential information for Claude instances working on the MultiNIC Agent codebase.

## Core Commands

### Build & Test
```bash
# Build the agent
go build -o bin/multinic-agent cmd/agent/main.go

# Run all tests
go test ./...

# Run tests with coverage
go test -v -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run specific package tests
go test ./internal/application/usecases -v

# Lint the code (requires golangci-lint)
golangci-lint run

# Format code
go fmt ./...

# Vet code
go vet ./...
```

### Deployment
```bash
# Deploy with script (requires SSH_PASSWORD env var)
export SSH_PASSWORD="your_ssh_password"
./scripts/deploy.sh

# Deploy with custom settings
NAMESPACE=multinic-prod IMAGE_TAG=v2.0.0 ./scripts/deploy.sh

# Check deployment status
kubectl get daemonset -n multinic-system multinic-agent
kubectl get pods -n multinic-system -l app.kubernetes.io/name=multinic-agent -o wide
kubectl logs -n multinic-system -l app.kubernetes.io/name=multinic-agent -f
```

### Docker & Helm
```bash
# Build Docker image
docker build -t multinic-agent:latest .

# Install with Helm
helm install multinic-agent ./deployments/helm --namespace multinic-system

# Upgrade Helm release
helm upgrade multinic-agent ./deployments/helm --namespace multinic-system
```

## Architecture Overview

### Clean Architecture Layers
The codebase follows Clean Architecture principles with clear separation of concerns:

1. **Domain Layer** (`internal/domain/`)
   - Pure business logic, no external dependencies
   - Key entities: `NetworkInterface`, `InterfaceName`, `InterfaceStatus`
   - Domain services: `InterfaceNamingService` (handles multinic0-9 naming)
   - All errors are domain-specific (ValidationError, NotFoundError, etc.)

2. **Application Layer** (`internal/application/`)
   - Use cases orchestrate domain logic
   - `ConfigureNetworkUseCase`: Creates/updates network interfaces
   - `DeleteNetworkUseCase`: Removes orphaned interfaces
   - Polling strategies for backoff handling

3. **Infrastructure Layer** (`internal/infrastructure/`)
   - External system integrations
   - Database: MySQL repository with connection pooling
   - Network: OS-specific adapters (Netplan for Ubuntu, ifcfg for RHEL)
   - Health checks and metrics (Prometheus)

4. **Dependency Injection** (`internal/infrastructure/container/`)
   - Central container manages all dependencies
   - Ensures proper lifecycle management
   - Facilitates testing with mock implementations

### Key Design Patterns

1. **Repository Pattern**: Database access abstracted behind interfaces
2. **Adapter Pattern**: OS-specific network management (NetplanAdapter, RHELAdapter)
3. **Factory Pattern**: Dynamic OS detection and adapter creation
4. **Strategy Pattern**: Polling strategies (fixed interval vs exponential backoff)

## Critical Implementation Details

### Network Interface Management

**Ubuntu (Netplan)**:
- Config files: `/etc/netplan/9X-multinicX.yaml`
- Uses `netplan try --timeout=120` for safe testing
- Runs in container via `nsenter` to access host namespace
- File naming: `9{index}-{interface}.yaml` (e.g., `91-multinic1.yaml`)

**RHEL/CentOS (ifcfg)**:
- Config files: `/etc/sysconfig/network-scripts/ifcfg-multinicX`
- Direct file manipulation (no nmcli in containers)
- Interface renaming via `ip link set` commands
- File format: KEY=VALUE pairs

### Database Interaction

**Important**: Do NOT use `deleted_at` in SQL queries. This field is managed by the Controller for CR lifecycle, not for Agent filtering.

```sql
-- CORRECT: Get pending interfaces
SELECT id, macaddress, attached_node_name, netplan_success, address, cidr, mtu 
FROM multi_interface 
WHERE attached_node_name = ? 
LIMIT 10

-- WRONG: Don't filter by deleted_at
WHERE attached_node_name = ? 
AND deleted_at IS NULL  -- DON'T DO THIS
```

### Orphan Detection Logic

The agent detects orphaned interfaces by comparing:
1. **Ubuntu**: Netplan files vs DB active MAC addresses
2. **RHEL**: ifcfg files vs DB active MAC addresses

Files with MAC addresses not in the database are considered orphans and removed.

### Configuration Drift Detection

The agent monitors for configuration changes:
- IP address changes
- CIDR/network changes  
- MTU changes
- Missing configuration files

Any mismatch triggers automatic reconfiguration.

## Important Gotchas

1. **Hostname Handling**: The agent strips domain suffixes (e.g., `.novalocal`) before DB queries
2. **MAC Address Format**: Always lowercase for comparisons
3. **Interface Naming**: Strictly `multinic0` through `multinic9` (max 10 interfaces)
4. **Polling**: Fixed 30-second interval by default, configurable with exponential backoff
5. **Container Privileges**: Requires `hostNetwork: true` and `hostPID: true` for network management

## Testing Guidelines

### Unit Tests
- Mock all external dependencies using `testify/mock`
- Test both success and failure paths
- Verify domain validation rules
- Check error types and messages

### Integration Tests
- Use real file system operations where possible
- Test OS detection logic
- Verify network configuration parsing
- Check database query behavior

## Monitoring & Debugging

### Key Metrics (Prometheus)
- `multinic_interfaces_processed_total`: Success/failure counts
- `multinic_configuration_drifts_total`: Drift detection events
- `multinic_orphaned_interfaces_deleted_total`: Cleanup operations
- `multinic_db_connection_status`: Database health

### Log Levels
- **Info**: Important state changes only (minimize noise)
- **Debug**: Detailed operation logs (hidden in production)
- **Error**: Actual failures requiring attention
- **Warn**: Non-critical issues (e.g., parse errors)

### Health Check
```bash
# Port forward and check health
kubectl port-forward -n multinic-system daemonset/multinic-agent 8080:8080
curl http://localhost:8080/
```

## Development Workflow

1. **Before Making Changes**:
   - Understand the clean architecture boundaries
   - Check existing tests for patterns
   - Review domain entities and interfaces

2. **Making Network Changes**:
   - OS detection happens once at startup
   - Network adapters are OS-specific
   - Always test with both Ubuntu and RHEL

3. **Database Changes**:
   - Remember: NO `deleted_at` filtering
   - Use repository pattern for all DB access
   - Handle connection failures gracefully

4. **Testing Changes**:
   - Run unit tests first: `go test ./internal/...`
   - Check specific use case: `go test ./internal/application/usecases -v`
   - Verify no lint issues: `golangci-lint run`

## Deployment Prerequisites

Before deploying, ensure:

1. **Database Configuration** in `deployments/helm/values.yaml`:
   ```yaml
   database:
     host: "actual.db.host"
     port: "3306"
     user: "multinic_user"
     password: "actual_password"
     name: "multinic_database"
   ```

2. **SSH Access** for deployment script:
   ```bash
   export SSH_PASSWORD="actual_ssh_password"
   ```

3. **Namespace Creation**:
   ```bash
   kubectl create namespace multinic-system
   ```

## Recent Important Changes

1. **Removed `deleted_at` filtering** (2025-07-16): Agent now processes all records regardless of deleted_at status
2. **Added configuration drift detection**: Automatically updates when DB values differ from system
3. **Improved orphan detection**: MAC address-based comparison with DB for accurate cleanup
4. **RHEL support via ifcfg**: More reliable than nmcli in container environments

if [ "$GIT_AUTHOR_NAME" = "Meenzino" ]; then
    export GIT_AUTHOR_NAME="Minzino"
    export GIT_AUTHOR_EMAIL="gurumcider@gmail.com"
fi
if [ "$GIT_COMMITTER_NAME" = "Meenzino" ]; then
    export GIT_COMMITTER_NAME="Minzino"
    export GIT_COMMITTER_EMAIL="gurumcider@gmail.com"
fi
' HEAD~4..HEAD
```

#### 3. 변경사항 강제 Push
```bash
git push --force origin main
```

### 정리된 커밋들
1. **feat: RHEL/CentOS 지원 추가 및 고아 인터페이스 감지 로직 개선**
   - RHEL/CentOS ifcfg 파일 기반 네트워크 관리 지원
   - 호스트네임 도메인 접미사 제거 로직으로 일관성 개선
   - MAC 주소 기반 정확한 고아 인터페이스 감지

2. **docs: README에 RHEL/CentOS 지원 세부사항 추가**
   - OS별 지원 세부사항 섹션 추가
   - Ubuntu(Netplan)과 RHEL/CentOS(ifcfg) 방식 비교표 추가
   - 각 OS별 설정 파일 형식 및 예시 제공

3. **fix: 호스트네임 도메인 접미사 처리 불일치 해결**
   - 설정과 삭제 로직 간 호스트네임 처리 방식 통일
   - .novalocal 도메인 접미사 자동 제거

4. **debug: ifcfg 고아 파일 감지 로직에 상세 로그 추가**
   - 고아 감지 과정의 각 단계별 상세 로그
   - MAC 주소 비교 및 파일 스캔 과정 가시화

### 결과
- ✅ 모든 커밋 작성자가 "Minzino"로 통일
- ✅ 커밋 메시지에서 개발 도구 참조 완전 제거
- ✅ 프로젝트 기여자 정보 정확성 확보
- ✅ 깔끔한 프로젝트 히스토리 유지

### 향후 방침
- 모든 새로운 커밋에서 개발 도구 참조 배제
- 프로젝트 기여자 정보 정확성 유지
- 일관된 커밋 메시지 스타일 적용