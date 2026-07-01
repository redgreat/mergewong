<script>
  import { BellRing, RefreshCw } from "lucide-svelte";
  export let channels = [];
  export let alertPage = 1;
  export let alertPageSize = 10;
  export let alertTotal = 0;
  export let canManage = false;
  export let onPrev = () => {};
  export let onNext = () => {};
  export let onOpenNew = () => {};
  export let onEdit = () => {};
  export let onTest = () => {};
  export let onDelete = () => {};
  export let onRefresh = () => {};
</script>

<section class="workspace-panel">
  <div class="card-header">
    <div><h2>预警发送规则</h2><p>维护同步任务失败时使用的企业微信群机器人。</p></div>
    <div class="header-actions"><span class="record-count">共 {alertTotal} 个发送群</span><button class="ghost icon-text" on:click={onRefresh}><RefreshCw size={15} />刷新</button>{#if canManage}<button on:click={onOpenNew}>新增发送群</button>{/if}</div>
  </div>
  <table class="data-table">
    <thead><tr><th>发送群名称</th><th>发送方式</th><th>机器人 ID</th><th>状态</th>{#if canManage}<th>操作</th>{/if}</tr></thead>
    <tbody>
      {#each channels as channel}
        <tr>
          <td>{channel.name}</td><td>企业微信机器人</td><td><code>{channel.robot_id_mask}</code></td>
          <td><span class={`pill ${channel.status === 1 ? "success" : "muted"}`}>{channel.status === 1 ? "启用" : "禁用"}</span></td>
          {#if canManage}<td class="row-actions"><button class="ghost" on:click={() => onEdit(channel)}>编辑</button><button class="ghost" on:click={() => onTest(channel)}>测试</button><button class="danger" on:click={() => onDelete(channel)}>删除</button></td>{/if}
        </tr>
      {/each}
      {#if channels.length === 0}<tr class="empty-row"><td colspan={canManage ? 5 : 4}><div class="empty-state"><span class="empty-icon"><BellRing size={24} /></span><strong>还没有预警发送群</strong><p>新增后，可在同步任务中选择失败预警的发送方。</p>{#if canManage}<button on:click={onOpenNew}>新增发送群</button>{/if}</div></td></tr>{/if}
    </tbody>
  </table>
  <div class="pager"><button class="ghost" disabled={alertPage <= 1} on:click={onPrev}>上一页</button><span>{alertPage} / {Math.max(1, Math.ceil(alertTotal / alertPageSize))}</span><button class="ghost" disabled={alertPage >= Math.ceil(alertTotal / alertPageSize)} on:click={onNext}>下一页</button></div>
</section>
