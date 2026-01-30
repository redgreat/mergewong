package database

import (
	"fmt"
	"sync"

	"github.com/redgreat/apiwong/internal/config"
	"gorm.io/gorm"
)

// Manager 数据库连接管理器
type Manager struct {
	connections map[string]*gorm.DB
	connector   *Connector
	mu          sync.RWMutex
}

var (
	instance *Manager
	once     sync.Once
)

// GetManager 获取数据库管理器单例
func GetManager() *Manager {
	once.Do(func() {
		instance = &Manager{
			connections: make(map[string]*gorm.DB),
			connector:   NewConnector(),
		}
	})
	return instance
}

// AddConnection 添加数据库连接
func (m *Manager) AddConnection(name string, cfg config.DatabaseConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查连接是否已存在
	if _, exists := m.connections[name]; exists {
		return fmt.Errorf("数据库连接 '%s' 已存在", name)
	}

	// 创建连接
	db, err := m.connector.Connect(cfg)
	if err != nil {
		return err
	}

	m.connections[name] = db
	return nil
}

// GetConnection 获取数据库连接
func (m *Manager) GetConnection(name string) (*gorm.DB, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	db, exists := m.connections[name]
	if !exists {
		return nil, fmt.Errorf("数据库连接 '%s' 不存在", name)
	}

	return db, nil
}

// RemoveConnection 移除数据库连接
func (m *Manager) RemoveConnection(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	db, exists := m.connections[name]
	if !exists {
		return fmt.Errorf("数据库连接 '%s' 不存在", name)
	}

	// 关闭连接
	sqlDB, err := db.DB()
	if err == nil {
		sqlDB.Close()
	}

	delete(m.connections, name)
	return nil
}

// ListConnections 列出所有连接名称
func (m *Manager) ListConnections() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.connections))
	for name := range m.connections {
		names = append(names, name)
	}
	return names
}

// HealthCheck 检查连接健康状态
func (m *Manager) HealthCheck(name string) error {
	db, err := m.GetConnection(name)
	if err != nil {
		return err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}

	return sqlDB.Ping()
}

// Close 关闭所有连接
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, db := range m.connections {
		sqlDB, err := db.DB()
		if err == nil {
			sqlDB.Close()
		}
		delete(m.connections, name)
	}
}
