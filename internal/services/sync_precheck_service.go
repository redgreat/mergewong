package services

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/redgreat/mergewong/internal/database"
	"github.com/redgreat/mergewong/internal/models"
	"gorm.io/gorm"
)

type PrecheckItem struct {
	Level   string `json:"level"`
	Object  string `json:"object"`
	Message string `json:"message"`
}

type PrecheckResult struct {
	Passed bool           `json:"passed"`
	Items  []PrecheckItem `json:"items"`
}

type mysqlColumn struct {
	Field string `gorm:"column:Field"`
	Type  string `gorm:"column:Type"`
	Key   string `gorm:"column:Key"`
}

func (s *SyncService) PrecheckTask(taskID uint) (*PrecheckResult, error) {
	task, err := s.GetTask(taskID)
	if err != nil {
		return nil, err
	}
	result := &PrecheckResult{Passed: true}
	add := func(level, object, message string) {
		result.Items = append(result.Items, PrecheckItem{Level: level, Object: object, Message: message})
		if level == "error" {
			result.Passed = false
		}
	}

	var sourceConn, targetConn models.DatabaseConnection
	if err := s.systemDB.Where("name = ?", task.SourceDB).First(&sourceConn).Error; err != nil {
		return nil, err
	}
	if err := s.systemDB.Where("name = ?", task.TargetDB).First(&targetConn).Error; err != nil {
		return nil, err
	}
	if sourceConn.Type != "mysql" || targetConn.Type != "mysql" {
		add("error", "连接类型", "可靠多表同步当前仅支持 MySQL → MySQL")
		return result, nil
	}
	if task.ScheduleType != "manual" {
		add("error", "执行方式", "全量初始化和 Binlog CDC 均由任务自身管理，不需要 Cron 或轮询间隔")
	}
	sourceDB, err := database.GetManager().GetConnection(task.SourceDB)
	if err != nil {
		add("error", "源连接", err.Error())
		return result, nil
	}
	targetDB, err := database.GetManager().GetConnection(task.TargetDB)
	if err != nil {
		add("error", "目标连接", err.Error())
		return result, nil
	}
	if sqlDB, err := sourceDB.DB(); err != nil || sqlDB.Ping() != nil {
		add("error", "源连接", "源数据库不可连接")
	} else {
		add("success", "源连接", "连接正常")
	}
	if sqlDB, err := targetDB.DB(); err != nil || sqlDB.Ping() != nil {
		add("error", "目标连接", "目标数据库不可连接")
	} else {
		add("success", "目标连接", "连接正常")
	}
	grants, grantErr := mysqlCurrentGrants(targetDB)
	if grantErr != nil {
		add("warning", "目标权限", "无法读取账号授权信息: "+grantErr.Error())
	}
	upperGrants := strings.ToUpper(grants)
	if task.SyncType == "cdc" || task.SyncType == "full_cdc" {
		var variable struct {
			VariableName string `gorm:"column:Variable_name"`
			Value        string `gorm:"column:Value"`
		}
		if err := sourceDB.Raw("SHOW VARIABLES LIKE 'log_bin'").Scan(&variable).Error; err != nil || strings.ToUpper(variable.Value) != "ON" {
			add("error", "Binlog", "源库必须开启 log_bin")
		} else {
			add("success", "Binlog", "log_bin 已开启")
		}
		variable = struct {
			VariableName string `gorm:"column:Variable_name"`
			Value        string `gorm:"column:Value"`
		}{}
		_ = sourceDB.Raw("SHOW VARIABLES LIKE 'binlog_format'").Scan(&variable).Error
		if strings.ToUpper(variable.Value) != "ROW" {
			add("error", "Binlog 格式", "binlog_format 必须为 ROW")
		} else {
			add("success", "Binlog 格式", "ROW 模式")
		}
		variable = struct {
			VariableName string `gorm:"column:Variable_name"`
			Value        string `gorm:"column:Value"`
		}{}
		_ = sourceDB.Raw("SHOW VARIABLES LIKE 'binlog_row_image'").Scan(&variable).Error
		if strings.ToUpper(variable.Value) != "FULL" {
			add("error", "行镜像", "binlog_row_image 必须为 FULL，才能可靠处理更新和删除")
		} else {
			add("success", "行镜像", "FULL 模式")
		}
		sourceGrants, err := mysqlCurrentGrants(sourceDB)
		upper := strings.ToUpper(sourceGrants)
		if err != nil || (!strings.Contains(upper, "ALL PRIVILEGES") && (!strings.Contains(upper, "REPLICATION SLAVE") || !strings.Contains(upper, "REPLICATION CLIENT"))) {
			add("error", "源端权限", "CDC 账号需要 REPLICATION SLAVE 和 REPLICATION CLIENT 权限")
		}
		if _, _, err := currentMySQLPosition(task.SourceDB); err != nil {
			add("error", "Binlog 位点", err.Error())
		} else {
			add("success", "Binlog 位点", "可读取当前位点")
		}
	}
	if grantErr == nil && !strings.Contains(upperGrants, "ALL PRIVILEGES") && !strings.Contains(upperGrants, "INSERT") {
		add("error", "目标权限", "目标账号缺少 INSERT 权限")
	}

	needsCreate := false
	if len(task.TaskTables) == 0 {
		task.TaskTables = []models.SyncTaskTable{{TaskID: task.ID, SourceTable: task.SourceTable, TargetTable: task.TargetTable, IncrementalKey: task.IncrementalKey, FieldMapping: task.FieldMapping}}
	}
	for i := range task.TaskTables {
		mapping := &task.TaskTables[i]
		object := mapping.SourceTable + " → " + mapping.TargetTable
		if !validMySQLIdentifier(mapping.SourceTable) || !validMySQLIdentifier(mapping.TargetTable) {
			add("error", object, "表名只能包含字母、数字、下划线和 $，且不能以数字开头")
			continue
		}
		sourceColumns, err := describeMySQLTable(sourceDB, mapping.SourceTable)
		if err != nil {
			add("error", object, "读取源表结构失败: "+err.Error())
			continue
		}
		permissionRows, permissionErr := sourceDB.Raw("SELECT * FROM `" + mapping.SourceTable + "` LIMIT 0").Rows()
		if permissionErr != nil {
			add("error", object, "源账号缺少读取权限: "+permissionErr.Error())
			continue
		}
		permissionRows.Close()
		pk := primaryKeyOf(sourceColumns)
		if pk == "" {
			add("error", object, "源表没有单列主键，无法稳定分页和幂等写入")
			continue
		}
		mapping.SourcePrimaryKey, mapping.TargetPrimaryKey = pk, mappedColumn(mapping.FieldMapping, pk)
		if targetDB.Migrator().HasTable(mapping.TargetTable) {
			targetColumns, err := describeMySQLTable(targetDB, mapping.TargetTable)
			if err != nil {
				add("error", object, "读取目标表结构失败: "+err.Error())
				continue
			}
			if primaryKeyOf(targetColumns) != mapping.TargetPrimaryKey {
				add("error", object, "目标表必须使用单列主键: "+mapping.TargetPrimaryKey)
				continue
			}
			for _, column := range sourceColumns {
				targetName := mappedColumn(mapping.FieldMapping, column.Field)
				targetColumn, ok := findColumn(targetColumns, targetName)
				if !ok {
					add("error", object, "目标表缺少字段: "+targetName)
					continue
				}
				if mysqlBaseType(column.Type) != mysqlBaseType(targetColumn.Type) {
					add("error", object, fmt.Sprintf("字段类型不兼容: %s(%s) → %s(%s)", column.Field, column.Type, targetName, targetColumn.Type))
				}
			}
			add("success", object, "源表、目标表和主键检查通过")
		} else {
			needsCreate = true
			if len(mapping.FieldMapping) > 0 {
				add("error", object, "目标表不存在时不能使用字段改名，请先创建目标表")
			} else {
				add("warning", object, "目标表不存在，首次执行时将按源表结构创建")
			}
		}
	}
	if needsCreate && grantErr == nil && !strings.Contains(upperGrants, "ALL PRIVILEGES") && !strings.Contains(upperGrants, "CREATE") {
		add("error", "目标权限", "存在待创建目标表，但目标账号缺少 CREATE 权限")
	}

	if result.Passed {
		err = s.systemDB.Transaction(func(tx *gorm.DB) error {
			for i := range task.TaskTables {
				m := &task.TaskTables[i]
				if m.ID == 0 {
					m.TaskID, m.Position = task.ID, i
					if err := tx.Create(m).Error; err != nil {
						return err
					}
				} else if err := tx.Model(&models.SyncTaskTable{}).Where("id = ?", m.ID).Updates(map[string]interface{}{"source_primary_key": m.SourcePrimaryKey, "target_primary_key": m.TargetPrimaryKey}).Error; err != nil {
					return err
				}
			}
			return tx.Model(&models.SyncTask{}).Where("id = ?", task.ID).Updates(map[string]interface{}{"validation_status": "passed", "status": 1, "runtime_status": "stopped"}).Error
		})
		if err != nil {
			return nil, err
		}
	} else {
		_ = s.systemDB.Model(&models.SyncTask{}).Where("id = ?", task.ID).Updates(map[string]interface{}{"validation_status": "failed", "status": 0}).Error
	}
	return result, nil
}

