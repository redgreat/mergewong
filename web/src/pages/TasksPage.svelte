<script>
  import { Workflow } from "lucide-svelte";
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
  export let canManage = false;
</script>

<section class="workspace-panel">
  <div class="card-header">
    <div>
      <h2>同步任务</h2>
    </div>
    <div class="header-actions">
      <span class="record-count">共 {taskTotal} 个任务</span>
      {#if canManage}<button on:click={onOpenNew}>新增任务</button>{/if}
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
        {#if canManage}<th>操作</th>{/if}
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
          {#if canManage}<td class="row-actions">
            <button class="ghost" on:click={() => onEdit(task)}>编辑</button>
            <button class="ghost" on:click={() => onExecute(task)}>执行</button>
            <button class="danger" on:click={() => onDelete(task)}>删除</button>
          </td>{/if}
        </tr>
      {/each}
      {#if tasks.length === 0}
        <tr class="empty-row"><td colspan={canManage ? 7 : 6}><div class="empty-state"><span class="empty-icon"><Workflow size={24} /></span><strong>还没有同步任务</strong>{#if canManage}<button on:click={onOpenNew}>新增任务</button>{/if}</div></td></tr>
      {/if}
    </tbody>
  </table>
  <div class="pager">
    <button class="ghost" disabled={taskPage <= 1} on:click={onPrev}>上一页</button>
    <span>{taskPage} / {Math.max(1, Math.ceil(taskTotal / taskPageSize))}</span>
    <button class="ghost" disabled={taskPage >= Math.ceil(taskTotal / taskPageSize)} on:click={onNext}>下一页</button>
  </div>
</section>
