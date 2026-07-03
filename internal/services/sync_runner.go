package services

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/redgreat/mergewong/internal/database"
	"github.com/redgreat/mergewong/internal/models"
	"gorm.io/gorm"
)

const defaultSyncBatchSize = 500

var taskRunLocks sync.Map
var ErrTaskPaused = errors.New("任务已暂停")

func acquireTaskRunLock(taskID uint) (func(), error) {
	value, _ := taskRunLocks.LoadOrStore(taskID, &sync.Mutex{})
	lock := value.(*sync.Mutex)
	if !lock.TryLock() {
		return nil, fmt.Errorf("同一任务正在执行，不能重叠运行")
	}
	return lock.Unlock, nil
}

func (s *SyncService) syncValidatedTask(task *models.SyncTask) (int64, error) {
	sourceDB, err := database.GetManager().GetConnection(task.SourceDB)
	if err != nil {
		return 0, err
	}
	targetDB, err := database.GetManager().GetConnection(task.TargetDB)
	if err != nil {
		return 0, err
	}
	var total int64
	for i := range task.TaskTables {
		rows, err := s.syncValidatedTable(task, &task.TaskTables[i], sourceDB, targetDB)
		if err != nil {
			return total, fmt.Errorf("表 %s 同步失败: %w", task.TaskTables[i].SourceTable, err)
		}
		total += rows
	}
	return total, nil
}

func (s *SyncService) syncValidatedTable(task *models.SyncTask, mapping *models.SyncTaskTable, sourceDB, targetDB *gorm.DB) (int64, error) {
	if mapping.SourcePrimaryKey == "" || mapping.TargetPrimaryKey == "" {
		return 0, fmt.Errorf("缺少预检查主键信息")
	}
	if !targetDB.Migrator().HasTable(mapping.TargetTable) {
		if len(mapping.FieldMapping) > 0 {
			return 0, fmt.Errorf("目标表不存在时暂不支持字段改名")
		}
		if err := createMySQLTableLike(sourceDB, targetDB, mapping.SourceTable, mapping.TargetTable); err != nil {
			return 0, err
		}
	}
	var checkpoint models.SyncCheckpoint
	err := s.systemDB.Where("task_table_id = ?", mapping.ID).First(&checkpoint).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return 0, err
	}
	if checkpoint.Completed {
		return 0, nil
	}
	checkpoint.TaskTableID = mapping.ID
	var sourceTotal int64
	if err := sourceDB.Table(mapping.SourceTable).Count(&sourceTotal).Error; err != nil {
		return 0, err
	}
	processed := mapping.SnapshotProcessed
	_ = s.systemDB.Model(mapping).Updates(map[string]interface{}{"sync_state": "initializing", "snapshot_total": sourceTotal, "progress_message": "正在全量初始化"}).Error
	var total int64
	for {
		var runtime struct{ RuntimeStatus string }
		if err := s.systemDB.Model(&models.SyncTask{}).Select("runtime_status").Where("id = ?", task.ID).Scan(&runtime).Error; err != nil {
			return total, err
		}
		if runtime.RuntimeStatus == "paused" {
			return total, ErrTaskPaused
		}
		batch, columns, lastCursor, lastPK, err := readMySQLBatch(task, mapping, sourceDB, &checkpoint)
		if err != nil {
			return total, err
		}
		if len(batch) == 0 {
			checkpoint.Completed = true
			if err := saveCheckpoint(s.systemDB, &checkpoint); err != nil {
				return total, err
			}
			if err := s.systemDB.Model(mapping).Updates(map[string]interface{}{"sync_state": "snapshot_completed", "snapshot_processed": sourceTotal, "progress_percent": 100, "progress_message": "全量初始化完成"}).Error; err != nil {
				return total, err
			}
			if current, loadErr := s.GetTask(task.ID); loadErr == nil {
				s.RecordTaskEvent(current, "table_snapshot_completed", "snapshot", "success", "表全量初始化完成", fmt.Sprintf("%s：%d 行", mapping.SourceTable, sourceTotal), sourceTotal, 0)
			}
			return total, nil
		}
		if err := writeMySQLBatch(targetDB, mapping, columns, batch); err != nil {
			return total, err
		}
		checkpoint.CursorValue, checkpoint.CursorPrimaryKey = lastCursor, lastPK
		if err := saveCheckpoint(s.systemDB, &checkpoint); err != nil {
			return total, err
		}
		total += int64(len(batch))
		processed += int64(len(batch))
		percent := float64(100)
		if sourceTotal > 0 {
			percent = float64(processed) * 100 / float64(sourceTotal)
			if percent > 100 {
				percent = 100
			}
		}
		if err := s.systemDB.Model(mapping).Updates(map[string]interface{}{"snapshot_processed": processed, "snapshot_total": sourceTotal, "progress_percent": percent, "progress_message": fmt.Sprintf("已初始化 %d / %d 行", processed, sourceTotal)}).Error; err != nil {
			return total, err
		}
	}
}

