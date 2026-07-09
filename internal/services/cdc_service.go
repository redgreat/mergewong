package services

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	gomysql "github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/replication"
	"github.com/redgreat/mergewong/internal/database"
	"github.com/redgreat/mergewong/internal/models"
	"gorm.io/gorm"
)

type cdcOperation struct {
	kind    string
	mapping *models.SyncTaskTable
	columns []string
	values  []interface{}
}

type cdcOperationRecord struct {
	Kind        string        `json:"kind"`
	TaskTableID uint          `json:"task_table_id"`
	Columns     []string      `json:"columns"`
	Values      []interface{} `json:"values"`
}

type cdcOperationMetrics struct {
	Insert int64 `json:"insert"`
	Update int64 `json:"update"`
	Delete int64 `json:"delete"`
}

type CDCManager struct {
	service *SyncService
	mu      sync.Mutex
	workers map[uint]cdcWorker
	nextID  uint64
}

type cdcWorker struct {
	id     uint64
	cancel context.CancelFunc
	done   chan struct{}
}

var cdcManager *CDCManager
var cdcOnce sync.Once

// #region debug-point xa-tx-not-synced:reporter
var xaDebugOnce sync.Once
var xaDebugServerURL string
var xaDebugSessionID string

func xaDebugReport(hypothesisID, location, msg string, data map[string]interface{}) {
	xaDebugOnce.Do(func() {
		xaDebugServerURL = "http://127.0.0.1:7777/event"
		xaDebugSessionID = "xa-tx-not-synced"
		content, err := os.ReadFile(filepath.Join(".dbg", "xa-tx-not-synced.env"))
		if err != nil {
			return
		}
		for _, line := range strings.Split(string(content), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "DEBUG_SERVER_URL=") {
				xaDebugServerURL = strings.TrimSpace(strings.TrimPrefix(line, "DEBUG_SERVER_URL="))
			}
			if strings.HasPrefix(line, "DEBUG_SESSION_ID=") {
				xaDebugSessionID = strings.TrimSpace(strings.TrimPrefix(line, "DEBUG_SESSION_ID="))
			}
		}
	})
	payload, err := json.Marshal(map[string]interface{}{
		"sessionId":     xaDebugSessionID,
		"runId":         "pre",
		"hypothesisId":  hypothesisID,
		"location":      location,
		"msg":           "[DEBUG] " + msg,
		"data":          data,
		"ts":            time.Now().UnixMilli(),
		"traceId":       "",
		"service":       "cdc",
		"task_debug_id": data["task_id"],
	})
	if err != nil {
		return
	}
	go func(url string, body []byte) {
		req, reqErr := http.NewRequest("POST", url, bytes.NewReader(body))
		if reqErr != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		resp, doErr := http.DefaultClient.Do(req)
		if doErr == nil && resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
	}(xaDebugServerURL, payload)
}

// #endregion

func GetCDCManager() *CDCManager {
	cdcOnce.Do(func() { cdcManager = &CDCManager{service: NewSyncService(), workers: map[uint]cdcWorker{}} })
	return cdcManager
}

func (m *CDCManager) StartAll() {
	var tasks []models.SyncTask
	runningStates := []string{"initializing", "catching_up", "cdc_running"}
	if err := m.service.systemDB.Where("status = ? AND validation_status = ? AND sync_type IN ? AND runtime_status IN ?", 1, "passed", []string{"cdc", "full_cdc"}, runningStates).Find(&tasks).Error; err != nil {
		log.Printf("加载 CDC 任务失败: %v", err)
		return
	}
	for _, task := range tasks {
		_ = m.StartTask(task.ID)
	}
}

