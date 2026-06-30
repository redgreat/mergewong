<script>
  import { ScrollText } from "lucide-svelte";
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
  export let tasks = [];
  export let logTaskId = "";
  export let logs = [];
  export let logPage = 1;
  export let logPageSize = 10;
  export let logTotal = 0;
  export let onChangeTask = () => {};
  export let onPrev = () => {};
  export let onNext = () => {};
</script>

<svelte:window on:click={handleWindowClick} />

<section class="workspace-panel">
  <div class="card-header">
    <div>
      <h2>同步日志</h2>
    </div>
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
        <th>状态</th>
        <th>消息</th>
        <th>影响行数</th>
        <th>耗时(ms)</th>
      </tr>
    </thead>
    <tbody>
      {#each logs as log}
        <tr>
          <td>{new Date(log.created_at).toLocaleString()}</td>
          <td>
            <span class={`pill ${log.status === "success" ? "success" : log.status === "failed" ? "danger" : "muted"}`}>
              {log.status}
            </span>
          </td>
          <td>{log.message || log.error_detail || "-"}</td>
          <td>{log.rows_affected}</td>
          <td>{log.duration}</td>
        </tr>
      {/each}
      {#if logs.length === 0}
        <tr class="empty-row"><td colspan="5"><div class="empty-state"><span class="empty-icon"><ScrollText size={24} /></span><strong>暂无同步日志</strong></div></td></tr>
      {/if}
    </tbody>
  </table>
  <div class="pager">
    <button class="ghost" disabled={logPage <= 1} on:click={onPrev}>上一页</button>
    <span>{logPage} / {Math.max(1, Math.ceil(logTotal / logPageSize))}</span>
    <button class="ghost" disabled={logPage >= Math.ceil(logTotal / logPageSize)} on:click={onNext}>下一页</button>
  </div>
</section>
