<script>
  export let loginForm = { username: "", password: "" };
  export let onLogin = () => {};
</script>

<section class="card login">
  <div class="card-header">
    <div>
      <h2>登录管理台</h2>
      <p>使用管理员账户进入系统</p>
    </div>
  </div>
  <div class="form-grid">
    <label>
      用户名
      <input type="text" bind:value={loginForm.username} />
    </label>
    <label>
      密码
      <input type="password" bind:value={loginForm.password} />
    </label>
  </div>
  <div class="actions">
    <button on:click={onLogin}>登录</button>
  </div>
</section>