func (m *CDCManager) StartTask(taskID uint) error {
	task, err := m.service.GetTask(taskID)
	if err != nil {
		return err
	}
	if task.Status == 0 || task.ValidationStatus != "passed" {
		return fmt.Errorf("任务未启用或预检查未通过")
	}
	if task.SyncType != "cdc" && task.SyncType != "full_cdc" {
		return fmt.Errorf("该任务不是 Binlog CDC 任务")
	}
	m.StopTask(taskID)
	ctx, cancel := context.WithCancel(context.Background())
	m.mu.Lock()
	m.nextID++
	workerID := m.nextID
	done := make(chan struct{})
	m.workers[taskID] = cdcWorker{id: workerID, cancel: cancel, done: done}
	m.mu.Unlock()
	go func() {
		defer close(done)
		err := m.run(ctx, task)
		if err != nil && ctx.Err() == nil {
			log.Printf("CDC 任务 %d 停止: %v", taskID, err)
			m.service.recordCDCFailure(task, err)
		} else {
			m.service.recordCDCStopped(task)
		}
		m.mu.Lock()
		if worker, ok := m.workers[taskID]; ok && worker.id == workerID {
			delete(m.workers, taskID)
		}
		m.mu.Unlock()
	}()
	return nil
}

func (m *CDCManager) StopTask(taskID uint) {
	m.mu.Lock()
	worker := m.workers[taskID]
	m.mu.Unlock()
	if worker.cancel != nil {
		worker.cancel()
		select {
		case <-worker.done:
		case <-time.After(10 * time.Second):
		}
	}
	m.mu.Lock()
	if current, ok := m.workers[taskID]; ok && current.id == worker.id {
		delete(m.workers, taskID)
	}
	m.mu.Unlock()
}

func (m *CDCManager) IsRunning(taskID uint) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.workers[taskID]
	return ok
}

func (m *CDCManager) Close() {
	m.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(m.workers))
	for _, worker := range m.workers {
		cancels = append(cancels, worker.cancel)
	}
	m.workers = map[uint]cdcWorker{}
	m.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}

func (m *CDCManager) run(ctx context.Context, task *models.SyncTask) error {
	now := time.Now()
	stage := "catching_up"
	if task.SyncType == "full_cdc" {
		stage = "initializing"
	}
	_ = m.service.UpdateTask(task.ID, map[string]interface{}{"last_run_at": &now, "last_run_status": "running", "runtime_status": stage, "phase_started_at": &now, "rows_per_second": 0, "last_run_message": runtimeLabel(stage)})
	var source models.DatabaseConnection
	if err := m.service.systemDB.Where("name = ?", task.SourceDB).First(&source).Error; err != nil {
		return err
	}
	checkpoint, err := m.loadOrCreateCheckpoint(task, &source)
	if err != nil {
		return err
	}
	if task.SyncType == "full_cdc" && !checkpoint.SnapshotCompleted {
		m.service.RecordTaskEvent(task, "snapshot_started", "snapshot", "running", "全量数据初始化开始", "", 0, 0)
		started := time.Now()
		rows, err := m.service.syncValidatedTask(task)
		if err != nil {
			return fmt.Errorf("全量初始化失败: %w", err)
		}
		checkpoint.SnapshotCompleted = true
		if err := m.service.systemDB.Save(checkpoint).Error; err != nil {
			return err
		}
		m.service.UpdateTask(task.ID, map[string]interface{}{"last_run_message": fmt.Sprintf("全量初始化完成，共 %d 行，正在消费 Binlog", rows)})
		elapsed := time.Since(started)
		_ = m.service.UpdateTask(task.ID, map[string]interface{}{"runtime_status": "catching_up", "rows_processed": rows, "rows_per_second": float64(rows) / elapsed.Seconds(), "phase_started_at": time.Now()})
		m.service.RecordTaskEvent(task, "snapshot_completed", "snapshot", "success", "全量数据初始化完成", "", rows, elapsed.Milliseconds())
	}
	if task.SyncType == "cdc" {
		if err := ensureCDCTargetTables(task); err != nil {
			return err
		}
	}
	activatedAt := time.Now()
	_ = m.service.systemDB.Model(&models.SyncTaskTable{}).Where("task_id = ? AND COALESCE(onboarding_file, '') = '' AND sync_state IN ?", task.ID, []string{"pending", "snapshot_completed", "failed"}).Updates(map[string]interface{}{"sync_state": "active", "progress_percent": 100, "activated_at": &activatedAt, "progress_message": "已合并到主同步链路"}).Error
	task, _ = m.service.GetTask(task.ID)
	return m.stream(ctx, task, &source, checkpoint)
}

