<script>
  export let token = "";
  export let view = "login";
  export let setView = (v) => {};
  export let connectionTotal = 0;
  export let taskTotal = 0;
  export let logTotal = 0;
  export let sidebarCollapsed = false;
  export let toggleSidebar = () => {};
  export let logout = () => {};
</script>

<aside class="sidebar" class:collapsed={sidebarCollapsed}>
  <div class="brand-block">
    <div class="brand-row">
      <div class="brand">APIWong</div>
      <button class="ghost icon" on:click={toggleSidebar}>{sidebarCollapsed ? "⮞" : "⮜"}</button>
    </div>
    <div class="subtitle">同步任务与数据库管理</div>
  </div>
  {#if token}
    <nav class="menu">
      <button class:active={view === "connections"} on:click={() => setView("connections")}>
        <span class="menu-icon">🗄️</span>
        <span class="menu-text">数据库连接</span>
      </button>
      <button class:active={view === "tasks"} on:click={() => setView("tasks")}>
        <span class="menu-icon">🔄</span>
        <span class="menu-text">同步任务</span>
      </button>
      <button class:active={view === "logs"} on:click={() => setView("logs")}>
        <span class="menu-icon">📜</span>
        <span class="menu-text">同步日志</span>
      </button>
    </nav>
    <div class="side-card">
      <div class="side-title">概览</div>
      <div class="side-stat">
        <span>连接</span>
        <strong>{connectionTotal}</strong>
      </div>
      <div class="side-stat">
        <span>任务</span>
        <strong>{taskTotal}</strong>
      </div>
      <div class="side-stat">
        <span>日志</span>
        <strong>{logTotal}</strong>
      </div>
    </div>
    <button class="outline full" on:click={logout}>退出登录</button>
  {/if}
</aside>
