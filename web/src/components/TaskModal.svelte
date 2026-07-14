<script>
  import { Check, ChevronDown, CircleHelp, Plus, Search, Trash2, X } from "lucide-svelte";
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
  let expandedMappingTable = "";
  let columnCache = {};
  let columnLoading = {};
  let columnErrors = {};
  let errors = {};
  $: if (!open) { step = 1; helpOpen = ""; errors = {}; expandedMappingTable = ""; columnCache = {}; columnLoading = {}; columnErrors = {}; }
  $: if (open && precheckResult) step = 4;
  $: stepOneReady = !!(form.name?.trim() && form.source_db && form.target_db);
  $: stepTwoReady = !!form.table_mappings?.length && form.table_mappings.every((table) => table.source_table?.trim() && table.target_table?.trim());
  $: filteredTables = availableTables.filter((table) => table.toLowerCase().includes(tableSearch.trim().toLowerCase()));
  $: effectiveBatchSize = Number(form.sync_batch_size || 0);
  $: effectiveTableWorkers = Number(form.snapshot_table_workers || 0);
  $: effectiveShardWorkers = Number(form.snapshot_shard_workers || 0);
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

  function schemaKey(connection, table) {
    return `${connection}::${table}`;
  }

  function sourceColumns(table) {
    return columnCache[schemaKey(form.source_db, table.source_table)] || [];
  }

  function ignoredFields(table) {
    return table.ignored_fields || [];
  }

  async function loadColumns(connection, tableName) {
    if (!connection || !tableName) return [];
    const key = schemaKey(connection, tableName);
    if (columnCache[key]) return columnCache[key];
    columnLoading = { ...columnLoading, [key]: true };
    columnErrors = { ...columnErrors, [key]: "" };
    try {
      const schema = await request(`/api/db/${encodeURIComponent(connection)}/table/${encodeURIComponent(tableName)}/schema`, { token });
      const columns = (schema || []).map((column) => column.Field || column.field || column.COLUMN_NAME || column.column_name).filter(Boolean);
      columnCache = { ...columnCache, [key]: columns };
      return columns;
    } catch (error) {
      columnErrors = { ...columnErrors, [key]: error.message };
      return [];
    } finally {
      columnLoading = { ...columnLoading, [key]: false };
    }
  }

  async function toggleMappingPanel(table) {
    expandedMappingTable = expandedMappingTable === table.source_table ? "" : table.source_table;
    if (expandedMappingTable) {
      await loadColumns(form.source_db, table.source_table);
    }
  }

  function mappingEntries(table) {
    return Object.entries(table.field_mapping || {});
  }

  function ignoredEntries(table) {
    return ignoredFields(table);
  }

  function updateMappingTarget(table, source, target) {
    table.field_mapping = { ...(table.field_mapping || {}), [source]: target };
    form.table_mappings = [...form.table_mappings];
  }

  function removeFieldMapping(table, source) {
    const next = { ...(table.field_mapping || {}) };
    delete next[source];
    table.field_mapping = next;
    form.table_mappings = [...form.table_mappings];
  }

  function chooseMappingSource(table, event) {
    const source = event.currentTarget.value;
    table.new_mapping_source = source;
    table.new_mapping_target = source;
    form.table_mappings = [...form.table_mappings];
  }

  function addFieldMapping(table) {
    const source = table.new_mapping_source;
    const target = (table.new_mapping_target || source || "").trim();
    if (!source || !target) return;
    table.field_mapping = { ...(table.field_mapping || {}), [source]: target };
    table.new_mapping_source = "";
    table.new_mapping_target = "";
    form.table_mappings = [...form.table_mappings];
  }

  function chooseIgnoredField(table, event) {
    table.new_ignored_field = event.currentTarget.value;
    form.table_mappings = [...form.table_mappings];
  }

  function addIgnoredField(table) {
    const field = table.new_ignored_field;
    if (!field) return;
    table.ignored_fields = [...new Set([...(table.ignored_fields || []), field])];
    const nextMapping = { ...(table.field_mapping || {}) };
    delete nextMapping[field];
    table.field_mapping = nextMapping;
    table.new_ignored_field = "";
    form.table_mappings = [...form.table_mappings];
  }

  function removeIgnoredField(table, field) {
    table.ignored_fields = (table.ignored_fields || []).filter((value) => value !== field);
    form.table_mappings = [...form.table_mappings];
  }

  function confirmTypeMismatch(item) {
    const table = (form.table_mappings || []).find((mapping) => `${mapping.source_table} → ${mapping.target_table}` === item.object);
    if (!table || !item.confirm_key) return;
    const confirmed = window.confirm(`确认忽略该校验项吗？\n\n${item.message}\n\n字段类型不一致可能导致同步写入失败、数据截断或目标数据不一致。请确认源端到目标端类型兼容后再继续。`);
    if (!confirmed) return;
    table.type_mismatch_ignores = [...new Set([...(table.type_mismatch_ignores || []), item.confirm_key])];
    form.table_mappings = [...form.table_mappings];
    onSave();
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

  function validateStep(currentStep) {
    errors = {};
    if (currentStep === 1) {
      if (!form.name?.trim()) errors.name = "请填写任务名称";
      if (!form.source_db) errors.source_db = "请选择源库连接";
      if (!form.target_db) errors.target_db = "请选择目标库连接";
    }
    if (currentStep === 2) {
      if (!form.table_mappings?.length) errors.tables = "请至少选择一张同步表";
    }
    return Object.keys(errors).length === 0;
  }

  function nextStep() {
    if (!validateStep(step)) return;
    step += 1;
  }

  function handleSave() {
    if (!validateStep(1) || !validateStep(2)) return;
    onSave();
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
            <label class="full">任务名称<input type="text" bind:value={form.name} placeholder="例如：订单数据同步" disabled={editing} />{#if errors.name}<span class="field-error">{errors.name}</span>{/if}</label>
            <label>源库连接<select value={form.source_db} on:change={changeSourceDB} disabled={editing}><option value="">请选择源端连接</option>{#each connections.filter((connection) => connection.usage === "source" || connection.usage === "both" || !connection.usage) as connection}<option value={connection.name}>{connection.name}</option>{/each}</select>{#if errors.source_db}<span class="field-error">{errors.source_db}</span>{/if}</label>
            <label>目标库连接<select bind:value={form.target_db} disabled={editing}><option value="">请选择目标端连接</option>{#each connections.filter((connection) => connection.usage === "target" || connection.usage === "both" || !connection.usage) as connection}<option value={connection.name}>{connection.name}</option>{/each}</select>{#if errors.target_db}<span class="field-error">{errors.target_db}</span>{/if}</label>
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
              <div class="selected-table-head"><span>源表</span><span>目标表名</span><span></span><span></span></div>
              <div class="selected-table-list">
                {#if !(form.table_mappings || []).length}<div class="object-empty">{errors.tables || "从左侧选择需要同步的表"}</div>
                {:else}{#each form.table_mappings as table}<div class="selected-table-item">
                  <div class="selected-table-row">
                    <span title={table.source_table}>{table.source_table}</span>
                    <input aria-label={`${table.source_table} 的目标表名`} bind:value={table.target_table} />
                    <button type="button" class="icon-button" aria-label={`${table.source_table} 字段映射`} on:click={() => toggleMappingPanel(table)}><ChevronDown size={15} class={expandedMappingTable === table.source_table ? "rotated" : ""} /></button>
                    <button type="button" class="icon-button" aria-label={`移除 ${table.source_table}`} on:click={() => removeTable(table.source_table)}><X size={15} /></button>
                  </div>
                  {#if expandedMappingTable === table.source_table}
                    <div class="field-map-panel">
                      {#if columnLoading[schemaKey(form.source_db, table.source_table)]}<div class="mapping-empty">正在读取字段清单...</div>
                      {:else if columnErrors[schemaKey(form.source_db, table.source_table)]}<div class="object-error">{columnErrors[schemaKey(form.source_db, table.source_table)]}</div>
                      {:else}
                        <div class="field-map-add">
                          <select aria-label={`${table.source_table} 的源字段`} value={table.new_mapping_source || ""} on:change={(event) => chooseMappingSource(table, event)}>
                            <option value="">选择源字段</option>
                            {#each sourceColumns(table).filter((column) => !(table.field_mapping || {})[column] && !ignoredFields(table).includes(column)) as column}<option value={column}>{column}</option>{/each}
                          </select>
                          <input aria-label={`${table.source_table} 的目标字段`} bind:value={table.new_mapping_target} placeholder="目标字段名" />
                          <button type="button" class="icon-button" aria-label="添加字段映射" disabled={!table.new_mapping_source} on:click={() => addFieldMapping(table)}><Plus size={15} /></button>
                        </div>
                        <div class="field-map-add ignore-add">
                          <select aria-label={`${table.source_table} 的不同步字段`} value={table.new_ignored_field || ""} on:change={(event) => chooseIgnoredField(table, event)}>
                            <option value="">选择不同步字段</option>
                            {#each sourceColumns(table).filter((column) => !ignoredFields(table).includes(column)) as column}<option value={column}>{column}</option>{/each}
                          </select>
                          <span>该字段不参与预检查和写入</span>
                          <button type="button" class="icon-button" aria-label="添加不同步字段" disabled={!table.new_ignored_field} on:click={() => addIgnoredField(table)}><Plus size={15} /></button>
                        </div>
                        {#if mappingEntries(table).length === 0}<div class="mapping-empty">未配置字段改名，同名字段按原名同步</div>
                        {:else}
                          <div class="field-map-list">
                            {#each mappingEntries(table) as [source, target]}
                              <div class="field-map-row">
                                <span title={source}>{source}</span>
                                <input aria-label={`${source} 的目标字段名`} value={target} on:input={(event) => updateMappingTarget(table, source, event.currentTarget.value)} />
                                <button type="button" class="icon-button" aria-label={`移除字段映射 ${source}`} on:click={() => removeFieldMapping(table, source)}><Trash2 size={14} /></button>
                              </div>
                            {/each}
                          </div>
                        {/if}
                        {#if ignoredEntries(table).length > 0}
                          <div class="field-map-list ignored-list">
                            {#each ignoredEntries(table) as field}
                              <div class="field-map-row ignored-row">
                                <span title={field}>{field}</span>
                                <em>不同步</em>
                                <button type="button" class="icon-button" aria-label={`恢复同步字段 ${field}`} on:click={() => removeIgnoredField(table, field)}><Trash2 size={14} /></button>
                              </div>
                            {/each}
                          </div>
                        {/if}
                      {/if}
                    </div>
                  {/if}
                </div>{/each}{/if}
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
            <h4>初始化资源策略</h4>
            <div class="help-wrap"><button class="help-button" type="button" aria-label="查看初始化资源策略说明" on:click|stopPropagation={() => toggleHelp("tuning")}><CircleHelp size={16} /></button>{#if helpOpen === "tuning"}<div class="help-popover">填写 0 表示自动按服务器规格选择。4C8G 建议保持自动，或手动设置为批大小 1000、表并发 1、分片并发 1，减少 Docker 被打满的风险。</div>{/if}</div>
          </div>
          <div class="resource-summary">
            <div class="resource-card"><strong>{effectiveBatchSize || "自动"}</strong><span>单批读取/写入行数</span></div>
            <div class="resource-card"><strong>{effectiveTableWorkers || "自动"}</strong><span>任务内并行初始化表数</span></div>
            <div class="resource-card"><strong>{effectiveShardWorkers || "自动"}</strong><span>单表分片并行数</span></div>
          </div>
          <div class="form-grid wizard-grid compact-grid resource-grid">
            <label>批大小
              <input type="number" min="0" max="20000" bind:value={form.sync_batch_size} placeholder="0 表示自动" />
              <small>建议 1000，填写 0 使用系统自动配置</small>
            </label>
            <label>表并发
              <input type="number" min="0" max="32" bind:value={form.snapshot_table_workers} placeholder="0 表示自动" />
              <small>同一任务同时初始化的表数量</small>
            </label>
            <label>分片并发
              <input type="number" min="0" max="32" bind:value={form.snapshot_shard_workers} placeholder="0 表示自动" />
              <small>单表拆分后并行读取的分片数</small>
            </label>
          </div>

          <div class="wizard-section-title alert-title">
            <h4>预警策略</h4>
            <div class="help-wrap"><button class="help-button" type="button" aria-label="查看预警策略说明" on:click|stopPropagation={() => toggleHelp("alert")}><CircleHelp size={16} /></button>{#if helpOpen === "alert"}<div class="help-popover">仅增量链路按同步延迟触发预警；全量初始化失败不发送预警。延迟超过阈值后预警，若每次提醒前仍未恢复到阈值内，会在 1 小时、3 小时、6 小时后继续提醒。第 6 小时提醒会提示后续不再重复提醒。</div>{/if}</div>
          </div>
          <div class="form-grid wizard-grid compact-grid">
            <label>预警发送方<select bind:value={form.alert_channel_id}><option value="">不发送预警</option>{#each alertChannels as channel}<option value={channel.id}>{channel.name}</option>{/each}</select></label>
			<label>同步延迟阈值（ms）<input type="number" min="0" bind:value={form.alert_delay_ms} placeholder="默认5000" /></label>
          </div>
        {:else}
          <div class="precheck-summary" class:passed={precheckResult?.passed}>
            <strong>{precheckResult?.passed ? "预检查通过" : "预检查未通过"}</strong>
          </div>
          <div class="precheck-list">
            {#each precheckResult?.items || [] as item}
              <div class={`precheck-item ${item.level}`}><span>{item.level === "success" ? "通过" : item.level === "warning" ? "提醒" : "阻断"}</span><strong>{item.object}</strong><p>{item.message}</p>{#if item.code === "type_mismatch" && item.level === "error"}<button type="button" class="ghost confirm-precheck" on:click={() => confirmTypeMismatch(item)}>确认忽略</button>{/if}</div>
            {/each}
          </div>
        {/if}
      </div>

      <div class="wizard-actions">
        <button class="ghost" type="button" on:click={onClose}>取消</button>
        <div>
          {#if step > 1 && step < 4}<button class="ghost" type="button" on:click={() => (step -= 1)}>上一步</button>{/if}
          {#if step < 3}<button type="button" disabled={(step === 1 && !stepOneReady) || (step === 2 && !stepTwoReady)} on:click={nextStep}>下一步</button>{:else if step === 3}<button disabled={saving} on:click={handleSave}>{saving ? "正在预检查…" : "保存并预检查"}</button>{:else}<button on:click={onClose}>完成</button>{/if}
        </div>
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
