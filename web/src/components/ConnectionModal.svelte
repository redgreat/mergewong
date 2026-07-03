<script>
  export let open = false;
  export let editing = false;
  export let form = {};
  export let onClose = () => {};
  export let onSave = () => {};

  function changeType(event) {
    const defaults = { mysql: 3306, postgres: 5432, sqlserver: 1433, oracle: 1521 };
    form.type = event.currentTarget.value;
    form.port = defaults[form.type];
    if (form.type === "mysql" && !form.charset) form.charset = "utf8mb4";
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
        </label>
        <label>
          端口
          <input type="number" min="1" max="65535" bind:value={form.port} />
        </label>
        <label>
          数据库名称
          <input type="text" bind:value={form.database} placeholder="填写 database / schema 所属数据库" autocomplete="off" />
        </label>
        <label>
          字符集
          <input type="text" bind:value={form.charset} placeholder="utf8mb4" />
        </label>
        <label>
          用户名
          <input type="text" bind:value={form.username} placeholder={form.usage === "source" ? "建议使用只读账号" : "请输入数据库账号"} autocomplete="username" />
        </label>
        <label>
          密码
          <input type="password" bind:value={form.password} placeholder={editing ? "留空表示不修改" : "请输入数据库密码"} autocomplete="new-password" />
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
        <button on:click={onSave}>{editing ? "保存修改" : "创建连接"}</button>
        <button class="ghost" on:click={onClose}>取消</button>
      </div>
    </div>
  </div>
{/if}
