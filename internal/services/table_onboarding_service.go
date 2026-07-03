package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	gomysql "github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/replication"
	"github.com/redgreat/mergewong/internal/database"
	"github.com/redgreat/mergewong/internal/models"
)

func (s *SyncService) AddTaskTablesOnline(taskID uint, requested []models.SyncTaskTable) ([]uint, error) {
	if err := validateTaskTables(requested); err != nil {
		return nil, err
	}
	task, err := s.GetTask(taskID)
	if err != nil {
		return nil, err
	}
	existing := map[string]models.SyncTaskTable{}
	for _, table := range task.TaskTables {
		existing[table.SourceTable] = table
	}
	requestedMap := map[string]models.SyncTaskTable{}
	for _, table := range requested {
		requestedMap[table.SourceTable] = table
	}
	for name, old := range existing {
		next, ok := requestedMap[name]
		if !ok {
			return nil, fmt.Errorf("运行中的任务不能移除表 %s，请先暂停任务", name)
		}
		if next.TargetTable != old.TargetTable {
			return nil, fmt.Errorf("运行中的任务不能修改表 %s 的目标表名，请先暂停任务", name)
		}
	}
	file, pos, err := currentMySQLPosition(task.SourceDB)
	if err != nil {
		return nil, err
	}
	sourceDB, err := database.GetManager().GetConnection(task.SourceDB)
	if err != nil {
		return nil, err
	}
	var added []models.SyncTaskTable
	for _, table := range requested {
		if _, ok := existing[table.SourceTable]; ok {
			continue
		}
		columns, err := describeMySQLTable(sourceDB, table.SourceTable)
		if err != nil {
			return nil, fmt.Errorf("读取新增表 %s 失败: %w", table.SourceTable, err)
		}
		pk := primaryKeyOf(columns)
		if pk == "" {
			return nil, fmt.Errorf("新增表 %s 必须有单列主键", table.SourceTable)
		}
		table.TaskID, table.Position = taskID, len(task.TaskTables)+len(added)
		table.SourcePrimaryKey, table.TargetPrimaryKey = pk, mappedColumn(table.FieldMapping, pk)
		table.SyncState, table.OnboardingFile, table.OnboardingPosition = "initializing", file, pos
		table.ProgressMessage = "等待独立初始化链路启动"
		added = append(added, table)
	}
	if len(added) == 0 {
		return nil, nil
	}
	if err := s.systemDB.Create(&added).Error; err != nil {
		return nil, err
	}
	ids := make([]uint, len(added))
	names := ""
	for i := range added {
		ids[i] = added[i].ID
		if i > 0 {
			names += "、"
		}
		names += added[i].SourceTable
	}
	s.RecordTaskEvent(task, "tables_onboarding_started", "object_onboarding", "running", "新增同步对象开始初始化", names, 0, 0)
	go s.runTableOnboarding(taskID, ids)
	return ids, nil
}

func (s *SyncService) runTableOnboarding(taskID uint, tableIDs []uint) {
	task, err := s.GetTask(taskID)
	if err != nil {
		return
	}
	var tables []models.SyncTaskTable
	if err := s.systemDB.Where("id IN ?", tableIDs).Find(&tables).Error; err != nil {
		return
	}
	fail := func(err error) {
		_ = s.systemDB.Model(&models.SyncTaskTable{}).Where("id IN ?", tableIDs).Updates(map[string]interface{}{"sync_state": "failed", "progress_message": err.Error()}).Error
		s.RecordTaskEvent(task, "tables_onboarding_failed", "object_onboarding", "failed", "新增同步对象初始化失败", err.Error(), 0, 0)
	}
	sourceDB, err := database.GetManager().GetConnection(task.SourceDB)
	if err != nil {
		fail(err)
		return
	}
	targetDB, err := database.GetManager().GetConnection(task.TargetDB)
	if err != nil {
		fail(err)
		return
	}
	var initialized int64
	for i := range tables {
		rows, err := s.syncValidatedTable(task, &tables[i], sourceDB, targetDB)
		if err != nil {
			fail(err)
			return
		}
		initialized += rows
	}
	_ = s.systemDB.Model(&models.SyncTaskTable{}).Where("id IN ?", tableIDs).Updates(map[string]interface{}{"sync_state": "catching_up", "progress_message": "正在追赶主链路 Binlog 位点"}).Error
	s.RecordTaskEvent(task, "tables_catchup_started", "object_onboarding", "running", "新增同步对象开始追数", "", initialized, 0)

	start := gomysql.Position{Name: tables[0].OnboardingFile, Pos: tables[0].OnboardingPosition}
	var main models.SyncCDCCheckpoint
	if err := s.systemDB.Where("task_id = ?", taskID).First(&main).Error; err != nil {
		fail(err)
		return
	}
	target := gomysql.Position{Name: main.BinlogFile, Pos: main.BinlogPosition}
	reached, err := s.catchupTables(task, tables, start, target)
	if err != nil {
		fail(err)
		return
	}

	GetCDCManager().StopTask(taskID)
	if err := s.systemDB.Where("task_id = ?", taskID).First(&main).Error; err != nil {
		fail(err)
		return
	}
	finalTarget := gomysql.Position{Name: main.BinlogFile, Pos: main.BinlogPosition}
	if reached.Compare(finalTarget) < 0 {
		if _, err := s.catchupTables(task, tables, reached, finalTarget); err != nil {
			fail(err)
			_ = GetCDCManager().StartTask(taskID)
			return
		}
	}
	now := time.Now()
	if err := s.systemDB.Model(&models.SyncTaskTable{}).Where("id IN ?", tableIDs).Updates(map[string]interface{}{"sync_state": "active", "progress_percent": 100, "progress_message": "已追平并合并到主同步链路", "activated_at": &now}).Error; err != nil {
		fail(err)
		_ = GetCDCManager().StartTask(taskID)
		return
	}
	s.RecordTaskEvent(task, "tables_merged", "object_onboarding", "success", "新增同步对象已追平并合并", fmt.Sprintf("合并位点 %s:%d", finalTarget.Name, finalTarget.Pos), initialized, 0)
	if err := GetCDCManager().StartTask(taskID); err != nil {
		fail(err)
	}
}

