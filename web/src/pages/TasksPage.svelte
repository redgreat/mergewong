<script>
  import { CircleAlert, EllipsisVertical, RefreshCw, Workflow, X } from "lucide-svelte";
  export let tasks = [], taskPage = 1, taskPageSize = 10, taskTotal = 0, canManage = false;
	export let onPrev = () => {}, onNext = () => {}, onOpenNew = () => {}, onEdit = () => {}, onDetail = () => {}, onDelete = () => {}, onRefresh = () => {};
  export let onPause = () => {}, onResume = () => {}, onUpdateCheckpoint = () => {};
  let menuTaskId = null;
  let detailTask = null;
  let checkpointTask = null;
  let deleteTask = null;
  let checkpoint = { file: "", position: 4 };
  let savingCheckpoint = false;

  const statusText = (task) => ({ pending: "待预检查", initializing: "全量初始化", catching_up: "增量追数", cdc_running: "增量同步中", paused: "暂停", stopped: "停止", completed: "完成", failed: "失败" }[task.validation_status === "pending" ? "pending" : task.runtime_status] || "停止");
  const statusClass = (task) => task.runtime_status === "failed" ? "danger" : ["initializing", "catching_up", "cdc_running"].includes(task.runtime_status) ? "success" : "muted";
  const delayText = (seconds) => seconds == null ? "-" : `${(seconds * 1000).toLocaleString()} ms`;
  const speedText = (speed) => speed > 0 ? `${speed >= 1000 ? (speed / 1000).toFixed(1) + "k" : speed.toFixed(1)} 行/秒` : "-";
  const canDelete = (task) => ["paused", "stopped", "completed"].includes(task.runtime_status);

  function openCheckpoint(task) {
    checkpointTask = task;
    checkpoint = { file: task.cdc_checkpoint?.binlog_file || "", position: task.cdc_checkpoint?.binlog_position || 4 };
    menuTaskId = null;
  }
  async function saveCheckpoint() {
    savingCheckpoint = true;
    try { await onUpdateCheckpoint(checkpointTask, { file: checkpoint.file.trim(), position: Number(checkpoint.position) }); checkpointTask = null; }
    finally { savingCheckpoint = false; }
  }
  function handleOutside(event) { if (!event.target.closest(".task-operation")) menuTaskId = null; }
</script>

