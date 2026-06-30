<script>
  import { ChevronDown, KeyRound, LogOut, Moon, Sun, UserRound } from "lucide-svelte";

  export let view = "login";
  export let token = "";
  export let theme = "dark";
  export let onToggleTheme = () => {};
  export let logout = () => {};
  export let onChangePassword = () => {};
  export let user = {};

  let menuOpen = false;

  $: pageTitle = view === "connections" ? "数据库连接" : view === "tasks" ? "同步任务" : view === "logs" ? "同步日志" : view === "users" ? "用户管理" : "登录";

  function handleWindowClick(event) {
    if (!event.target.closest(".account-menu")) menuOpen = false;
  }
</script>

<svelte:window on:click={handleWindowClick} />

<header class="topbar">
  <div class="breadcrumb" aria-label="面包屑">
    <span>数据同步</span>
    <span class="breadcrumb-separator">/</span>
    <strong>{pageTitle}</strong>
  </div>

  {#if token}
    <div class="top-actions">
      <button class="icon-button" aria-label={theme === "dark" ? "切换到白天主题" : "切换到暗黑主题"} title={theme === "dark" ? "白天主题" : "暗黑主题"} on:click={onToggleTheme}>
        {#if theme === "dark"}<Sun size={18} />{:else}<Moon size={18} />{/if}
      </button>
      <div class="account-menu">
        <button class="account-trigger" aria-expanded={menuOpen} on:click|stopPropagation={() => (menuOpen = !menuOpen)}>
          <span class="avatar"><UserRound size={16} /></span>
          <span class="account-copy"><strong>{user.username || "用户"}</strong></span>
          <ChevronDown size={15} class={menuOpen ? "rotated" : ""} />
        </button>
        {#if menuOpen}
          <div class="account-dropdown">
            <div class="account-summary"><strong>{user.username || "用户"}</strong><span>{user.role === "admin" ? "管理员" : "只读用户"}</span></div>
            <button class="password-action" on:click={onChangePassword}><KeyRound size={16} />修改密码</button>
            <button on:click={logout}><LogOut size={16} />退出登录</button>
          </div>
        {/if}
      </div>
    </div>
  {/if}
</header>
