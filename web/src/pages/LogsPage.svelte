<script>
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

<section class="card">
  <div class="card-header">
    <div>
      <h2>同步日志</h2>
      <p>查看每次同步执行结果</p>
    </div>
  </div>
  <div class="toolbar">
    <label>
      选择任务
      <select bind:value={logTaskId} on:change={onChangeTask}>
        <option value="">请选择</option>
        {#each tasks as task}
          <option value={task.id}>{task.name}</option>
        {/each}
      </select>
    </label>
    <div class="toolbar-right">
      <span class="pill">总数 {logTotal}</span>
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
    </tbody>
  </table>
  <div class="pager">
    <button class="ghost" disabled={logPage <= 1} on:click={onPrev}>上一页</button>
    <span>{logPage} / {Math.max(1, Math.ceil(logTotal / logPageSize))}</span>
    <button class="ghost" disabled={logPage >= Math.ceil(logTotal / logPageSize)} on:click={onNext}>下一页</button>
  </div>
</section>
