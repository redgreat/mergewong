<script>
  export let open = false;
  export let editing = false;
  export let form = {};
  export let connections = [];
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
          <h3>{editing ? "编辑同步任务" : "新增同步任务"}</h3>
          <p>支持 Cron 定时与字段映射</p>
        </div>
        <button class="ghost icon" on:click={onClose}>✕</button>
      </div>
      <div class="form-grid">
        <label>
          任务名称
          <input type="text" bind:value={form.name} />
        </label>
        <label>
          源库连接
          <select bind:value={form.source_db}>
            <option value="">请选择</option>
            {#each connections as connection}
              <option value={connection.name}>{connection.name}</option>
            {/each}
          </select>
        </label>
        <label>
          源表
          <input type="text" bind:value={form.source_table} />
        </label>
        <label>
          目标库连接
          <select bind:value={form.target_db}>
            <option value="">请选择</option>
            {#each connections as connection}
              <option value={connection.name}>{connection.name}</option>
            {/each}
          </select>
        </label>
        <label>
          目标表
          <input type="text" bind:value={form.target_table} />
        </label>
        <label>
          同步类型
          <select bind:value={form.sync_type}>
            <option value="full">全量</option>
            <option value="incremental">增量</option>
          </select>
        </label>
        <label>
          增量字段
          <input type="text" bind:value={form.incremental_key} disabled={form.sync_type !== "incremental"} />
        </label>
        <label>
          Cron 表达式
          <input type="text" bind:value={form.cron_expression} />
        </label>
        <label class="full">
          字段映射 JSON
          <textarea rows="6" bind:value={form.field_mapping_json}></textarea>
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
        <button on:click={onSave}>{editing ? "保存修改" : "创建任务"}</button>
        <button class="ghost" on:click={onClose}>取消</button>
      </div>
    </div>
  </div>
{/if}
