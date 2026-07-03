<script>
  import { Check, CircleHelp, Search, X } from "lucide-svelte";
  import { request } from "../api.js";

  export let open = false;
  export let editing = false;
  export let form = {};
  export let connections = [];
  export let alertChannels = [];
  export let token = "";
  export let precheckResult = null;
  export let saving = false;
  export let onClose = () => {};
  export let onSave = () => {};

  let step = 1;
  let helpOpen = "";
  let availableTables = [];
  let tableSearch = "";
  let loadedConnection = "";
  let loadingTables = false;
  let tableError = "";
  $: if (!open) { step = 1; helpOpen = ""; }
  $: if (open && precheckResult) step = 4;
  $: stepOneReady = !!(form.name?.trim() && form.source_db && form.target_db);
  $: stepTwoReady = !!form.table_mappings?.length && form.table_mappings.every((table) => table.source_table?.trim() && table.target_table?.trim());
  $: filteredTables = availableTables.filter((table) => table.toLowerCase().includes(tableSearch.trim().toLowerCase()));
  $: if (open && step === 2 && form.source_db && loadedConnection !== form.source_db) loadSourceTables();

  async function loadSourceTables() {
    const connectionName = form.source_db;
    loadingTables = true;
    tableError = "";
    try {
      availableTables = await request(`/api/db/${encodeURIComponent(connectionName)}/tables`, { token });
      loadedConnection = connectionName;
    } catch (error) {
      tableError = error.message;
      availableTables = [];
      loadedConnection = connectionName;
    } finally {
      loadingTables = false;
    }
  }

  function isSelected(tableName) {
    return (form.table_mappings || []).some((table) => table.source_table === tableName);
  }

  function toggleTable(tableName) {
    if (isSelected(tableName)) {
      form.table_mappings = form.table_mappings.filter((table) => table.source_table !== tableName);
    } else {
      form.table_mappings = [...(form.table_mappings || []), { source_table: tableName, target_table: tableName, field_mapping: {} }];
    }
  }

  function removeTable(tableName) {
    form.table_mappings = form.table_mappings.filter((table) => table.source_table !== tableName);
  }

  function toggleHelp(name) {
    helpOpen = helpOpen === name ? "" : name;
  }

  function changeSourceDB(event) {
    const nextSource = event.currentTarget.value;
    if (nextSource !== form.source_db) {
      form.source_db = nextSource;
      form.table_mappings = [];
      availableTables = [];
      loadedConnection = "";
    }
  }

  function handleEscape(event) {
    if (event.key !== "Escape" || !open) return;
    if (helpOpen) helpOpen = "";
    else onClose();
  }
</script>

