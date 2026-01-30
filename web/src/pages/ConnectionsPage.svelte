<script>
  export let connections = [];
  export let connectionPage = 1;
  export let connectionPageSize = 10;
  export let connectionTotal = 0;
  export let onPrev = () => {};
  export let onNext = () => {};
  export let onOpenNew = () => {};
  export let onEdit = (c) => {};
  export let onTest = (c) => {};
  export let onDelete = (c) => {};
</script>

<section class="card">
  <div class="card-header">
    <div>
      <h2>连接列表</h2>
      <p>维护系统可用的数据库连接</p>
    </div>
    <div class="header-actions">
      <span class="pill">总数 {connectionTotal}</span>
      <button on:click={onOpenNew}>新增连接</button>
    </div>
  </div>
  <table class="data-table">
    <thead>
      <tr>
        <th>名称</th>
        <th>类型</th>
        <th>地址</th>
        <th>数据库</th>
        <th>用户</th>
        <th>状态</th>
        <th>操作</th>
      </tr>
    </thead>
    <tbody>
      {#each connections as connection}
        <tr>
          <td>{connection.name}</td>
          <td>{connection.type}</td>
          <td>{connection.host}:{connection.port}</td>
          <td>{connection.database}</td>
          <td>{connection.username}</td>
          <td>
            <span class={`pill ${connection.status === 1 ? "success" : "muted"}`}>
              {connection.status === 1 ? "启用" : "禁用"}
            </span>
          </td>
          <td class="row-actions">
            <button class="ghost" on:click={() => onEdit(connection)}>编辑</button>
            <button class="ghost" on:click={() => onTest(connection)}>测试</button>
            <button class="danger" on:click={() => onDelete(connection)}>删除</button>
          </td>
        </tr>
      {/each}
    </tbody>
  </table>
  <div class="pager">
    <button class="ghost" disabled={connectionPage <= 1} on:click={onPrev}>上一页</button>
    <span>{connectionPage} / {Math.max(1, Math.ceil(connectionTotal / connectionPageSize))}</span>
    <button class="ghost" disabled={connectionPage >= Math.ceil(connectionTotal / connectionPageSize)} on:click={onNext}>下一页</button>
  </div>
</section>