<svelte:window on:click={handleOutside} />
<section class="workspace-panel task-workspace">
  <div class="card-header"><div></div><div class="header-actions"><span class="record-count">共 {taskTotal} 个任务</span><button class="ghost icon-text" on:click={onRefresh}><RefreshCw size={15} />刷新</button>{#if canManage}<button on:click={onOpenNew}>新增任务</button>{/if}</div></div>
  <table class="data-table task-monitor-table">
    <thead><tr><th>名称</th><th>源连接</th><th>目标连接</th><th>类型</th><th>状态</th><th>同步延迟</th><th>同步速率</th><th>预警</th>{#if canManage}<th>操作</th>{/if}</tr></thead>
    <tbody>
      {#each tasks as task}
        <tr>
		  <td><button class="task-name-link" on:click={() => onDetail(task)}>{task.name}</button>{#if task.task_tables?.length > 1}<span class="cell-sub">{task.task_tables.length} 张表</span>{/if}</td>
          <td>{task.source_db}</td><td>{task.target_db}</td>
          <td>{task.sync_type === "full_cdc" ? "全量 + CDC" : task.sync_type === "cdc" ? "Binlog CDC" : "全量"}</td>
          <td><button class={`status-link ${statusClass(task)}`} class:clickable={task.runtime_status === "failed"} disabled={task.runtime_status !== "failed"} on:click={() => (detailTask = task)}>{#if task.runtime_status === "failed"}<CircleAlert size={14} />{/if}{statusText(task)}</button></td>
          <td>{delayText(task.delay_seconds)}</td><td>{speedText(task.rows_per_second)}</td><td>{task.alert_channel?.name || "-"}</td>
          {#if canManage}<td><div class="task-operation"><button class="icon-button" aria-label={`操作 ${task.name}`} on:click|stopPropagation={() => (menuTaskId = menuTaskId === task.id ? null : task.id)}><EllipsisVertical size={17} /></button>{#if menuTaskId === task.id}<div class="operation-menu">
            {#if ["initializing", "catching_up", "cdc_running"].includes(task.runtime_status)}<button on:click={() => { menuTaskId = null; onPause(task); }}>暂停</button>{:else}<button disabled={task.validation_status !== "passed"} on:click={() => { menuTaskId = null; onResume(task); }}>开始</button>{/if}
            <button on:click={() => { menuTaskId = null; onEdit(task); }}>修改同步对象</button>
			<button on:click={() => { menuTaskId = null; onDetail(task); }}>详情</button>
            <button disabled={task.runtime_status !== "paused" || task.sync_type === "full"} on:click={() => openCheckpoint(task)}>修改 Binlog 位点</button>
            <button class="danger-text" disabled={!canDelete(task)} on:click={() => { deleteTask = task; menuTaskId = null; }}>删除</button>
          </div>{/if}</div></td>{/if}
        </tr>
      {/each}
      {#if tasks.length === 0}<tr class="empty-row"><td colspan={canManage ? 9 : 8}><div class="empty-state"><span class="empty-icon"><Workflow size={24} /></span><strong>还没有同步任务</strong></div></td></tr>{/if}
    </tbody>
  </table>
  <div class="pager"><button class="ghost" disabled={taskPage <= 1} on:click={onPrev}>上一页</button><span>{taskPage} / {Math.max(1, Math.ceil(taskTotal / taskPageSize))}</span><button class="ghost" disabled={taskPage >= Math.ceil(taskTotal / taskPageSize)} on:click={onNext}>下一页</button></div>
</section>

{#if detailTask}<div class="modal-layer"><button class="modal-backdrop" aria-label="关闭" on:click={() => (detailTask = null)}></button><div class="modal compact-modal"><div class="modal-header"><h3>同步失败详情</h3><button class="ghost icon" on:click={() => (detailTask = null)}><X size={17} /></button></div><div class="error-detail"><strong>{detailTask.name}</strong><p>{detailTask.last_run_message || "未记录错误详情"}</p></div><div class="actions"><button on:click={() => (detailTask = null)}>关闭</button></div></div></div>{/if}
{#if checkpointTask}<div class="modal-layer"><button class="modal-backdrop" aria-label="关闭" on:click={() => (checkpointTask = null)}></button><div class="modal compact-modal"><div class="modal-header"><h3>修改 Binlog 位点</h3><button class="ghost icon" on:click={() => (checkpointTask = null)}><X size={17} /></button></div><div class="form-grid single-column"><label>File<input bind:value={checkpoint.file} placeholder="例如：mysql-bin.000123" /></label><label>Position<input type="number" min="4" bind:value={checkpoint.position} placeholder="SHOW MASTER STATUS 中的 Position" /></label></div><div class="actions"><button disabled={savingCheckpoint || !checkpoint.file.trim()} on:click={saveCheckpoint}>{savingCheckpoint ? "保存中…" : "保存位点"}</button><button class="ghost" on:click={() => (checkpointTask = null)}>取消</button></div></div></div>{/if}
{#if deleteTask}<div class="modal-layer"><button class="modal-backdrop" aria-label="关闭" on:click={() => (deleteTask = null)}></button><div class="modal compact-modal"><div class="modal-header"><h3>确认删除任务</h3><button class="ghost icon" on:click={() => (deleteTask = null)}><X size={17} /></button></div><p>确定删除“{deleteTask.name}”吗？任务配置会被删除，同步日志会保留。</p><div class="actions"><button class="danger" on:click={() => { const task = deleteTask; deleteTask = null; onDelete(task); }}>确认删除</button><button class="ghost" on:click={() => (deleteTask = null)}>取消</button></div></div></div>{/if}