func ensureCDCTargetTables(task *models.SyncTask) error {
	sourceDB, err := database.GetManager().GetConnection(task.SourceDB)
	if err != nil {
		return err
	}
	targetDB, err := database.GetManager().GetConnection(task.TargetDB)
	if err != nil {
		return err
	}
	for i := range task.TaskTables {
		mapping := &task.TaskTables[i]
		if targetDB.Migrator().HasTable(mapping.TargetTable) {
			continue
		}
		if len(mapping.FieldMapping) > 0 {
			return fmt.Errorf("目标表 %s 不存在时不能使用字段改名", mapping.TargetTable)
		}
		if err := createMySQLTableLike(sourceDB, targetDB, mapping.SourceTable, mapping.TargetTable); err != nil {
			return err
		}
	}
	return nil
}

func (m *CDCManager) loadOrCreateCheckpoint(task *models.SyncTask, source *models.DatabaseConnection) (*models.SyncCDCCheckpoint, error) {
	var checkpoint models.SyncCDCCheckpoint
	err := m.service.systemDB.Where("task_id = ?", task.ID).First(&checkpoint).Error
	if err == nil {
		return &checkpoint, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	file, pos, err := currentMySQLPosition(task.SourceDB)
	if err != nil {
		return nil, err
	}
	checkpoint = models.SyncCDCCheckpoint{TaskID: task.ID, BinlogFile: file, BinlogPosition: pos, SnapshotCompleted: task.SyncType == "cdc"}
	if err := m.service.systemDB.Create(&checkpoint).Error; err != nil {
		return nil, err
	}
	return &checkpoint, nil
}

func currentMySQLPosition(connectionName string) (string, uint32, error) {
	db, err := database.GetManager().GetConnection(connectionName)
	if err != nil {
		return "", 0, err
	}
	var status struct {
		File     string `gorm:"column:File"`
		Position uint32 `gorm:"column:Position"`
	}
	if err := db.Raw("SHOW MASTER STATUS").Scan(&status).Error; err != nil {
		return "", 0, fmt.Errorf("读取 Binlog 位点失败: %w", err)
	}
	if status.File == "" {
		return "", 0, fmt.Errorf("读取 Binlog 位点失败：源库未返回 Master Status")
	}
	return status.File, status.Position, nil
}

func (m *CDCManager) stream(ctx context.Context, task *models.SyncTask, source *models.DatabaseConnection, checkpoint *models.SyncCDCCheckpoint) error {
	syncer := replication.NewBinlogSyncer(replication.BinlogSyncerConfig{ServerID: 410000 + uint32(task.ID), Flavor: "mysql", Host: source.Host, Port: uint16(source.Port), User: source.Username, Password: source.Password, Charset: source.Charset, ParseTime: true, HeartbeatPeriod: 10 * time.Second})
	defer syncer.Close()
	streamer, err := syncer.StartSync(gomysql.Position{Name: checkpoint.BinlogFile, Pos: checkpoint.BinlogPosition})
	if err != nil {
		return err
	}
	targetDB, err := database.GetManager().GetConnection(task.TargetDB)
	if err != nil {
		return err
	}
	mappings := map[string]*models.SyncTaskTable{}
	for i := range task.TaskTables {
		if task.TaskTables[i].SyncState == "active" {
			mappings[task.TaskTables[i].SourceTable] = &task.TaskTables[i]
		}
	}
	columnCache := map[string][]string{}
	var operations []cdcOperation
	currentFile := checkpoint.BinlogFile
	streamStarted := time.Now()
	lastMetricsUpdate := time.Time{}
	lastMetricsLog := time.Time{}
	lastMetricsRows := int64(0)
	lastMetricsOps := cdcOperationMetrics{}
	var sessionRows int64
	var opMetrics cdcOperationMetrics
	_ = m.service.UpdateTask(task.ID, map[string]interface{}{"runtime_status": "catching_up", "phase_started_at": &streamStarted, "last_run_message": "增量追数中"})

	startTitle := "Binlog 增量同步开始"
	if checkpoint.SnapshotCompleted && task.RowsProcessed > 0 {
		startTitle = "Binlog 增量同步继续"
	}
	m.service.RecordTaskEvent(task, "cdc_started", "cdc", "running", startTitle, fmt.Sprintf("起始位点 %s:%d", checkpoint.BinlogFile, checkpoint.BinlogPosition), 0, 0)
	for {
		event, err := streamer.GetEvent(ctx)
		if err != nil {
			return err
		}
		if event.Header.EventType == replication.XA_PREPARE_LOG_EVENT {
			xidKey, onePhase, parseErr := parseXAPrepareEvent(event.RawData)
			if parseErr != nil {
				return parseErr
			}
			// #region debug-point A:xa_prepare_log_event
			xaDebugReport("A", "cdc_service.go:XA_PREPARE_LOG_EVENT", "捕获 XA_PREPARE_LOG_EVENT", map[string]interface{}{"task_id": task.ID, "xid_key": xidKey, "one_phase": onePhase, "ops_len": len(operations), "file": currentFile, "pos": event.Header.LogPos, "event_ts": event.Header.Timestamp})
			// #endregion
			applied := int64(len(operations))
			if onePhase {
				if err := applyCDCTransaction(targetDB, operations); err != nil {
					return err
				}
			} else if err := m.saveXAPrepared(task.ID, xidKey, currentFile, event.Header.LogPos, operations); err != nil {
				return err
			}
			operations = operations[:0]
			sessionRows += applied
			if err := m.advanceCheckpoint(task, checkpoint, currentFile, event.Header.LogPos, sessionRows, streamStarted, event.Header.Timestamp, &lastMetricsUpdate, &lastMetricsLog, &lastMetricsRows, &lastMetricsOps, opMetrics); err != nil {
				return err
			}
			continue
		}
		switch e := event.Event.(type) {
		case *replication.RotateEvent:
			currentFile = string(e.NextLogName)
		case *replication.RowsEvent:
			if string(e.Table.Schema) != source.Database {
				continue
			}
			mapping := mappings[string(e.Table.Table)]
			if mapping == nil {
				continue
			}
			columns := columnCache[mapping.SourceTable]
			if len(columns) == 0 {
				columns, err = mysqlColumnNames(task.SourceDB, mapping.SourceTable)
				if err != nil {
					return err
				}
				columnCache[mapping.SourceTable] = columns
			}
			if int(e.ColumnCount) != len(columns) {
				return fmt.Errorf("表 %s Binlog 列数与当前表结构不一致", mapping.SourceTable)
			}
			switch event.Header.EventType {
			case replication.WRITE_ROWS_EVENTv1, replication.WRITE_ROWS_EVENTv2:
				for _, row := range e.Rows {
					operations = append(operations, cdcOperation{kind: "upsert", mapping: mapping, columns: columns, values: row})
					opMetrics.Insert++
				}
			case replication.UPDATE_ROWS_EVENTv1, replication.UPDATE_ROWS_EVENTv2:
				for i := 1; i < len(e.Rows); i += 2 {
					operations = append(operations, cdcOperation{kind: "upsert", mapping: mapping, columns: columns, values: e.Rows[i]})
					opMetrics.Update++
				}
			case replication.DELETE_ROWS_EVENTv1, replication.DELETE_ROWS_EVENTv2:
				for _, row := range e.Rows {
					operations = append(operations, cdcOperation{kind: "delete", mapping: mapping, columns: columns, values: row})
					opMetrics.Delete++
				}
			}
		case *replication.XIDEvent:
			// #region debug-point A:xid_event_before_apply
			xaDebugReport("A", "cdc_service.go:XIDEvent", "XIDEvent 提交点触发 apply", map[string]interface{}{"task_id": task.ID, "ops_len": len(operations), "file": currentFile, "pos": event.Header.LogPos, "event_ts": event.Header.Timestamp})
			// #endregion
			applied := int64(len(operations))
			if err := applyCDCTransaction(targetDB, operations); err != nil {
				// #region debug-point D:apply_error
				xaDebugReport("D", "cdc_service.go:XIDEvent", "applyCDCTransaction 返回错误", map[string]interface{}{"task_id": task.ID, "ops_len": len(operations), "err": err.Error()})
				// #endregion
				return err
			}
			operations = operations[:0]
			sessionRows += applied
			if err := m.advanceCheckpoint(task, checkpoint, currentFile, event.Header.LogPos, sessionRows, streamStarted, event.Header.Timestamp, &lastMetricsUpdate, &lastMetricsLog, &lastMetricsRows, &lastMetricsOps, opMetrics); err != nil {
				return err
			}
		case *replication.QueryEvent:
			rawQuery := strings.TrimSpace(string(e.Query))
			query := strings.ToUpper(rawQuery)
			if action, xidKey, ok := parseXAQuery(rawQuery); ok {
				// #region debug-point A:xa_query_detected
				xaDebugReport("A", "cdc_service.go:QueryEvent", "识别到 XA QueryEvent", map[string]interface{}{"task_id": task.ID, "action": action, "xid_key": xidKey, "raw": rawQuery, "ops_len": len(operations), "file": currentFile, "pos": event.Header.LogPos, "event_ts": event.Header.Timestamp})
				// #endregion
				applied := int64(0)
				deletePrepared := action == "rollback"
				switch action {
				case "prepare":
					if err := m.saveXAPrepared(task.ID, xidKey, currentFile, event.Header.LogPos, operations); err != nil {
						return err
					}
					operations = operations[:0]
				case "commit":
					prepared, err := m.loadXAPreparedOperations(task, xidKey)
					if err != nil {
						// #region debug-point B:xa_prepared_load_failed
						xaDebugReport("B", "cdc_service.go:XA_COMMIT", "加载 XA prepared 失败", map[string]interface{}{"task_id": task.ID, "xid_key": xidKey, "err": err.Error(), "ops_len": len(operations)})
						// #endregion
						if len(operations) > 0 {
							applied = int64(len(operations))
							if err := applyCDCTransaction(targetDB, operations); err != nil {
								// #region debug-point D:apply_error_xa_commit_fallback
								xaDebugReport("D", "cdc_service.go:XA_COMMIT", "XA COMMIT fallback applyCDCTransaction 返回错误", map[string]interface{}{"task_id": task.ID, "ops_len": len(operations), "err": err.Error()})
								// #endregion
								return err
							}
							operations = operations[:0]
							deletePrepared = true
							break
						}
						m.service.RecordTaskEvent(task, "xa_commit_without_prepare", "cdc", "failed", "XA COMMIT 未找到 prepared 缓存", fmt.Sprintf("xid=%s；已停止任务等待人工处理", xidKey), 0, 0)
						return fmt.Errorf("XA COMMIT 未找到 prepared 缓存: xid=%s", xidKey)
					} else {
						// #region debug-point A:xa_prepared_loaded
						xaDebugReport("A", "cdc_service.go:XA_COMMIT", "加载 XA prepared 成功并准备 apply", map[string]interface{}{"task_id": task.ID, "xid_key": xidKey, "prepared_ops_len": len(prepared), "file": currentFile, "pos": event.Header.LogPos})
						// #endregion
						applied = int64(len(prepared))
						if err := applyCDCTransaction(targetDB, prepared); err != nil {
							// #region debug-point D:apply_error_xa_commit
							xaDebugReport("D", "cdc_service.go:XA_COMMIT", "XA COMMIT applyCDCTransaction 返回错误", map[string]interface{}{"task_id": task.ID, "prepared_ops_len": len(prepared), "err": err.Error()})
							// #endregion
							return err
						}
						deletePrepared = true
					}
				case "rollback":
					operations = operations[:0]
				}
				if deletePrepared {
					if err := m.deleteXAPrepared(task.ID, xidKey); err != nil {
						return err
					}
				}
				sessionRows += applied
				if err := m.advanceCheckpoint(task, checkpoint, currentFile, event.Header.LogPos, sessionRows, streamStarted, event.Header.Timestamp, &lastMetricsUpdate, &lastMetricsLog, &lastMetricsRows, &lastMetricsOps, opMetrics); err != nil {
					return err
				}
				continue
			}
			if query == "COMMIT" {
				applied := int64(len(operations))
				if err := applyCDCTransaction(targetDB, operations); err != nil {
					return err
				}
				operations = operations[:0]
				sessionRows += applied
				if err := m.advanceCheckpoint(task, checkpoint, currentFile, event.Header.LogPos, sessionRows, streamStarted, event.Header.Timestamp, &lastMetricsUpdate, &lastMetricsLog, &lastMetricsRows, &lastMetricsOps, opMetrics); err != nil {
					return err
				}
			}
		}
	}
}

func (m *CDCManager) advanceCheckpoint(task *models.SyncTask, checkpoint *models.SyncCDCCheckpoint, file string, pos uint32, sessionRows int64, started time.Time, eventTimestamp uint32, lastMetricsUpdate, lastMetricsLog *time.Time, lastMetricsRows *int64, lastMetricsOps *cdcOperationMetrics, opMetrics cdcOperationMetrics) error {
	now := time.Now()
	if !lastMetricsUpdate.IsZero() && now.Sub(*lastMetricsUpdate) < time.Second {
		return nil
	}
	*lastMetricsUpdate = now
	checkpoint.BinlogFile, checkpoint.BinlogPosition, checkpoint.LastEventAt = file, pos, &now
	if err := m.service.systemDB.Model(checkpoint).Updates(map[string]interface{}{"binlog_file": file, "binlog_position": pos, "last_event_at": &now, "snapshot_completed": checkpoint.SnapshotCompleted}).Error; err != nil {
		return err
	}
	delay := int64(0)
	eventTime := time.Unix(int64(eventTimestamp), 0)
	if eventTimestamp > 0 && now.After(eventTime) {
		delay = int64(now.Sub(eventTime).Seconds())
	}
	elapsed := time.Since(started).Seconds()
	speed := float64(0)
	if elapsed > 0 {
		speed = float64(sessionRows) / elapsed
	}
	runtimeStatus := "cdc_running"
	if delay > 5 {
		runtimeStatus = "catching_up"
	}
	if err := m.service.UpdateTask(task.ID, map[string]interface{}{"runtime_status": runtimeStatus, "rows_processed": task.RowsProcessed + sessionRows, "rows_per_second": speed, "delay_seconds": delay, "last_success_at": &now, "last_run_status": "running", "last_run_message": runtimeLabel(runtimeStatus)}); err != nil {
		return err
	}
	// #region debug-point E:checkpoint_advanced
	xaDebugReport("E", "cdc_service.go:advanceCheckpoint", "推进 checkpoint/指标", map[string]interface{}{"task_id": task.ID, "file": file, "pos": pos, "delay_seconds": delay, "runtime_status": runtimeStatus, "session_rows": sessionRows, "op_metrics": opMetrics})
	// #endregion
	return m.service.RecordCDCMetricSnapshot(task, now, delay, speed, sessionRows, lastMetricsLog, lastMetricsRows, lastMetricsOps, opMetrics)
}

func mysqlColumnNames(connectionName, table string) ([]string, error) {
	db, err := database.GetManager().GetConnection(connectionName)
	if err != nil {
		return nil, err
	}
	return mysqlColumnNamesFromDB(db, table)
}

func mysqlColumnNamesFromDB(db *gorm.DB, table string) ([]string, error) {
	columns, err := describeMySQLTable(db, table)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(columns))
	for i := range columns {
		names[i] = columns[i].Field
	}
	return names, nil
}

