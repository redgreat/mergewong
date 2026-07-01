<script>
  import { Database, RefreshCw } from "lucide-svelte";
  export let connections = [];
  export let tasks = [];
  export let connectionPage = 1;
  export let connectionPageSize = 10;
  export let connectionTotal = 0;
  export let onPrev = () => {};
  export let onNext = () => {};
  export let onOpenNew = () => {};
  export let onEdit = (c) => {};
  export let onTest = (c) => {};
  export let onDelete = (c) => {};
  export let canManage = false;
  export let onRefresh = () => {};

  function isUsedByTask(conn) {
    return tasks.some(t => t.source_db === conn.name || t.target_db === conn.name);
  }
</script>

<section class="workspace-panel">
  <div class="card-header">
    <div>
      <h2>数据库连接</h2>
    </div>
    <div class="header-actions">
      <span class="record-count">共 {connectionTotal} 个连接</span>
      <button class="ghost icon-text" on:click={onRefresh}><RefreshCw size={15} />刷新</button>
      {#if canManage}<button on:click={onOpenNew}>新增连接</button>{/if}
    </div>
  </div>
  <table class="data-table">
    <thead>
      <tr>
        <th>名称</th>
        <th>类型</th>
        <th>用途</th>
        <th>地址</th>
        <th>数据库</th>
        <th>用户</th>
        {#if canManage}<th>操作</th>{/if}
      </tr>
    </thead>
    <tbody>
      {#each connections as connection}
        {@const inUse = isUsedByTask(connection)}
        <tr>
          <td>{connection.name}</td>
          <td>{connection.type}</td>
          <td><span class="pill">{connection.usage === "source" ? "源端" : connection.usage === "target" ? "目标端" : "源端 / 目标端"}</span></td>
          <td>{connection.host}:{connection.port}</td>
          <td>{connection.database}</td>
          <td>{connection.username}</td>
          {#if canManage}<td class="row-actions">
            <button class="ghost" disabled={inUse} title={inUse ? "该连接正被同步任务使用，不可编辑" : ""} on:click={() => !inUse && onEdit(connection)}>编辑</button>
            <button class="ghost" on:click={() => onTest(connection)}>测试</button>
            <button class="danger" disabled={inUse} title={inUse ? "该连接正被同步任务使用，不可删除" : ""} on:click={() => !inUse && onDelete(connection)}>删除</button>
          </td>{/if}
        </tr>
      {/each}
      {#if connections.length === 0}
        <tr class="empty-row"><td colspan={canManage ? 7 : 6}><div class="empty-state"><span class="empty-icon"><Database size={24} /></span><strong>还没有数据库连接</strong>{#if canManage}<button on:click={onOpenNew}>新增连接</button>{/if}</div></td></tr>
      {/if}
    </tbody>
  </table>
  <div class="pager">
    <button class="ghost" disabled={connectionPage <= 1} on:click={onPrev}>上一页</button>
    <span>{connectionPage} / {Math.max(1, Math.ceil(connectionTotal / connectionPageSize))}</span>
    <button class="ghost" disabled={connectionPage >= Math.ceil(connectionTotal / connectionPageSize)} on:click={onNext}>下一页</button>
  </div>
</section>
