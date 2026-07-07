<script>
  export let loginForm = { username: "", password: "" };
  export let onLogin = () => {};

  let loading = false;

  function handleSubmit(e) {
    e.preventDefault();
    loading = true;
    onLogin().finally(() => (loading = false));
  }
</script>

<div class="login-screen">
  <div class="login-bg">
    <div class="login-glow login-glow-a"></div>
    <div class="login-glow login-glow-b"></div>
  </div>

  <div class="login-card">
    <div class="login-brand">
      <span class="login-logo"><img src="/favicon.png" alt="logo" width="28" height="28" style="border-radius:7px;display:block;" /></span>
      <span class="login-title">数据同步管理平台</span>
    </div>

    <form class="login-form" on:submit={handleSubmit}>
      <label class="login-field">
        <span>用户名</span>
        <input type="text" bind:value={loginForm.username} placeholder="请输入用户名" autocomplete="username" />
      </label>
      <label class="login-field">
        <span>密码</span>
        <input type="password" bind:value={loginForm.password} placeholder="请输入密码" autocomplete="current-password" />
      </label>
      <button type="submit" class="login-btn" disabled={loading || !loginForm.username || !loginForm.password}>
        {loading ? "登录中…" : "登 录"}
      </button>
    </form>

    <p class="login-footer">© {new Date().getFullYear()} wangcw</p>
  </div>
</div>

<style>
  .login-screen {
    position: fixed;
    inset: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    overflow: hidden;
    background: var(--bg);
  }

  .login-bg {
    position: absolute;
    inset: 0;
    pointer-events: none;
  }

  .login-glow {
    position: absolute;
    border-radius: 50%;
    filter: blur(100px);
    opacity: .35;
    animation: glowFloat 12s ease-in-out infinite alternate;
  }

  .login-glow-a {
    width: 480px;
    height: 480px;
    background: var(--primary);
    top: -120px;
    left: -100px;
  }

  .login-glow-b {
    width: 400px;
    height: 400px;
    background: var(--success);
    bottom: -100px;
    right: -80px;
    animation-delay: -6s;
  }

  @keyframes glowFloat {
    0% { transform: translate(0, 0) scale(1); }
    100% { transform: translate(40px, 30px) scale(1.08); }
  }

  .login-card {
    position: relative;
    z-index: 1;
    width: min(400px, calc(100vw - 48px));
    padding: 42px 36px 30px;
    background: var(--surface);
    border: 1px solid var(--border);
    border-radius: 18px;
    box-shadow: var(--shadow);
    animation: cardIn .3s ease;
  }

  @keyframes cardIn {
    from { opacity: 0; transform: translateY(16px) scale(.98); }
  }

  .login-brand {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 12px;
  }

  .login-logo {
    display: inline-grid;
    place-items: center;
    width: 42px;
    height: 42px;
    border-radius: 11px;
    background: var(--primary-soft);
  }

  .login-title {
    font-size: 24px;
    font-weight: 720;
    letter-spacing: -.02em;
    color: var(--text);
  }

  .login-form {
    display: flex;
    flex-direction: column;
    gap: 18px;
    margin-top: 30px;
    padding-top: 2px;
  }

  .login-field {
    display: flex;
    flex-direction: column;
    gap: 7px;
  }

  .login-field span {
    color: var(--text-secondary);
    font-size: 13px;
    font-weight: 550;
  }

  .login-field input {
    min-height: 44px;
    padding-inline: 14px;
    border-radius: 10px;
    background: color-mix(in srgb, var(--surface-2) 70%, transparent);
    transition: border-color .16s ease, box-shadow .16s ease, background .16s ease;
  }

  .login-field input:focus {
    background: var(--surface);
    border-color: color-mix(in srgb, var(--primary) 70%, var(--border));
    box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 18%, transparent);
  }

  .login-btn {
    width: 100%;
    min-height: 44px;
    margin-top: 8px;
    border-radius: 10px;
    font-size: 15px;
    letter-spacing: .04em;
    box-shadow: 0 10px 24px color-mix(in srgb, var(--primary) 28%, transparent);
  }

  .login-footer {
    margin: 24px 0 0;
    text-align: center;
    color: var(--text-muted);
    font-size: 12px;
  }
</style>
