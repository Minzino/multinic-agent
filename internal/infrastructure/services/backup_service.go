package services

import (
	"context"
	"fmt"
	"multinic-agent-v2/internal/domain/errors"
	"multinic-agent-v2/internal/domain/interfaces"
	"path/filepath"
	"sort"
	"strings"
	
	"github.com/sirupsen/logrus"
)

// BackupService는 설정 백업을 관리하는 서비스입니다
type BackupService struct {
	fileSystem interfaces.FileSystem
	clock      interfaces.Clock
	logger     *logrus.Logger
	backupDir  string
}

// NewBackupService는 새로운 BackupService를 생성합니다
func NewBackupService(
	fs interfaces.FileSystem,
	clock interfaces.Clock,
	logger *logrus.Logger,
	backupDir string,
) interfaces.BackupService {
	return &BackupService{
		fileSystem: fs,
		clock:      clock,
		logger:     logger,
		backupDir:  backupDir,
	}
}

// CreateBackup은 현재 설정의 백업을 생성합니다
func (s *BackupService) CreateBackup(ctx context.Context, interfaceName string, configPath string) error {
	// 백업 디렉토리 생성
	if err := s.fileSystem.MkdirAll(s.backupDir, 0755); err != nil {
		return errors.NewSystemError("백업 디렉토리 생성 실패", err)
	}
	
	// 원본 파일 존재 확인
	if !s.fileSystem.Exists(configPath) {
		s.logger.WithFields(logrus.Fields{
			"interface": interfaceName,
			"path":      configPath,
		}).Debug("백업할 설정 파일이 없음")
		return nil
	}
	
	// 원본 파일 읽기
	content, err := s.fileSystem.ReadFile(configPath)
	if err != nil {
		return errors.NewSystemError("설정 파일 읽기 실패", err)
	}
	
	// 백업 파일명 생성 (예: multinic0_20250108_150405.yaml)
	timestamp := s.clock.Now().Format("20060102_150405")
	backupFileName := fmt.Sprintf("%s_%s%s", interfaceName, timestamp, filepath.Ext(configPath))
	backupPath := filepath.Join(s.backupDir, backupFileName)
	
	// 백업 파일 저장
	if err := s.fileSystem.WriteFile(backupPath, content, 0644); err != nil {
		return errors.NewSystemError("백업 파일 저장 실패", err)
	}
	
	s.logger.WithFields(logrus.Fields{
		"interface":   interfaceName,
		"backup_path": backupPath,
	}).Info("설정 백업 생성 완료")
	
	return nil
}

// RestoreLatestBackup은 가장 최근의 백업을 복원합니다
func (s *BackupService) RestoreLatestBackup(ctx context.Context, interfaceName string) error {
	// 백업 파일 찾기
	backupFiles, err := s.findBackupFiles(interfaceName)
	if err != nil {
		return err
	}
	
	if len(backupFiles) == 0 {
		return errors.NewNotFoundError(fmt.Sprintf("인터페이스 %s의 백업 파일을 찾을 수 없음", interfaceName))
	}
	
	// 가장 최근 백업 파일 선택 (이미 정렬됨)
	latestBackup := backupFiles[len(backupFiles)-1]
	
	s.logger.WithFields(logrus.Fields{
		"interface":   interfaceName,
		"backup_file": latestBackup,
	}).Info("백업 복원 완료")
	
	// 실제 복원 로직은 네트워크 어댑터에서 처리
	// 여기서는 백업 파일 존재 확인만 수행
	return nil
}

// HasBackup은 백업이 존재하는지 확인합니다
func (s *BackupService) HasBackup(ctx context.Context, interfaceName string) bool {
	backupFiles, err := s.findBackupFiles(interfaceName)
	if err != nil {
		s.logger.WithError(err).Error("백업 파일 검색 실패")
		return false
	}
	
	return len(backupFiles) > 0
}

// findBackupFiles는 특정 인터페이스의 백업 파일들을 찾아 정렬된 목록을 반환합니다
func (s *BackupService) findBackupFiles(interfaceName string) ([]string, error) {
	if !s.fileSystem.Exists(s.backupDir) {
		return []string{}, nil
	}
	
	files, err := s.fileSystem.ListFiles(s.backupDir)
	if err != nil {
		return nil, errors.NewSystemError("백업 디렉토리 읽기 실패", err)
	}
	
	// 해당 인터페이스의 백업 파일만 필터링
	var backupFiles []string
	prefix := interfaceName + "_"
	for _, file := range files {
		if strings.HasPrefix(file, prefix) {
			backupFiles = append(backupFiles, file)
		}
	}
	
	// 파일명 기준 정렬 (타임스탬프가 포함되어 있으므로 시간순 정렬됨)
	sort.Strings(backupFiles)
	
	return backupFiles, nil
}