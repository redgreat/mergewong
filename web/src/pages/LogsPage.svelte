<script>
  import { RefreshCw, ScrollText } from "lucide-svelte";
  let taskQuery = "";
  let resultsOpen = false;
  $: filteredTasks = tasks.filter((task) => task.name.toLowerCase().includes(taskQuery.trim().toLowerCase())).slice(0, 8);

  function chooseTask(task) {
    logTaskId = task ? String(task.id) : "";
    taskQuery = task ? task.name : "";
    resultsOpen = false;
    onChangeTask();
  }

  function handleWindowClick(event) {
    if (!event.target.closest(".task-search")) resultsOpen = false;
  }
	const eventLabel = (value) => ({ task_created: "新增任务", task_updated: "修改任务", task_deleted: "删除任务", precheck: "预检查", snapshot_started: "全量开始", snapshot_completed: "全量完成", cdc_started: "增量开始", cdc_failed: "增量报错", task_paused: "暂停任务", task_resumed: "开始任务", checkpoint_changed: "修改位点", alert_sent: "发送预警" }[value] || value || "运行事件");
	const statusLabel = (value) => ({ success: "成功", failed: "失败", running: "进行中", warning: "预警" }[value] || value);
  export let tasks = [];
  export let logTaskId = "";
  export let logs = [];
  export let logPage = 1;
  export let logPageSize = 10;
  export let logTotal = 0;
  export let onChangeTask = () => {};
  export let onPrev = () => {};
  export let onNext = () => {};
  export let onRefresh = () => {};
</script>

<svelte:window on:click={handleWindowClick} />

<section class="workspace-panel">
  <div class="card-header">
    <div></div>
    <div class="header-actions"><button class="ghost icon-text" on:click={onRefresh}><RefreshCw size={15} />刷新</button></div>
  </div>
  <div class="toolbar">
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
    <div class="toolbar-right">
      <span class="record-count">共 {logTotal} 条记录</span>
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