func readMySQLBatch(task *models.SyncTask, mapping *models.SyncTaskTable, db *gorm.DB, checkpoint *models.SyncCheckpoint) ([]map[string]interface{}, []string, string, string, error) {
	pk := quoteMySQL(mapping.SourcePrimaryKey)
	table := quoteMySQL(mapping.SourceTable)
	query := "SELECT * FROM " + table
	params := []interface{}{}
	if task.SyncType == "incremental" {
		cursor := quoteMySQL(mapping.IncrementalKey)
		if checkpoint.CursorValue != "" || checkpoint.CursorPrimaryKey != "" {
			query += " WHERE (" + cursor + " > ?) OR (" + cursor + " = ? AND " + pk + " > ?)"
			params = append(params, checkpoint.CursorValue, checkpoint.CursorValue, checkpoint.CursorPrimaryKey)
		}
		query += " ORDER BY " + cursor + ", " + pk
	} else {
		if checkpoint.CursorPrimaryKey != "" {
			query += " WHERE " + pk + " > ?"
			params = append(params, checkpoint.CursorPrimaryKey)
		}
		query += " ORDER BY " + pk
	}
	query += fmt.Sprintf(" LIMIT %d", defaultSyncBatchSize)
	rows, err := db.Raw(query, params...).Rows()
	if err != nil {
		return nil, nil, "", "", err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return nil, nil, "", "", err
	}
	batch := []map[string]interface{}{}
	lastCursor, lastPK := "", ""
	for rows.Next() {
		values, pointers := make([]interface{}, len(columns)), make([]interface{}, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, nil, "", "", err
		}
		row := map[string]interface{}{}
		for i, column := range columns {
			row[column] = values[i]
			if column == mapping.SourcePrimaryKey {
				lastPK = valueString(values[i])
			}
			if task.SyncType == "incremental" && column == mapping.IncrementalKey {
				lastCursor = valueString(values[i])
			}
		}
		batch = append(batch, row)
	}
	return batch, columns, lastCursor, lastPK, rows.Err()
}

func writeMySQLBatch(db *gorm.DB, mapping *models.SyncTaskTable, sourceColumns []string, batch []map[string]interface{}) error {
	return db.Transaction(func(tx *gorm.DB) error { return writeMySQLBatchTx(tx, mapping, sourceColumns, batch) })
}

func writeMySQLBatchTx(db *gorm.DB, mapping *models.SyncTaskTable, sourceColumns []string, batch []map[string]interface{}) error {
	targetColumns := make([]string, len(sourceColumns))
	for i, column := range sourceColumns {
		targetColumns[i] = mappedColumn(mapping.FieldMapping, column)
	}
	quoted := make([]string, len(targetColumns))
	for i, column := range targetColumns {
		quoted[i] = quoteMySQL(column)
	}
	rowPlaceholder := "(" + strings.TrimSuffix(strings.Repeat("?,", len(targetColumns)), ",") + ")"
	placeholders, args := make([]string, 0, len(batch)), make([]interface{}, 0, len(batch)*len(targetColumns))
	for _, row := range batch {
		placeholders = append(placeholders, rowPlaceholder)
		for _, column := range sourceColumns {
			args = append(args, row[column])
		}
	}
	updates := []string{}
	for _, column := range targetColumns {
		if column != mapping.TargetPrimaryKey {
			q := quoteMySQL(column)
			updates = append(updates, q+"=VALUES("+q+")")
		}
	}
	if len(updates) == 0 {
		q := quoteMySQL(mapping.TargetPrimaryKey)
		updates = append(updates, q+"=VALUES("+q+")")
	}
	query := "INSERT INTO " + quoteMySQL(mapping.TargetTable) + " (" + strings.Join(quoted, ",") + ") VALUES " + strings.Join(placeholders, ",") + " ON DUPLICATE KEY UPDATE " + strings.Join(updates, ",")
	return db.Exec(query, args...).Error
}

func createMySQLTableLike(sourceDB, targetDB *gorm.DB, sourceTable, targetTable string) error {
	row := sourceDB.Raw("SHOW CREATE TABLE " + quoteMySQL(sourceTable)).Row()
	var tableName, ddl string
	if err := row.Scan(&tableName, &ddl); err != nil {
		return err
	}
	from := "CREATE TABLE `" + sourceTable + "`"
	to := "CREATE TABLE `" + targetTable + "`"
	if !strings.Contains(ddl, from) {
		return fmt.Errorf("无法解析源表建表语句")
	}
	return targetDB.Exec(strings.Replace(ddl, from, to, 1)).Error
}

func saveCheckpoint(db *gorm.DB, checkpoint *models.SyncCheckpoint) error {
	return db.Where("task_table_id = ?", checkpoint.TaskTableID).Assign(map[string]interface{}{"cursor_value": checkpoint.CursorValue, "cursor_primary_key": checkpoint.CursorPrimaryKey, "completed": checkpoint.Completed}).FirstOrCreate(checkpoint).Error
}

func quoteMySQL(identifier string) string {
	return "`" + strings.ReplaceAll(identifier, "`", "``") + "`"
}

func valueString(value interface{}) string {
	if bytes, ok := value.([]byte); ok {
		return string(bytes)
	}
	return fmt.Sprint(value)
}
