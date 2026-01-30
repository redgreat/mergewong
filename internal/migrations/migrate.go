package migrations

import (
	"fmt"
	"log"

	"github.com/redgreat/apiwong/internal/models"
	"github.com/redgreat/apiwong/internal/utils"
	"gorm.io/gorm"
)

// Migrator 数据库迁移器
type Migrator struct{}

// Run 执行所有迁移任务
func (m *Migrator) Run(db *gorm.DB) error {
	log.Println("========================================")
	log.Println("开始数据库初始化和迁移")
	log.Println("========================================")

	// 1. 检查数据库连接
	if err := m.checkConnection(db); err != nil {
		return fmt.Errorf("数据库连接检查失败: %w", err)
	}

	// 2. 自动迁移表结构
	if err := m.migrateSchema(db); err != nil {
		return fmt.Errorf("表结构迁移失败: %w", err)
	}

	// 3. 初始化基础数据
	if err := m.initializeData(db); err != nil {
		return fmt.Errorf("基础数据初始化失败: %w", err)
	}

	log.Println("========================================")
	log.Println("数据库初始化和迁移完成")
	log.Println("========================================")
	return nil
}

// checkConnection 检查数据库连接
func (m *Migrator) checkConnection(db *gorm.DB) error {
	log.Println("[1/3] 检查数据库连接...")

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}

	if err := sqlDB.Ping(); err != nil {
		return err
	}

	// 获取数据库统计信息
	stats := sqlDB.Stats()
	log.Printf("  ✓ 数据库连接正常")
	log.Printf("  ✓ 最大打开连接: %d", stats.MaxOpenConnections)
	log.Printf("  ✓ 当前打开连接: %d", stats.OpenConnections)
	log.Printf("  ✓ 空闲连接: %d", stats.Idle)

	return nil
}

// migrateSchema 迁移表结构
func (m *Migrator) migrateSchema(db *gorm.DB) error {
	log.Println("[2/3] 迁移数据库表结构...")

	// 定义所有需要迁移的模型
	models := []interface{}{
		&models.User{},
		&models.DatabaseConnection{},
		&models.SyncTask{},
		&models.SyncLog{},
	}

	// 执行自动迁移
	for _, model := range models {
		modelName := fmt.Sprintf("%T", model)
		log.Printf("  - 正在迁移: %s", modelName)

		if err := db.AutoMigrate(model); err != nil {
			return fmt.Errorf("迁移 %s 失败: %w", modelName, err)
		}
	}

	log.Println("  ✓ 所有表结构迁移完成")
	return nil
}

// initializeData 初始化基础数据
func (m *Migrator) initializeData(db *gorm.DB) error {
	log.Println("[3/3] 初始化基础数据...")

	// 初始化管理员账户
	if err := m.initAdminUser(db); err != nil {
		return err
	}

	log.Println("  ✓ 基础数据初始化完成")
	return nil
}

// initAdminUser 初始化管理员账户
func (m *Migrator) initAdminUser(db *gorm.DB) error {
	// 检查是否已有管理员账户
	var count int64
	if err := db.Model(&models.User{}).Where("role = ?", "admin").Count(&count).Error; err != nil {
		return err
	}

	if count > 0 {
		log.Printf("  - 管理员账户已存在 (%d 个)，跳过创建", count)
		return nil
	}

	log.Println("  - 创建默认管理员账户...")

	// 创建默认管理员
	// 用户名: admin
	// 密码: admin123
	hashedPassword, err := utils.HashPassword("admin123")
	if err != nil {
		return fmt.Errorf("密码加密失败: %w", err)
	}

	admin := models.User{
		Username: "admin",
		Password: hashedPassword,
		Email:    "admin@apiwong.com",
		Role:     "admin",
		Status:   1,
	}

	if err := db.Create(&admin).Error; err != nil {
		return fmt.Errorf("创建管理员账户失败: %w", err)
	}

	log.Println("  ✓ 默认管理员账户创建成功")
	log.Println("    用户名: admin")
	log.Println("    密码: admin123")
	log.Println("    ⚠️  请在首次登录后立即修改密码！")

	return nil
}