func parseXAPrepareEvent(raw []byte) (string, bool, error) {
	if len(raw) < replication.EventHeaderSize+10 {
		return "", false, fmt.Errorf("XA PREPARE 事件长度异常")
	}
	body := raw[replication.EventHeaderSize:]
	onePhase := body[0] != 0
	formatID := binary.LittleEndian.Uint32(body[1:5])
	gtridLen := int(binary.LittleEndian.Uint32(body[5:9]))
	if len(body) >= 13 {
		bqualLen32 := int(binary.LittleEndian.Uint32(body[9:13]))
		total := 13 + gtridLen + bqualLen32
		if total > 13 && len(body) >= total {
			gtrid := body[13 : 13+gtridLen]
			bqual := body[13+gtridLen : total]
			return xaKey(formatID, gtrid, bqual), onePhase, nil
		}
	}
	bqualLen8 := int(body[9])
	total := 10 + gtridLen + bqualLen8
	if total <= 10 || len(body) < total {
		return "", false, fmt.Errorf("XA PREPARE XID 长度异常")
	}
	gtrid := body[10 : 10+gtridLen]
	bqual := body[10+gtridLen : total]
	return xaKey(formatID, gtrid, bqual), onePhase, nil
}

func parseXAQuery(query string) (string, string, bool) {
	upper := strings.ToUpper(strings.TrimSpace(query))
	action := ""
	rest := ""
	switch {
	case strings.HasPrefix(upper, "XA PREPARE "):
		action, rest = "prepare", strings.TrimSpace(query[len("XA PREPARE "):])
	case strings.HasPrefix(upper, "XA COMMIT "):
		action, rest = "commit", strings.TrimSpace(query[len("XA COMMIT "):])
	case strings.HasPrefix(upper, "XA ROLLBACK "):
		action, rest = "rollback", strings.TrimSpace(query[len("XA ROLLBACK "):])
	default:
		return "", "", false
	}
	rest = trimXAOnePhase(rest)
	xidKey, err := parseXIDKey(rest)
	if err != nil {
		return "", "", false
	}
	return action, xidKey, true
}

