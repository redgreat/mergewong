<script>
  import { Database, PanelLeftClose, PanelLeftOpen, ScrollText, UserRoundCog, Workflow } from "lucide-svelte";

  export let token = "";
  export let view = "login";
  export let setView = (v) => {};
  export let connectionTotal = 0;
  export let taskTotal = 0;
  export let logTotal = 0;
  export let sidebarCollapsed = false;
  export let toggleSidebar = () => {};
  export let role = "viewer";
</script>

<aside class="sidebar" class:collapsed={sidebarCollapsed}>
  <div class="brand-block">
    <div class="brand-row">
      <div class="brand"><span class="brand-mark"><img src="/favicon.png" alt="logo" width="24" height="24" style="border-radius:6px;display:block;" /></span><span class="brand-name">数据同步</span></div>
      <button class="icon-button sidebar-toggle" aria-label={sidebarCollapsed ? "展开侧栏" : "收起侧栏"} title={sidebarCollapsed ? "展开侧栏" : "收起侧栏"} on:click={toggleSidebar}>
        {#if sidebarCollapsed}<PanelLeftOpen size={18} />{:else}<PanelLeftClose size={18} />{/if}
      </button>
    </div>
  </div>
  {#if token}
    <nav class="menu">
      <button aria-label="数据库连接" title="数据库连接" class:active={view === "connections"} on:click={() => setView("connections")}>
        <span class="menu-icon"><Database size={18} /></span>
        <span class="menu-text">数据库连接</span>
      </button>
      <button aria-label="同步任务" title="同步任务" class:active={view === "tasks"} on:click={() => setView("tasks")}>
        <span class="menu-icon"><Workflow size={18} /></span>
        <span class="menu-text">同步任务</span>
      </button>
      <button aria-label="同步日志" title="同步日志" class:active={view === "logs"} on:click={() => setView("logs")}>
        <span class="menu-icon"><ScrollText size={18} /></span>
        <span class="menu-text">同步日志</span>
      </button>
      {#if role === "admin"}
        <button aria-label="用户管理" title="用户管理" class:active={view === "users"} on:click={() => setView("users")}>
          <span class="menu-icon"><UserRoundCog size={18} /></span>
          <span class="menu-text">用户管理</span>
        </button>
      {/if}
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
  {/if}
</aside>
