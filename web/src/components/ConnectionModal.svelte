<script>
  export let open = false;
  export let editing = false;
  export let form = {};
  export let onClose = () => {};
  export let onSave = () => {};

  let errors = {};

  function changeType(event) {
    const defaults = { mysql: 3306, postgres: 5432, sqlserver: 1433, oracle: 1521 };
    form.type = event.currentTarget.value;
    form.port = defaults[form.type];
    if (form.type === "mysql" && !form.charset) form.charset = "utf8mb4";
  }

  function validate() {
    errors = {};
    if (!form.name?.trim()) errors.name = "请填写连接名称";
    if (!form.host?.trim()) errors.host = "请填写主机地址";
    if (!form.port) errors.port = "请填写端口";
    if (!form.database?.trim()) errors.database = "请填写数据库名称";
    if (!form.username?.trim()) errors.username = "请填写用户名";
    if (!editing && !form.password) errors.password = "请填写密码";
    return Object.keys(errors).length === 0;
  }

  function handleSave() {
    if (!validate()) return;
    onSave();
  }
</script>

<svelte:window on:keydown={(event) => event.key === "Escape" && open && onClose()} />
{#if open}
  <div class="modal-layer">
    <button class="modal-backdrop" type="button" aria-label="关闭" on:click={onClose}></button>
    <div class="modal connection-modal" role="dialog" aria-modal="true">
      <div class="modal-header">
        <div><h3>{editing ? "编辑数据库连接" : "新增数据库连接"}</h3></div>
        <button class="ghost icon" on:click={onClose}>✕</button>
      </div>
      <div class="form-grid">
        <label class="full">
          连接名称
          <input type="text" bind:value={form.name} placeholder="例如：生产库-源端只读" autocomplete="off" />
          {#if errors.name}<span class="field-error">{errors.name}</span>{/if}
        </label>
        <label>
          数据库类型
          <select value={form.type} on:change={changeType}>
            <option value="mysql">MySQL</option>
            <option value="postgres">PostgreSQL</option>
            <option value="sqlserver">SQL Server</option>
            <option value="oracle">Oracle</option>
          </select>
        </label>
        <label>
          连接用途
          <select bind:value={form.usage}>
            <option value="source">源端（读取数据）</option>
            <option value="target">目标端（写入数据）</option>
            <option value="both">源端和目标端</option>
          </select>
        </label>
        <label>
          主机地址
          <input type="text" bind:value={form.host} placeholder="例如：10.0.0.12 或 db.example.com" autocomplete="off" />
          {#if errors.host}<span class="field-error">{errors.host}</span>{/if}
        </label>
        <label>
          端口
          <input type="number" min="1" max="65535" bind:value={form.port} />
          {#if errors.port}<span class="field-error">{errors.port}</span>{/if}
        </label>
        <label>
          数据库名称
          <input type="text" bind:value={form.database} placeholder="填写 database / schema 所属数据库" autocomplete="off" />
          {#if errors.database}<span class="field-error">{errors.database}</span>{/if}
        </label>
        <label>
          字符集
          <input type="text" bind:value={form.charset} placeholder="utf8mb4" />
        </label>
        <label>
          用户名
          <input type="text" bind:value={form.username} placeholder={form.usage === "source" ? "建议使用只读账号" : "请输入数据库账号"} autocomplete="username" />
          {#if errors.username}<span class="field-error">{errors.username}</span>{/if}
        </label>
        <label>
          密码
          <input type="password" bind:value={form.password} placeholder={editing ? "留空表示不修改" : "请输入数据库密码"} autocomplete="new-password" />
          {#if errors.password}<span class="field-error">{errors.password}</span>{/if}
        </label>
        <label>
          最大空闲连接
          <input type="number" min="0" bind:value={form.max_idle} />
        </label>
        <label>
          最大打开连接
          <input type="number" min="1" bind:value={form.max_open} />
        </label>
      </div>
      <div class="actions">
        <button on:click={handleSave}>{editing ? "保存修改" : "创建连接"}</button>
        <button class="ghost" on:click={onClose}>取消</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .field-error {
    display: block;
    margin-top: 4px;
    color: var(--danger);
    font-size: 12px;
  }
</style>
