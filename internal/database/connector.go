package database

import (
	"fmt"
	"time"

	"github.com/redgreat/apiwong/internal/config"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Connector 数据库连接器
type Connector struct{}

// NewConnector 创建连接器
func NewConnector() *Connector {
	return &Connector{}
}

// Connect 根据配置创建数据库连接
func (c *Connector) Connect(cfg config.DatabaseConfig) (*gorm.DB, error) {
	var dialector gorm.Dialector
	var dsn string

	switch cfg.Type {
	case "mysql":
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
			cfg.Username, cfg.Password, cfg.Host, cfg.Port, cfg.Database, cfg.Charset)
		dialector = mysql.Open(dsn)

	case "postgres":
		dsn = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable TimeZone=Asia/Shanghai",
			cfg.Host, cfg.Port, cfg.Username, cfg.Password, cfg.Database)
		dialector = postgres.Open(dsn)

	case "sqlserver":
		dsn = fmt.Sprintf("sqlserver://%s:%s@%s:%d?database=%s",
			cfg.Username, cfg.Password, cfg.Host, cfg.Port, cfg.Database)
		dialector = sqlserver.Open(dsn)

	// Oracle 支持（需要安装 Oracle 客户端和驱动）
	// case "oracle":
	// 	dsn = fmt.Sprintf("oracle://%s:%s@%s:%d/%s",
	// 		cfg.Username, cfg.Password, cfg.Host, cfg.Port, cfg.Database)
	// 	dialector = oracle.Open(dsn)

	default:
		return nil, fmt.Errorf("不支持的数据库类型: %s", cfg.Type)
	}

	// GORM 配置
	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
		NowFunc: func() time.Time {
			return time.Now().Local()
		},
	})

	if err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	// 设置连接池
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取数据库实例失败: %w", err)
	}

	sqlDB.SetMaxIdleConns(cfg.MaxIdle)
	sqlDB.SetMaxOpenConns(cfg.MaxOpen)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return db, nil
}