func trimXAOnePhase(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasSuffix(strings.ToUpper(value), " ONE PHASE") {
		return strings.TrimSpace(value[:len(value)-len(" ONE PHASE")])
	}
	return value
}

func parseXIDKey(expr string) (string, error) {
	parts := splitXIDParts(expr)
	if len(parts) == 0 || len(parts) > 3 {
		return "", fmt.Errorf("XA XID 格式不支持: %s", expr)
	}
	gtrid, err := parseXIDBytes(parts[0])
	if err != nil {
		return "", err
	}
	bqual := []byte{}
	if len(parts) > 1 {
		bqual, err = parseXIDBytes(parts[1])
		if err != nil {
			return "", err
		}
	}
	formatID := uint32(1)
	if len(parts) > 2 {
		parsed, err := strconv.ParseUint(strings.TrimSpace(parts[2]), 10, 32)
		if err != nil {
			return "", err
		}
		formatID = uint32(parsed)
	}
	return xaKey(formatID, gtrid, bqual), nil
}

func splitXIDParts(expr string) []string {
	parts := []string{}
	start := 0
	inQuote := false
	for i, r := range expr {
		switch r {
		case '\'':
			inQuote = !inQuote
		case ',':
			if !inQuote {
				parts = append(parts, strings.TrimSpace(expr[start:i]))
				start = i + 1
			}
		}
	}
	parts = append(parts, strings.TrimSpace(expr[start:]))
	return parts
}

