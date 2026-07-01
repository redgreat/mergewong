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
    <div class="modal compact-modal" role="dialog" aria-modal="true">
      <div class="modal-header"><div><h3>{editing ? "编辑预警发送群" : "新增预警发送群"}</h3><p>发送方式固定为企业微信机器人。</p></div><button class="ghost icon" on:click={onClose}>✕</button></div>
      <div class="form-grid single-column">
        <label>发送群名称<input type="text" bind:value={form.name} placeholder="例如：运维预警群" /></label>
        <label>企业微信机器人 ID<input type="password" bind:value={form.robot_id} placeholder={editing ? "留空表示不修改" : "填写 webhook 中的 key"} autocomplete="off" /></label>
        <label>状态<select bind:value={form.status}><option value="1">启用</option><option value="0">禁用</option></select></label>
      </div>
      <div class="actions"><button on:click={onSave}>{editing ? "保存修改" : "创建发送群"}</button><button class="ghost" on:click={onClose}>取消</button></div>
    </div>
  </div>
{/if}