var mysqlIdentifierPattern = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)

func validMySQLIdentifier(value string) bool { return mysqlIdentifierPattern.MatchString(value) }

func describeMySQLTable(db *gorm.DB, table string) ([]mysqlColumn, error) {
	if !taskIdentifierPattern.MatchString(table) {
		return nil, fmt.Errorf("非法表名")
	}
	var columns []mysqlColumn
	if err := db.Raw("DESCRIBE `" + table + "`").Scan(&columns).Error; err != nil {
		return nil, err
	}
	if len(columns) == 0 {
		return nil, fmt.Errorf("表不存在")
	}
	return columns, nil
}

func primaryKeyOf(columns []mysqlColumn) string {
	keys := []string{}
	for _, column := range columns {
		if strings.EqualFold(column.Key, "PRI") {
			keys = append(keys, column.Field)
		}
	}
	if len(keys) == 1 {
		return keys[0]
	}
	return ""
}

func hasColumn(columns []mysqlColumn, name string) bool {
	for _, column := range columns {
		if column.Field == name {
			return true
		}
	}
	return false
}

func findColumn(columns []mysqlColumn, name string) (mysqlColumn, bool) {
	for _, column := range columns {
		if column.Field == name {
			return column, true
		}
	}
	return mysqlColumn{}, false
}

func mysqlBaseType(value string) string {
	value = strings.ToLower(value)
	if index := strings.Index(value, "("); index >= 0 {
		value = value[:index]
	}
	return strings.TrimSpace(strings.TrimSuffix(value, " unsigned"))
}

func mappedColumn(mapping models.FieldMapping, source string) string {
	if target, ok := mapping[source]; ok && target != "" {
		return target
	}
	return source
}

func mysqlCurrentGrants(db *gorm.DB) (string, error) {
	rows, err := db.Raw("SHOW GRANTS FOR CURRENT_USER").Rows()
	if err != nil {
		return "", err
	}
	defer rows.Close()
	grants := []string{}
	for rows.Next() {
		var grant string
		if err := rows.Scan(&grant); err != nil {
			return "", err
		}
		grants = append(grants, grant)
	}
	return strings.Join(grants, "\n"), rows.Err()
}
