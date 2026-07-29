<script>
  import { RefreshCw, ScrollText, Search } from "lucide-svelte";
  let taskQuery = "";
  let resultsOpen = false;
  $: filteredTasks = tasks.filter((task) => task.name.toLowerCase().includes(taskQuery.trim().toLowerCase())).slice(0, 8);

  function todayString() {
    const d = new Date();
    const y = d.getFullYear();
    const m = String(d.getMonth() + 1).padStart(2, "0");
    const day = String(d.getDate()).padStart(2, "0");
    return `${y}-${m}-${day}`;
  }
  function yesterdayString() {
    const d = new Date();
    d.setDate(d.getDate() - 1);
    const y = d.getFullYear();
    const m = String(d.getMonth() + 1).padStart(2, "0");
    const day = String(d.getDate()).padStart(2, "0");
    return `${y}-${m}-${day}`;
  }

  function chooseTask(task) {
    logTaskId = task ? String(task.id) : "";
    taskQuery = task ? task.name : "";
    resultsOpen = false;
    onChangeTask();
  }

  function handleWindowClick(event) {
    if (!event.target.closest(".task-search")) resultsOpen = false;
  }

  function handleFilter() {
    if (onFilter) onFilter();
  }

	const eventLabel = (value) => ({ task_created: "新增任务", task_updated: "修改任务", task_deleted: "删除任务", precheck: "预检查", snapshot_started: "全量开始", snapshot_completed: "全量完成", cdc_started: "增量开始", cdc_failed: "增量报错", task_paused: "暂停任务", task_resumed: "开始任务", checkpoint_changed: "修改位点", alert_sent: "发送预警" }[value] || value || "运行事件");
	const statusLabel = (value) => ({ success: "成功", failed: "失败", running: "进行中", warning: "预警" }[value] || value);
  export let tasks = [];
  export let logTaskId = "";
  export let logs = [];
  export let logPage = 1;
  export let logPageSize = 10;
  export let logTotal = 0;
  export let logDateFrom = "";
  export let logDateTo = "";
  export let onChangeTask = () => {};
  export let onPrev = () => {};
  export let onNext = () => {};
  export let onRefresh = () => {};
  export let onFilter = null;
</script>

<svelte:window on:click={handleWindowClick} />

<section class="workspace-panel">
  <div class="card-header">
    <div></div>
    <div class="header-actions"><button class="ghost icon-text" on:click={onRefresh}><RefreshCw size={15} />刷新</button></div>
  </div>
  <div class="toolbar log-toolbar">
    <div class="toolbar-row">
      <div class="task-search">
        <label>查询任务<input type="search" placeholder="输入任务名称" bind:value={taskQuery} on:focus={() => (resultsOpen = true)} /></label>
        {#if resultsOpen}
          <div class="search-results">
            <button class:active={!logTaskId} on:click={() => chooseTask(null)}>全部任务</button>
            {#each filteredTasks as task}<button class:active={String(task.id) === String(logTaskId)} on:click={() => chooseTask(task)}>{task.name}</button>{/each}
            {#if filteredTasks.length === 0}<span>没有匹配任务</span>{/if}
          </div>
        {/if}
      </div>
      <div class="date-filter">
        <div class="date-field">
          <span class="date-label">从</span>
          <input type="date" bind:value={logDateFrom} class="date-input" />
        </div>
        <div class="date-field">
          <span class="date-label">至</span>
          <input type="date" bind:value={logDateTo} class="date-input" />
        </div>
        <button class="ghost icon-text filter-btn" on:click={handleFilter}><Search size={14} />筛选</button>
      </div>
      <div class="toolbar-right">
        <span class="record-count">共 {logTotal} 条记录</span>
      </div>
    </div>
  </div>
  <table class="data-table">
    <thead>
      <tr>
        <th>时间</th>
			<th>同步任务</th>
			<th>事件</th>
        <th>状态</th>
			<th>阶段与详情</th>
			<th>数据量</th>
			<th>耗时</th>
      </tr>
    </thead>
    <tbody>
      {#each logs as log}
        <tr>
          <td>{new Date(log.created_at).toLocaleString()}</td>
			  <td>{log.task_name || tasks.find((task) => task.id === log.task_id)?.name || `任务 #${log.task_id}`}</td>
			  <td>{eventLabel(log.event_type)}</td>
          <td>
            <span class={`pill ${log.status === "success" ? "success" : log.status === "failed" ? "danger" : "muted"}`}>
				  {statusLabel(log.status)}
            </span>
          </td>
			  <td><strong>{log.message || "-"}</strong>{#if log.detail || log.error_detail}<span class="cell-sub log-detail">{log.detail || log.error_detail}</span>{/if}</td>
			  <td>{log.rows_affected ? `${log.rows_affected} 行` : "-"}</td>
			  <td>{log.duration ? `${(log.duration / 1000).toFixed(2)} 秒` : "-"}</td>
        </tr>
      {/each}
      {#if logs.length === 0}
			<tr class="empty-row"><td colspan="7"><div class="empty-state"><span class="empty-icon"><ScrollText size={24} /></span><strong>暂无同步日志</strong></div></td></tr>
      {/if}
    </tbody>
  </table>
  <div class="pager">
    <button class="ghost" disabled={logPage <= 1} on:click={onPrev}>上一页</button>
    <span>{logPage} / {Math.max(1, Math.ceil(logTotal / logPageSize))}</span>
    <button class="ghost" disabled={logPage >= Math.ceil(logTotal / logPageSize)} on:click={onNext}>下一页</button>
  </div>
</section>

<style>
  .log-toolbar {
    overflow: visible;
  }
  .toolbar-row {
    display: flex;
    flex-wrap: nowrap;
    align-items: center;
    gap: 0.75rem;
    width: 100%;
  }
  .task-search {
    flex: 0 0 auto;
    min-width: 11rem;
    max-width: 14rem;
  }
  .task-search label {
    white-space: nowrap;
  }
  .task-search input {
    width: 100%;
  }
  .date-filter {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    flex: 0 0 auto;
  }
  .date-field {
    display: flex;
    align-items: center;
    gap: 0.25rem;
  }
  .date-label {
    font-size: 0.8125rem;
    color: var(--text-secondary);
    white-space: nowrap;
    width: 1.2rem;
    text-align: right;
  }
  .date-input {
    padding: 0.35rem 0.5rem;
    border: 1px solid var(--border);
    border-radius: 0.5rem;
    background: var(--bg-primary);
    color: var(--text-primary);
    font-size: 0.8125rem;
    width: 8.5rem;
    height: 1.8rem;
    box-sizing: border-box;
  }
  .date-input:focus {
    border-color: var(--accent);
    outline: none;
    box-shadow: 0 0 0 2px color-mix(in srgb, var(--accent) 30%, transparent);
  }
  .filter-btn {
    height: 1.8rem;
    padding: 0 0.6rem;
    font-size: 0.8125rem;
  }
  .toolbar-right {
    flex: 0 0 auto;
    margin-left: auto;
    white-space: nowrap;
  }
</style>