func parseXIDBytes(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	upper := strings.ToUpper(value)
	if strings.HasPrefix(upper, "X'") && strings.HasSuffix(value, "'") {
		return hex.DecodeString(value[2 : len(value)-1])
	}
	if strings.HasPrefix(upper, "0X") {
		return hex.DecodeString(value[2:])
	}
	if strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") {
		return []byte(strings.ReplaceAll(value[1:len(value)-1], "''", "'")), nil
	}
	return []byte(value), nil
}

func xaKey(formatID uint32, gtrid, bqual []byte) string {
	return fmt.Sprintf("%d:%s:%s", formatID, hex.EncodeToString(gtrid), hex.EncodeToString(bqual))
}

func (m *CDCManager) saveXAPrepared(taskID uint, xidKey, file string, pos uint32, operations []cdcOperation) error {
	if len(operations) == 0 {
		var existing models.SyncXAPreparedTransaction
		if err := m.loadXAPreparedRecord(taskID, xidKey, &existing); err == nil {
			return nil
		}
	}
	records := make([]cdcOperationRecord, 0, len(operations))
	for _, op := range operations {
		values := make([]interface{}, len(op.values))
		for i := range op.values {
			values[i] = normalizeMySQLScannedValue(op.values[i])
		}
		records = append(records, cdcOperationRecord{Kind: op.kind, TaskTableID: op.mapping.ID, Columns: op.columns, Values: values})
	}
	bytes, err := json.Marshal(records)
	if err != nil {
		return err
	}
	prepared := models.SyncXAPreparedTransaction{TaskID: taskID, XIDKey: xidKey, BinlogFile: file, BinlogPosition: pos, OperationsJSON: string(bytes)}
	return m.service.systemDB.Where("task_id = ? AND xid_key = ?", taskID, xidKey).Assign(prepared).FirstOrCreate(&prepared).Error
}