func (s *SyncService) catchupTables(task *models.SyncTask, tables []models.SyncTaskTable, start, target gomysql.Position) (gomysql.Position, error) {
	if start.Compare(target) >= 0 {
		return start, nil
	}
	var source models.DatabaseConnection
	if err := s.systemDB.Where("name = ?", task.SourceDB).First(&source).Error; err != nil {
		return start, err
	}
	targetDB, err := database.GetManager().GetConnection(task.TargetDB)
	if err != nil {
		return start, err
	}
	syncer := replication.NewBinlogSyncer(replication.BinlogSyncerConfig{ServerID: 510000 + uint32(task.ID), Flavor: "mysql", Host: source.Host, Port: uint16(source.Port), User: source.Username, Password: source.Password, Charset: source.Charset, ParseTime: true})
	defer syncer.Close()
	streamer, err := syncer.StartSync(start)
	if err != nil {
		return start, err
	}
	mappings := map[string]*models.SyncTaskTable{}
	for i := range tables {
		mappings[tables[i].SourceTable] = &tables[i]
	}
	columns := map[string][]string{}
	var operations []cdcOperation
	current := start
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	for {
		event, err := streamer.GetEvent(ctx)
		if err != nil {
			return current, err
		}
		switch e := event.Event.(type) {
		case *replication.RotateEvent:
			current.Name = string(e.NextLogName)
		case *replication.RowsEvent:
			if string(e.Table.Schema) != source.Database {
				continue
			}
			mapping := mappings[string(e.Table.Table)]
			if mapping == nil {
				continue
			}
			cols := columns[mapping.SourceTable]
			if len(cols) == 0 {
				cols, err = mysqlColumnNames(task.SourceDB, mapping.SourceTable)
				if err != nil {
					return current, err
				}
				columns[mapping.SourceTable] = cols
			}
			switch event.Header.EventType {
			case replication.WRITE_ROWS_EVENTv1, replication.WRITE_ROWS_EVENTv2:
				for _, row := range e.Rows {
					operations = append(operations, cdcOperation{kind: "upsert", mapping: mapping, columns: cols, values: row})
				}
			case replication.UPDATE_ROWS_EVENTv1, replication.UPDATE_ROWS_EVENTv2:
				for i := 1; i < len(e.Rows); i += 2 {
					operations = append(operations, cdcOperation{kind: "upsert", mapping: mapping, columns: cols, values: e.Rows[i]})
				}
			case replication.DELETE_ROWS_EVENTv1, replication.DELETE_ROWS_EVENTv2:
				for _, row := range e.Rows {
					operations = append(operations, cdcOperation{kind: "delete", mapping: mapping, columns: cols, values: row})
				}
			}
		case *replication.XIDEvent:
			if err := applyCDCTransaction(targetDB, operations); err != nil {
				return current, err
			}
			operations = operations[:0]
			current.Pos = event.Header.LogPos
			if current.Compare(target) >= 0 {
				return current, nil
			}
		case *replication.QueryEvent:
			if strings.EqualFold(strings.TrimSpace(string(e.Query)), "COMMIT") {
				if err := applyCDCTransaction(targetDB, operations); err != nil {
					return current, err
				}
				operations = operations[:0]
				current.Pos = event.Header.LogPos
				if current.Compare(target) >= 0 {
					return current, nil
				}
			}
		}
	}
}

func (s *SyncService) ResumePendingTableOnboarding() {
	var taskIDs []uint
	_ = s.systemDB.Model(&models.SyncTaskTable{}).Where("sync_state IN ?", []string{"initializing", "snapshot_completed", "catching_up"}).Distinct().Pluck("task_id", &taskIDs).Error
	for _, taskID := range taskIDs {
		var ids []uint
		_ = s.systemDB.Model(&models.SyncTaskTable{}).Where("task_id = ? AND sync_state IN ?", taskID, []string{"initializing", "snapshot_completed", "catching_up"}).Pluck("id", &ids).Error
		if len(ids) > 0 {
			go s.runTableOnboarding(taskID, ids)
		}
	}
}
