//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBackupServiceGetSchedule_NilServiceOrRepoReturnsEmptyConfig(t *testing.T) {
	var nilService *BackupService

	cfg, err := nilService.GetSchedule(context.Background())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, &BackupScheduleConfig{}, cfg)

	serviceWithNilRepo := &BackupService{}
	cfg, err = serviceWithNilRepo.GetSchedule(context.Background())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, &BackupScheduleConfig{}, cfg)
}

func TestBackupServiceUpdateSchedule_NilServiceOrRepoReturnsRepoNotInitialized(t *testing.T) {
	var nilService *BackupService
	_, err := nilService.UpdateSchedule(context.Background(), BackupScheduleConfig{})
	require.EqualError(t, err, "setting repository not initialized")

	serviceWithNilRepo := &BackupService{}
	_, err = serviceWithNilRepo.UpdateSchedule(context.Background(), BackupScheduleConfig{})
	require.EqualError(t, err, "setting repository not initialized")
}

func TestBackupServiceUpdateS3Config_NilServiceOrRepoReturnsRepoNotInitialized(t *testing.T) {
	var nilService *BackupService
	_, err := nilService.UpdateS3Config(context.Background(), BackupS3Config{})
	require.EqualError(t, err, "setting repository not initialized")

	serviceWithNilRepo := &BackupService{}
	_, err = serviceWithNilRepo.UpdateS3Config(context.Background(), BackupS3Config{})
	require.EqualError(t, err, "setting repository not initialized")
}

func TestBackupServiceUpdateObjectStorageSettings_NilServiceOrRepoReturnsRepoNotInitialized(t *testing.T) {
	var nilService *BackupService
	_, err := nilService.UpdateObjectStorageSettings(context.Background(), ObjectStorageSettings{})
	require.EqualError(t, err, "setting repository not initialized")

	serviceWithNilRepo := &BackupService{}
	_, err = serviceWithNilRepo.UpdateObjectStorageSettings(context.Background(), ObjectStorageSettings{})
	require.EqualError(t, err, "setting repository not initialized")
}

func TestBackupServiceListBackups_NilServiceOrRepoReturnsEmpty(t *testing.T) {
	var nilService *BackupService

	records, err := nilService.ListBackups(context.Background())
	require.NoError(t, err)
	require.Empty(t, records)

	serviceWithNilRepo := &BackupService{}
	records, err = serviceWithNilRepo.ListBackups(context.Background())
	require.NoError(t, err)
	require.Empty(t, records)
}

func TestBackupServiceGetBackupRecord_NilServiceOrRepoReturnsNotFound(t *testing.T) {
	var nilService *BackupService

	record, err := nilService.GetBackupRecord(context.Background(), "missing")
	require.Nil(t, record)
	require.ErrorIs(t, err, ErrBackupNotFound)

	serviceWithNilRepo := &BackupService{}
	record, err = serviceWithNilRepo.GetBackupRecord(context.Background(), "missing")
	require.Nil(t, record)
	require.ErrorIs(t, err, ErrBackupNotFound)
}

func TestBackupServiceSaveRecordsLocked_NilServiceOrRepoReturnsRepoNotInitialized(t *testing.T) {
	var nilService *BackupService
	err := nilService.saveRecordsLocked(context.Background(), []BackupRecord{{ID: "a"}})
	require.EqualError(t, err, "setting repository not initialized")

	serviceWithNilRepo := &BackupService{}
	err = serviceWithNilRepo.saveRecordsLocked(context.Background(), []BackupRecord{{ID: "a"}})
	require.EqualError(t, err, "setting repository not initialized")
}

func TestBackupServiceSaveRecord_NilServiceOrRepoReturnsRepoNotInitialized(t *testing.T) {
	var nilService *BackupService
	err := nilService.saveRecord(context.Background(), &BackupRecord{ID: "a"})
	require.EqualError(t, err, "setting repository not initialized")

	serviceWithNilRepo := &BackupService{}
	err = serviceWithNilRepo.saveRecord(context.Background(), &BackupRecord{ID: "a"})
	require.EqualError(t, err, "setting repository not initialized")
}
