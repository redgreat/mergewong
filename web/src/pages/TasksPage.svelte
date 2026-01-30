<script>
  export let tasks = [];
  export let taskPage = 1;
  export let taskPageSize = 10;
  export let taskTotal = 0;
  export let onPrev = () => {};
  export let onNext = () => {};
  export let onOpenNew = () => {};
  export let onEdit = (t) => {};
  export let onExecute = (t) => {};
  export let onDelete = (t) => {};
</script>

<section class="card">
  <div class="card-header">
    <div>
      <h2>任务列表</h2>
      <p>管理全量与增量同步任务</p>
    </div>
    <div class="header-actions">
      <span class="pill">总数 {taskTotal}</span>
      <button on:click={onOpenNew}>新增任务</button>
    </div>
  </div>
  <table class="data-table">
    <thead>
      <tr>
        <th>名称</th>
        <th>源表</th>
        <th>目标表</th>
        <th>类型</th>
        <th>状态</th>
        <th>最近执行</th>
        <th>操作</th>
      </tr>
    </thead>
    <tbody>
      {#each tasks as task}
        <tr>
          <td>{task.name}</td>
          <td>{task.source_db}.{task.source_table}</td>
          <td>{task.target_db}.{task.target_table}</td>
          <td>{task.sync_type === "incremental" ? "增量" : "全量"}</td>
          <td>
            <span class={`pill ${task.status === 1 ? "success" : "muted"}`}>
              {task.status === 1 ? "启用" : "禁用"}
            </span>
          </td>
          <td>{task.last_run_status || "-"}</td>
          <td class="row-actions">
            <button class="ghost" on:click={() => onEdit(task)}>编辑</button>
            <button class="ghost" on:click={() => onExecute(task)}>执行</button>
            <button class="danger" on:click={() => onDelete(task)}>删除</button>
          </td>
        </tr>
      {/each}
    </tbody>
  </table>
  <div class="pager">
    <button class="ghost" disabled={taskPage <= 1} on:click={onPrev}>上一页</button>
    <span>{taskPage} / {Math.max(1, Math.ceil(taskTotal / taskPageSize))}</span>
    <button class="ghost" disabled={taskPage >= Math.ceil(taskTotal / taskPageSize)} on:click={onNext}>下一页</button>
  </div>
</section>
