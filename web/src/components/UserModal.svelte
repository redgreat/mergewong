<script>
  import { X } from "lucide-svelte";
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
    <div class="modal compact-modal" role="dialog" aria-modal="true" aria-label={editing ? "编辑用户" : "新增用户"}>
      <div class="modal-header"><h3>{editing ? "编辑用户" : "新增用户"}</h3><button class="icon-button" aria-label="关闭" on:click={onClose}><X size={17} /></button></div>
      <div class="form-grid single-column">
        <label>用户名<input type="text" bind:value={form.username} disabled={editing} /></label>
        {#if !editing}<label>初始密码<input type="password" bind:value={form.password} minlength="6" /></label>{/if}
        <label>邮箱<input type="email" bind:value={form.email} /></label>
        <label>角色<select bind:value={form.role}><option value="viewer">只读用户</option><option value="admin">管理员</option></select></label>
        <label>状态<select bind:value={form.status}><option value="1">启用</option><option value="0">禁用</option></select></label>
      </div>
      <div class="actions"><button on:click={onSave}>{editing ? "保存修改" : "创建用户"}</button><button class="ghost" on:click={onClose}>取消</button></div>
    </div>
  </div>
{/if}
