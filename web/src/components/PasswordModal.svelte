<script>
  import { X } from "lucide-svelte";
  export let open = false;
  export let form = {};
  export let onClose = () => {};
  export let onSave = () => {};
</script>

<svelte:window on:keydown={(event) => event.key === "Escape" && open && onClose()} />
{#if open}
  <div class="modal-layer">
    <button class="modal-backdrop" type="button" aria-label="关闭" on:click={onClose}></button>
    <div class="modal compact-modal" role="dialog" aria-modal="true" aria-label="修改密码">
      <div class="modal-header"><h3>修改密码</h3><button class="icon-button" aria-label="关闭" on:click={onClose}><X size={17} /></button></div>
      <div class="form-grid single-column">
        <label>当前密码<input type="password" bind:value={form.current_password} /></label>
        <label>新密码<input type="password" bind:value={form.new_password} minlength="6" /></label>
        <label>确认新密码<input type="password" bind:value={form.confirm_password} minlength="6" /></label>
      </div>
      <div class="actions"><button on:click={onSave}>确认修改</button><button class="ghost" on:click={onClose}>取消</button></div>
    </div>
  </div>
{/if}
