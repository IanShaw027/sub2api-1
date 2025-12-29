package infrastructure

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/repository"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// InitDB 初始化数据库连接
func InitDB(cfg *config.Config) (*gorm.DB, error) {
	// 初始化时区（在数据库连接之前，确保时区设置正确）
	if err := timezone.Init(cfg.Timezone); err != nil {
		return nil, err
	}

	gormConfig := &gorm.Config{}
	if cfg.Server.Mode == "debug" {
		gormConfig.Logger = logger.Default.LogMode(logger.Info)
	}

	// 使用带时区的 DSN 连接数据库
	db, err := gorm.Open(postgres.Open(cfg.Database.DSNWithTimezone(cfg.Timezone)), gormConfig)
	if err != nil {
		return nil, err
	}

	// 配置数据库连接池参数
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)
	sqlDB.SetConnMaxIdleTime(10 * time.Minute)

	// 自动迁移（始终执行，确保数据库结构与代码同步）
	// GORM 的 AutoMigrate 只会添加新字段，不会删除或修改已有字段，是安全的
	if err := repository.AutoMigrate(db); err != nil {
		return nil, err
	}

	return db, nil
}
