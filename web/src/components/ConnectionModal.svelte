<script>
  export let open = false;
  export let editing = false;
  export let form = {};
  export let onClose = () => {};
  export let onSave = () => {};
</script>

<svelte:window on:keydown={(event) => event.key === "Escape" && open && onClose()} />
{#if open}
  <div class="modal-layer">
    <button class="modal-backdrop" type="button" aria-label="关闭" on:click={onClose}></button>
    <div class="modal" role="dialog" aria-modal="true">
      <div class="modal-header">
        <div>
          <h3>{editing ? "编辑数据库连接" : "新增数据库连接"}</h3>
          <p>保存后立即生效</p>
        </div>
        <button class="ghost icon" on:click={onClose}>✕</button>
      </div>
      <div class="form-grid">
        <label>
          连接名
          <input type="text" bind:value={form.name} />
        </label>
        <label>
          类型
          <select bind:value={form.type}>
            <option value="mysql">MySQL</option>
            <option value="postgres">PostgreSQL</option>
            <option value="sqlserver">SQL Server</option>
            <option value="oracle">Oracle</option>
          </select>
        </label>
        <label>
          主机
          <input type="text" bind:value={form.host} />
        </label>
        <label>
          端口
          <input type="number" bind:value={form.port} />
        </label>
        <label>
          数据库
          <input type="text" bind:value={form.database} />
        </label>
        <label>
          用户名
          <input type="text" bind:value={form.username} />
        </label>
        <label>
          密码
          <input type="password" bind:value={form.password} placeholder={editing ? "留空则不修改" : ""} />
        </label>
        <label>
          字符集
          <input type="text" bind:value={form.charset} />
        </label>
        <label>
          空闲连接
          <input type="number" bind:value={form.max_idle} />
        </label>
        <label>
          最大连接
          <input type="number" bind:value={form.max_open} />
        </label>
        <label>
          状态
          <select bind:value={form.status}>
            <option value="1">启用</option>
            <option value="0">禁用</option>
          </select>
        </label>
      </div>
      <div class="actions">
        <button on:click={onSave}>{editing ? "保存修改" : "创建连接"}</button>
        <button class="ghost" on:click={onClose}>取消</button>
      </div>
    </div>
  </div>
{/if}
