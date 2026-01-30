package services

import (
	"fmt"

	"github.com/redgreat/apiwong/internal/database"
)


// DBService 数据库服务
type DBService struct{}

// NewDBService 创建数据库服务
func NewDBService() *DBService {
	return &DBService{}
}

// QueryData 执行查询
func (s *DBService) QueryData(dbName, sql string, params []interface{}, page, pageSize int) ([]map[string]interface{}, int64, error) {
	db, err := database.GetManager().GetConnection(dbName)
	if err != nil {
		return nil, 0, err
	}

	// 获取总数（去掉分页参数）
	var total int64
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM (%s) AS count_table", sql)
	if err := db.Raw(countSQL, params...).Count(&total).Error; err != nil {
		total = 0 // 如果计数失败，设为 0
	}

	// 添加分页
	if page > 0 && pageSize > 0 {
		offset := (page - 1) * pageSize
		sql = fmt.Sprintf("%s LIMIT %d OFFSET %d", sql, pageSize, offset)
	}

	// 执行查询
	var results []map[string]interface{}
	rows, err := db.Raw(sql, params...).Rows()
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	// 获取列名
	columns, err := rows.Columns()
	if err != nil {
		return nil, 0, err
	}

	// 读取数据
	for rows.Next() {
		// 创建接收数据的切片
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		// 扫描数据
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, 0, err
		}

		// 转换为 map
		result := make(map[string]interface{})
		for i, col := range columns {
			var v interface{}
			val := values[i]
			b, ok := val.([]byte)
			if ok {
				v = string(b)
			} else {
				v = val
			}
			result[col] = v
		}
		results = append(results, result)
	}

	return results, total, nil
}

// ExecSQL 执行 SQL（INSERT/UPDATE/DELETE）
func (s *DBService) ExecSQL(dbName, sql string, params []interface{}) (int64, error) {
	db, err := database.GetManager().GetConnection(dbName)
	if err != nil {
		return 0, err
	}

	result := db.Exec(sql, params...)
	if result.Error != nil {
		return 0, result.Error
	}

	return result.RowsAffected, nil
}

// ListTables 列出所有表
func (s *DBService) ListTables(dbName string) ([]string, error) {
	db, err := database.GetManager().GetConnection(dbName)
	if err != nil {
		return nil, err
	}

	var tables []string
	// 使用 GORM 的 Migrator 获取表列表
	if err := db.Raw("SHOW TABLES").Scan(&tables).Error; err != nil {
		return nil, err
	}

	return tables, nil
}

// GetTableSchema 获取表结构
func (s *DBService) GetTableSchema(dbName, tableName string) ([]map[string]interface{}, error) {
	db, err := database.GetManager().GetConnection(dbName)
	if err != nil {
		return nil, err
	}

	var schema []map[string]interface{}
	sql := fmt.Sprintf("DESCRIBE %s", tableName)
	
	rows, err := db.Raw(sql).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// 获取列名
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	// 读取数据
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		result := make(map[string]interface{})
		for i, col := range columns {
			var v interface{}
			val := values[i]
			b, ok := val.([]byte)
			if ok {
				v = string(b)
			} else {
				v = val
			}
			result[col] = v
		}
		schema = append(schema, result)
	}

	return schema, nil
}

// InsertData 插入数据
func (s *DBService) InsertData(dbName, tableName string, data map[string]interface{}) error {
	db, err := database.GetManager().GetConnection(dbName)
	if err != nil {
		return err
	}

	return db.Table(tableName).Create(data).Error
}

// UpdateData 更新数据
func (s *DBService) UpdateData(dbName, tableName, idField string, id interface{}, data map[string]interface{}) error {
	db, err := database.GetManager().GetConnection(dbName)
	if err != nil {
		return err
	}

	return db.Table(tableName).Where(fmt.Sprintf("%s = ?", idField), id).Updates(data).Error
}

// DeleteData 删除数据
func (s *DBService) DeleteData(dbName, tableName, idField string, id interface{}) error {
	db, err := database.GetManager().GetConnection(dbName)
	if err != nil {
		return err
	}

	return db.Table(tableName).Where(fmt.Sprintf("%s = ?", idField), id).Delete(nil).Error
}