<svelte:window on:keydown={handleEscape} on:click={() => (helpOpen = "")} />
{#if open}
  <div class="modal-layer">
    <button class="modal-backdrop" type="button" aria-label="关闭" on:click={onClose}></button>
    <div class="modal task-modal task-wizard" role="dialog" aria-modal="true">
      <div class="modal-header">
        <div><h3>{editing ? "编辑同步任务" : "新增同步任务"}</h3></div>
        <button class="ghost icon" aria-label="关闭" on:click={onClose}>✕</button>
      </div>

      <div class="wizard-steps" aria-label="任务配置步骤">
        {#each [[1, "基础配置"], [2, "同步对象"], [3, "执行与预警"], [4, "预检查"]] as item}
          <button type="button" disabled={(item[0] >= 2 && !stepOneReady) || (item[0] >= 3 && !stepTwoReady) || (item[0] === 4 && !precheckResult)} class:active={step === item[0]} class:done={step > item[0]} on:click={() => (step = item[0])}>
            <span>{#if step > item[0]}<Check size={14} />{:else}{item[0]}{/if}</span>{item[1]}
          </button>
        {/each}
      </div>

      <div class="wizard-body">
        {#if step === 1}
          <div class="form-grid wizard-grid">
            <label class="full">任务名称<input type="text" bind:value={form.name} placeholder="例如：订单数据同步" disabled={editing} /></label>
            <label>源库连接<select value={form.source_db} on:change={changeSourceDB} disabled={editing}><option value="">请选择源端连接</option>{#each connections.filter((connection) => connection.usage === "source" || connection.usage === "both" || !connection.usage) as connection}<option value={connection.name}>{connection.name}</option>{/each}</select></label>
            <label>目标库连接<select bind:value={form.target_db} disabled={editing}><option value="">请选择目标端连接</option>{#each connections.filter((connection) => connection.usage === "target" || connection.usage === "both" || !connection.usage) as connection}<option value={connection.name}>{connection.name}</option>{/each}</select></label>
            <label>同步类型<select bind:value={form.sync_type} disabled={editing}><option value="full_cdc">全量初始化 + Binlog CDC</option><option value="cdc">仅 Binlog CDC</option><option value="full">仅全量初始化</option></select></label>
          </div>
        {:else if step === 2}
          <div class="object-picker">
            <section class="object-panel">
              <div class="object-panel-header"><strong>源表</strong><button type="button" class="ghost" on:click={loadSourceTables}>刷新</button></div>
              <div class="object-search"><Search size={16} /><input aria-label="搜索源表" bind:value={tableSearch} placeholder="搜索表名" /></div>
              <div class="object-list">
                {#if loadingTables}<div class="object-empty">正在读取表清单…</div>
                {:else if tableError}<div class="object-error">{tableError}</div>
                {:else if filteredTables.length === 0}<div class="object-empty">没有可选表</div>
                {:else}{#each filteredTables as table}<label class="object-row"><input type="checkbox" checked={isSelected(table)} on:change={() => toggleTable(table)} /><span>{table}</span></label>{/each}{/if}
              </div>
            </section>
            <section class="object-panel selected-panel">
              <div class="object-panel-header"><strong>已选同步对象</strong><span>{form.table_mappings?.length || 0} 张表</span></div>
              <div class="selected-table-head"><span>源表</span><span>目标表名</span><span></span></div>
              <div class="selected-table-list">
                {#if !(form.table_mappings || []).length}<div class="object-empty">从左侧选择需要同步的表</div>
                {:else}{#each form.table_mappings as table}<div class="selected-table-row"><span title={table.source_table}>{table.source_table}</span><input aria-label={`${table.source_table} 的目标表名`} bind:value={table.target_table} /><button type="button" class="icon-button" aria-label={`移除 ${table.source_table}`} on:click={() => removeTable(table.source_table)}><X size={15} /></button></div>{/each}{/if}
              </div>
            </section>
          </div>
        {:else if step === 3}
          <div class="wizard-section-title">
            <h4>运行方式</h4>
            <div class="help-wrap"><button class="help-button" type="button" aria-label="查看运行方式说明" on:click|stopPropagation={() => toggleHelp("schedule")}><CircleHelp size={16} /></button>{#if helpOpen === "schedule"}<div class="help-popover">全量+CDC 会先记录 Binlog 位点，再初始化存量数据，最后从该位点持续消费增删改事件；无需 Cron。</div>{/if}</div>
          </div>
          <div class="mode-summary"><strong>{form.sync_type === "full_cdc" ? "全量初始化后持续同步" : form.sync_type === "cdc" ? "从当前位点开始持续同步" : "执行一次全量初始化"}</strong></div>

          <div class="wizard-section-title alert-title">
            <h4>预警策略</h4>
            <div class="help-wrap"><button class="help-button" type="button" aria-label="查看预警策略说明" on:click|stopPropagation={() => toggleHelp("alert")}><CircleHelp size={16} /></button>{#if helpOpen === "alert"}<div class="help-popover">运行延迟表示单次执行持续过久；停止表示计划任务长时间未再次启动；报错预警在执行失败时发送。</div>{/if}</div>
          </div>
          <div class="form-grid wizard-grid compact-grid">
            <label>预警发送方<select bind:value={form.alert_channel_id}><option value="">不发送预警</option>{#each alertChannels as channel}<option value={channel.id}>{channel.name}</option>{/each}</select></label>
			<label>同步延迟阈值（ms）<input type="number" min="0" bind:value={form.alert_delay_ms} placeholder="默认5000" /></label>
            {#if !editing}
            <label>停止阈值（分钟）<input type="number" min="0" bind:value={form.alert_stopped_minutes} disabled={form.sync_type === "full"} /></label>
            <label>重复提醒间隔（分钟）<input type="number" min="1" bind:value={form.alert_cooldown_minutes} /></label>
            <label class="checkbox-label"><input type="checkbox" bind:checked={form.alert_on_error} />执行报错时立即预警</label>
            {/if}
          </div>
        {:else}
          <div class="precheck-summary" class:passed={precheckResult?.passed}>
            <strong>{precheckResult?.passed ? "预检查通过" : "预检查未通过"}</strong>
          </div>
          <div class="precheck-list">
            {#each precheckResult?.items || [] as item}
              <div class={`precheck-item ${item.level}`}><span>{item.level === "success" ? "通过" : item.level === "warning" ? "提醒" : "阻断"}</span><strong>{item.object}</strong><p>{item.message}</p></div>
            {/each}
          </div>
        {/if}
      </div>

      <div class="wizard-actions">
        <button class="ghost" type="button" on:click={onClose}>取消</button>
        <div>
          {#if step > 1 && step < 4}<button class="ghost" type="button" on:click={() => (step -= 1)}>上一步</button>{/if}
          {#if step < 3}<button type="button" disabled={(step === 1 && !stepOneReady) || (step === 2 && !stepTwoReady)} on:click={() => (step += 1)}>下一步</button>{:else if step === 3}<button disabled={saving} on:click={onSave}>{saving ? "正在预检查…" : "保存并预检查"}</button>{:else}<button on:click={onClose}>完成</button>{/if}
        </div>
      </div>
    </div>
  </div>
{/if}