func (m *CDCManager) loadXAPreparedOperations(task *models.SyncTask, xidKey string) ([]cdcOperation, error) {
	var prepared models.SyncXAPreparedTransaction
	if err := m.loadXAPreparedRecord(task.ID, xidKey, &prepared); err != nil {
		return nil, fmt.Errorf("XA prepared 事务不存在: %s", xidKey)
	}
	var records []cdcOperationRecord
	if err := json.Unmarshal([]byte(prepared.OperationsJSON), &records); err != nil {
		return nil, err
	}
	tableByID := map[uint]*models.SyncTaskTable{}
	for i := range task.TaskTables {
		tableByID[task.TaskTables[i].ID] = &task.TaskTables[i]
	}
	operations := make([]cdcOperation, 0, len(records))
	for _, record := range records {
		mapping := tableByID[record.TaskTableID]
		if mapping == nil {
			return nil, fmt.Errorf("XA prepared 事务引用了不存在的同步表: %d", record.TaskTableID)
		}
		operations = append(operations, cdcOperation{kind: record.Kind, mapping: mapping, columns: record.Columns, values: record.Values})
	}
	return operations, nil
}

func (m *CDCManager) loadXAPreparedRecord(taskID uint, xidKey string, prepared *models.SyncXAPreparedTransaction) error {
	err := m.service.systemDB.Where("task_id = ? AND xid_key = ?", taskID, xidKey).First(prepared).Error
	if err == nil {
		return nil
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	suffix, ok := xaKeySuffix(xidKey)
	if !ok {
		return err
	}
	return m.service.systemDB.Where("task_id = ? AND xid_key LIKE ?", taskID, "%:"+suffix).Order("id DESC").First(prepared).Error
}

func (m *CDCManager) deleteXAPrepared(taskID uint, xidKey string) error {
	query := m.service.systemDB.Where("task_id = ? AND xid_key = ?", taskID, xidKey)
	if suffix, ok := xaKeySuffix(xidKey); ok {
		query = m.service.systemDB.Where("task_id = ? AND (xid_key = ? OR xid_key LIKE ?)", taskID, xidKey, "%:"+suffix)
	}
	return query.Delete(&models.SyncXAPreparedTransaction{}).Error
}

func xaKeySuffix(xidKey string) (string, bool) {
	parts := strings.SplitN(xidKey, ":", 2)
	if len(parts) != 2 || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}

func applyCDCTransaction(db *gorm.DB, operations []cdcOperation) error {
	if len(operations) == 0 {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		for _, op := range operations {
			if op.kind == "upsert" {
				row := map[string]interface{}{}
				for i, column := range op.columns {
					row[column] = normalizeMySQLScannedValue(op.values[i])
				}
				if err := writeMySQLBatchTx(tx, op.mapping, op.columns, []map[string]interface{}{row}); err != nil {
					return err
				}
				continue
			}
			pkIndex := -1
			for i, column := range op.columns {
				if column == op.mapping.SourcePrimaryKey {
					pkIndex = i
					break
				}
			}
			if pkIndex < 0 {
				return fmt.Errorf("表 %s 缺少主键列", op.mapping.SourceTable)
			}
			if err := tx.Exec("DELETE FROM "+quoteMySQL(op.mapping.TargetTable)+" WHERE "+quoteMySQL(op.mapping.TargetPrimaryKey)+" = ?", normalizeMySQLScannedValue(op.values[pkIndex])).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *SyncService) recordCDCFailure(task *models.SyncTask, err error) {
	now := time.Now()
	_ = s.UpdateTask(task.ID, map[string]interface{}{"last_run_at": &now, "last_run_status": "failed", "runtime_status": "failed", "last_run_message": err.Error()})
	s.RecordTaskEvent(task, "cdc_failed", "cdc", "failed", "CDC 同步失败", err.Error(), 0, 0)
}

func (s *SyncService) recordCDCStopped(task *models.SyncTask) {
	var checkpoint models.SyncCDCCheckpoint
	detail := ""
	if err := s.systemDB.Where("task_id = ?", task.ID).First(&checkpoint).Error; err == nil {
		detail = fmt.Sprintf("停留位点 %s:%d", checkpoint.BinlogFile, checkpoint.BinlogPosition)
	}
	s.RecordTaskEvent(task, "cdc_stopped", "cdc", "success", "Binlog 增量同步已停止", detail, 0, 0)
}
