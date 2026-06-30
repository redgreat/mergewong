<script>
  import { UsersRound } from "lucide-svelte";

  export let users = [];
  export let userPage = 1;
  export let userPageSize = 10;
  export let userTotal = 0;
  export let currentUserId = 0;
  export let onPrev = () => {};
  export let onNext = () => {};
  export let onOpenNew = () => {};
  export let onEdit = () => {};
  export let onDelete = () => {};
</script>

<section class="workspace-panel">
  <div class="card-header">
    <div><h2>用户管理</h2></div>
    <div class="header-actions">
      <span class="record-count">共 {userTotal} 个用户</span>
      <button on:click={onOpenNew}>新增用户</button>
    </div>
  </div>
  <table class="data-table">
    <thead><tr><th>用户名</th><th>邮箱</th><th>角色</th><th>状态</th><th>创建时间</th><th>操作</th></tr></thead>
    <tbody>
      {#each users as user}
        <tr>
          <td><strong>{user.username}</strong>{#if user.id === currentUserId}<span class="self-label">当前用户</span>{/if}</td>
          <td>{user.email || "-"}</td>
          <td><span class="pill">{user.role === "admin" ? "管理员" : "只读用户"}</span></td>
          <td><span class={`pill ${user.status === 1 ? "success" : "muted"}`}>{user.status === 1 ? "启用" : "禁用"}</span></td>
          <td>{new Date(user.created_at).toLocaleString()}</td>
          <td class="row-actions">
            <button class="ghost" on:click={() => onEdit(user)}>编辑</button>
            <button class="danger" disabled={user.id === currentUserId} on:click={() => onDelete(user)}>删除</button>
          </td>
        </tr>
      {/each}
      {#if users.length === 0}
        <tr class="empty-row"><td colspan="6"><div class="empty-state"><span class="empty-icon"><UsersRound size={24} /></span><strong>暂无用户</strong><button on:click={onOpenNew}>新增用户</button></div></td></tr>
      {/if}
    </tbody>
  </table>
  <div class="pager">
    <button class="ghost" disabled={userPage <= 1} on:click={onPrev}>上一页</button>
    <span>{userPage} / {Math.max(1, Math.ceil(userTotal / userPageSize))}</span>
    <button class="ghost" disabled={userPage >= Math.ceil(userTotal / userPageSize)} on:click={onNext}>下一页</button>
  </div>
</section>
