<script>
  export let view = "login";
  export let token = "";
</script>

<div class="topbar">
  <div class="top-left">
    <div class="page-title">
      {view === "connections" ? "数据库连接" : view === "tasks" ? "同步任务" : view === "logs" ? "同步日志" : "登录"}
    </div>
    <div class="page-subtitle">APIWong 管理台 · API /api</div>
  </div>
  {#if token}
    <div class="top-actions">
      <span class="pill">在线</span>
    </div>
  {/if}
</div>
