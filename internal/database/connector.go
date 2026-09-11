package database

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/redgreat/mergewong/internal/config"
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
		// interpolateParams=true 让驱动走文本协议（客户端插值）而非预处理语句（二进制协议）。
		// AnalyticDB(ADB) 对 JSON 列在二进制协议的批量 INSERT ... ON DUPLICATE KEY UPDATE 存在缺陷，
		// 会导致 JSON 值之后的列整体错位（如 smallint 列收到字符串）。改为文本协议后，
		// JSON 值以转义后的字符串字面量内联，可正确写入 JSON 列。
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local&interpolateParams=true",
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
	gormLogger := logger.New(log.New(os.Stdout, "\r\n", log.LstdFlags), logger.Config{
		SlowThreshold:             time.Second,
		LogLevel:                  logger.Warn,
		IgnoreRecordNotFoundError: true,
		Colorful:                  false,
	})
	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: gormLogger,
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
