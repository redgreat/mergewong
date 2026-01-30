package config

import (
	"fmt"
	"log"

	"github.com/spf13/viper"
)

// Config 全局配置
type Config struct {
	Server    ServerConfig              `mapstructure:"server"`
	JWT       JWTConfig                 `mapstructure:"jwt"`
	Databases map[string]DatabaseConfig `mapstructure:"databases"`
	Log       LogConfig                 `mapstructure:"log"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port string `mapstructure:"port"`
	Mode string `mapstructure:"mode"` // debug, release, test
}

// JWTConfig JWT 配置
type JWTConfig struct {
	Secret     string `mapstructure:"secret"`
	ExpireTime int    `mapstructure:"expire_time"` // 小时
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Type     string `mapstructure:"type"` // mysql, postgres, sqlserver, oracle
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Database string `mapstructure:"database"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Charset  string `mapstructure:"charset"`
	MaxIdle  int    `mapstructure:"max_idle"`
	MaxOpen  int    `mapstructure:"max_open"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level      string `mapstructure:"level"` // debug, info, warn, error
	OutputPath string `mapstructure:"output_path"`
}

var AppConfig *Config

// LoadConfig 加载配置文件
func LoadConfig(configPath string) error {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	// 允许从环境变量读取配置
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	AppConfig = &Config{}
	if err := viper.Unmarshal(AppConfig); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	log.Printf("配置文件加载成功: %s", configPath)
	return nil
}

// GetDatabaseConfig 获取指定名称的数据库配置
func GetDatabaseConfig(name string) (DatabaseConfig, bool) {
	if AppConfig == nil {
		return DatabaseConfig{}, false
	}
	config, exists := AppConfig.Databases[name]
	return config, exists
}
