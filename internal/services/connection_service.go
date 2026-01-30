package services

import (
	"fmt"

	"github.com/redgreat/apiwong/internal/config"
	"github.com/redgreat/apiwong/internal/database"
	"github.com/redgreat/apiwong/internal/models"
	"gorm.io/gorm"
)

type ConnectionService struct {
	systemDB *gorm.DB
}

func NewConnectionService() *ConnectionService {
	db, _ := database.GetManager().GetConnection("system")
	return &ConnectionService{systemDB: db}
}

func (s *ConnectionService) ListConnections(page, pageSize int) ([]models.DatabaseConnection, int64, error) {
	var connections []models.DatabaseConnection
	var total int64

	s.systemDB.Model(&models.DatabaseConnection{}).Count(&total)

	offset := (page - 1) * pageSize
	if err := s.systemDB.Order("id DESC").Offset(offset).Limit(pageSize).Find(&connections).Error; err != nil {
		return nil, 0, err
	}

	return connections, total, nil
}

func (s *ConnectionService) GetConnection(id uint) (*models.DatabaseConnection, error) {
	var connection models.DatabaseConnection
	if err := s.systemDB.First(&connection, id).Error; err != nil {
		return nil, err
	}
	return &connection, nil
}

func (s *ConnectionService) CreateConnection(connection *models.DatabaseConnection) error {
	if err := s.systemDB.Create(connection).Error; err != nil {
		return err
	}

	if connection.Status == 1 {
		if err := s.addToManager(connection); err != nil {
			s.systemDB.Delete(&models.DatabaseConnection{}, connection.ID)
			return err
		}
	}

	return nil
}

func (s *ConnectionService) UpdateConnection(id uint, updates map[string]interface{}) error {
	return s.systemDB.Model(&models.DatabaseConnection{}).Where("id = ?", id).Updates(updates).Error
}

func (s *ConnectionService) DeleteConnection(id uint) error {
	connection, err := s.GetConnection(id)
	if err != nil {
		return err
	}

	if connection.Status == 1 {
		_ = s.removeFromManager(connection.Name)
	}

	return s.systemDB.Delete(&models.DatabaseConnection{}, id).Error
}

func (s *ConnectionService) LoadEnabledConnections() error {
	var connections []models.DatabaseConnection
	if err := s.systemDB.Where("status = ?", 1).Find(&connections).Error; err != nil {
		return err
	}

	for _, connection := range connections {
		if _, err := database.GetManager().GetConnection(connection.Name); err == nil {
			continue
		}
		if err := s.addToManager(&connection); err != nil {
			return err
		}
	}

	return nil
}

func (s *ConnectionService) TestConnection(connection *models.DatabaseConnection) error {
	cfg := s.toConfig(connection)
	connector := database.NewConnector()
	db, err := connector.Connect(cfg)
	if err != nil {
		return err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("获取数据库实例失败: %w", err)
	}
	defer sqlDB.Close()

	return sqlDB.Ping()
}

func (s *ConnectionService) RefreshManager(oldConnection, newConnection *models.DatabaseConnection) error {
	if oldConnection != nil && oldConnection.Status == 1 {
		_ = s.removeFromManager(oldConnection.Name)
	}

	if newConnection != nil && newConnection.Status == 1 {
		return s.addToManager(newConnection)
	}

	return nil
}

func (s *ConnectionService) addToManager(connection *models.DatabaseConnection) error {
	cfg := s.toConfig(connection)
	return database.GetManager().AddConnection(connection.Name, cfg)
}

func (s *ConnectionService) removeFromManager(name string) error {
	return database.GetManager().RemoveConnection(name)
}

func (s *ConnectionService) toConfig(connection *models.DatabaseConnection) config.DatabaseConfig {
	return config.DatabaseConfig{
		Type:     connection.Type,
		Host:     connection.Host,
		Port:     connection.Port,
		Database: connection.Database,
		Username: connection.Username,
		Password: connection.Password,
		Charset:  connection.Charset,
		MaxIdle:  connection.MaxIdle,
		MaxOpen:  connection.MaxOpen,
	}
}
