<script>
  import { onMount } from "svelte";
  import { ArrowLeft, Database, Gauge, RefreshCw, RotateCw, ShieldAlert, Workflow, X } from "lucide-svelte";
  import { request } from "../api.js";
  export let task = {};
  export let token = "";
  export let canManage = false;
  export let onBack = () => {};
  export let onRefresh = () => {};
  const stateText = (state) => ({ pending:"等待初始化", initializing:"全量初始化", snapshot_completed:"全量完成", catching_up:"增量追数", active:"同步中", failed:"失败" }[state] || state || "等待初始化");
  const runtimeText = (state) => ({ pending:"待预检查", initializing:"全量初始化", catching_up:"增量追数", cdc_running:"增量同步中", paused:"暂停", stopped:"停止", completed:"完成", failed:"失败" }[state] || state);
  const jobText = (status) => ({ running:"执行中", canceling:"取消中", canceled:"已取消", success:"完成", failed:"失败" }[status] || status || "-");
  const jobTypeText = (type) => ({ compare:"全量对比", repair:"补数" }[type] || type);
  const diffTypeText = (type) => ({ missing_target:"目标缺少", missing_source:"源端缺少", mismatch:"字段不一致" }[type] || type || "-");
  const delayText = (seconds=0) => `${(seconds * 1000).toLocaleString()} ms`;
  let repairJobs = [];
  let repairDiffs = [];
  let repairError = "";
  let diffError = "";
  let diffJob = null;
  let diffPage = 1;
  let diffTotal = 0;
  const diffPageSize = 10;
  let metricPoints = [];
  let metricError = "";
  let metricFrom = "";
  let metricTo = "";
  let loadedMetricTaskId = 0;
  let cutoffColumn = "LastUpdateTime";
  let cutoffTime = "";
  let timeColumns = [];
  let cutoffColumnError = "";
  let loadedCutoffTaskId = 0;
  let repairBusy = false;
	$: snapshotTotal = (task.task_tables || []).reduce((sum, table) => sum + Number(table.snapshot_total || 0), 0);
	$: snapshotProcessed = (task.task_tables || []).reduce((sum, table) => sum + Number(table.snapshot_processed || 0), 0);
	$: overallPercent = snapshotTotal > 0 ? Math.min(100, snapshotProcessed * 100 / snapshotTotal) : ((task.task_tables || []).every((table) => table.sync_state === "active") ? 100 : 0);
  $: runningJob = repairJobs.find((job) => job.status === "running" || job.status === "canceling");
  $: diffTotalPages = Math.max(1, Math.ceil(diffTotal / diffPageSize));
  $: maxDelay = Math.max(1, ...metricPoints.map((point) => Number(point.delay_seconds || 0)));
  $: maxRows = Math.max(1, ...metricPoints.map((point) => metricRowTotal(point)));
  $: delayPolyline = metricPoints.map((point, index) => `${chartX(index)},${chartY(Number(point.delay_seconds || 0), maxDelay)}`).join(" ");
  $: if (task.id && token && loadedMetricTaskId !== task.id) {
    loadedMetricTaskId = task.id;
    setMetricRange("24h", false);
    loadMetrics();
  }
  $: if (task.id && token && loadedCutoffTaskId !== task.id) {
    loadedCutoffTaskId = task.id;
    cutoffTime = toLocalDateTimeInput(new Date());
    loadCutoffColumns();
  }
  function toLocalDateTimeInput(date) {
    const pad = (value) => String(value).padStart(2, "0");
    return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
  }
  function normalizeCutoffTime(value) {
    if (!value) return "";
    const normalized = value.replace("T", " ");
    return normalized.length === 16 ? `${normalized}:00` : normalized;
  }
  function setMetricRange(range, refresh = true) {
    const now = new Date();
    const from = new Date(now);
    if (range === "7d") from.setDate(from.getDate() - 7);
    else if (range === "30d") from.setDate(from.getDate() - 30);
    else from.setHours(from.getHours() - 24);
    metricFrom = toLocalDateTimeInput(from);
    metricTo = toLocalDateTimeInput(now);
    if (refresh) loadMetrics();
  }
  async function loadMetrics() {
    if (!task.id || !token) return;
    try {
      metricPoints = await request(`/api/sync/tasks/${task.id}/metrics`, { token, params: { from: normalizeCutoffTime(metricFrom), to: normalizeCutoffTime(metricTo) } });
      metricError = "";
    } catch (err) {
      metricError = err.message;
      metricPoints = [];
    }
  }
  function chartX(index) {
    if (metricPoints.length <= 1) return 24;
    return 24 + (index * 552) / (metricPoints.length - 1);
  }
  function chartY(value, max) {
    return 138 - (Number(value || 0) * 108) / max;
  }
  function metricRowTotal(point) {
    return Number(point.insert_rows || 0) + Number(point.update_rows || 0) + Number(point.delete_rows || 0) + Number(point.read_rows || 0);
  }
  function metricTime(value) {
    return value ? new Date(value).toLocaleString() : "-";
  }
  function compactNumber(value) {
    const num = Number(value || 0);
    if (num >= 1000000) return `${(num / 1000000).toFixed(1)}m`;
    if (num >= 1000) return `${(num / 1000).toFixed(1)}k`;
    return String(num);
  }
  const columnName = (column) => column.Field || column.field || column.COLUMN_NAME || column.column_name || "";
  const columnType = (column) => column.Type || column.type || column.DATA_TYPE || column.data_type || "";
  function preferredTimeColumn(columns) {
    return columns.find((name) => name === "LastUpdateTime")
      || columns.find((name) => name === "UpdatedAt" || name === "UpdateTime" || name === "updated_at")
      || columns[0]
      || cutoffColumn;
  }
  async function loadCutoffColumns() {
    const firstTable = task.task_tables?.[0]?.source_table;
    if (!firstTable || !task.source_db) return;
    try {
      const schema = await request(`/api/db/${encodeURIComponent(task.source_db)}/table/${encodeURIComponent(firstTable)}/schema`, { token });
      const columns = (schema || []).filter((column) => /(date|time|timestamp)/i.test(columnType(column))).map(columnName).filter(Boolean);
      timeColumns = columns;
      cutoffColumnError = "";
      if (columns.length > 0 && !columns.includes(cutoffColumn)) {
        cutoffColumn = preferredTimeColumn(columns);
      }
    } catch (err) {
      cutoffColumnError = err.message;
    }
  }
  async function loadRepairJobs() {
    if (!task.id || !token) return;
    try { repairJobs = await request(`/api/sync/tasks/${task.id}/repair/jobs`, { token }); repairError = ""; }
    catch (err) { repairError = err.message; }
  }
  function canRepairJob(job) {
    return canManage && job.job_type === "compare" && job.status === "success" && Number(job.diff_rows || 0) > 0;
  }
  function valueText(value) {
    if (value === null || value === undefined) return "NULL";
    if (typeof value === "object") return JSON.stringify(value);
    return String(value);
  }
  function changedFields(diff) {
    return (diff.fields || []).filter((field) => !field.equal);
  }
  async function openDiffs(job, page = 1) {
    if (!job || Number(job.diff_rows || 0) <= 0) return;
    diffJob = job;
    diffPage = page;
    try {
      const result = await request(`/api/sync/repair/jobs/${job.id}/diffs`, { token, params: { page, page_size: diffPageSize } });
      repairDiffs = result.data || [];
      diffTotal = result.total || 0;
      diffError = "";
    } catch (err) {
      diffError = err.message;
      repairDiffs = [];
      diffTotal = 0;
    }
  }
  async function startCompare() {
    if (!task.id || repairBusy) return;
    repairBusy = true;
    try {
      const body = { cutoff_column: cutoffColumn.trim(), cutoff_time: normalizeCutoffTime(cutoffTime) };
      await request(`/api/sync/tasks/${task.id}/repair/compare`, { method: "POST", token, body });
      await loadRepairJobs();
      onRefresh();
    } catch (err) { repairError = err.message; }
    finally { repairBusy = false; }
  }
  async function startRepair(job) {
    if (!job || repairBusy) return;
    repairBusy = true;
    try {
      await request(`/api/sync/tasks/${task.id}/repair/jobs/${job.id}/apply`, { method: "POST", token });
      await loadRepairJobs();
      onRefresh();
    } catch (err) { repairError = err.message; }
    finally { repairBusy = false; }
  }
  async function cancelRepair(job) {
    if (!job || repairBusy) return;
    repairBusy = true;
    try {
      await request(`/api/sync/repair/jobs/${job.id}/cancel`, { method: "POST", token });
      await loadRepairJobs();
    } catch (err) { repairError = err.message; }
    finally { repairBusy = false; }
  }
	onMount(() => {
    loadRepairJobs();
    loadMetrics();
    let ticks = 0;
    const timer = setInterval(() => { onRefresh(); loadRepairJobs(); ticks += 1; if (ticks % 15 === 0) loadMetrics(); }, 2000);
    return () => clearInterval(timer);
  });
</script>

<section class="task-detail-page">
  <div class="detail-heading"><div><button class="ghost icon-text" on:click={onBack}><ArrowLeft size={16}/>返回任务</button><h2>{task.name}</h2><p>{task.source_db} → {task.target_db}</p></div><button class="ghost icon-text" on:click={onRefresh}><RefreshCw size={15}/>刷新</button></div>
  <div class="metric-grid">
    <div class="metric-card"><span><Workflow size={16}/>运行状态</span><strong>{runtimeText(task.runtime_status)}</strong><small>{task.last_run_message || "-"}</small></div>
    <div class="metric-card"><span><Gauge size={16}/>同步延迟</span><strong>{delayText(task.delay_seconds)}</strong><small>{(task.rows_per_second || 0).toFixed(1)} 行/秒</small></div>
	<div class="metric-card"><span><Database size={16}/>全量初始化进度</span><strong>{overallPercent.toFixed(1)}%</strong><small>{snapshotProcessed} / {snapshotTotal} 行</small></div>
    <div class="metric-card"><span>Binlog 位点</span><strong class="position-text">{task.cdc_checkpoint?.binlog_file || "-"}</strong><small>{task.cdc_checkpoint?.binlog_position || "-"}</small></div>
  </div>
  <section class="workspace-panel detail-section trend-section">
    <div class="card-header">
      <div><h2>运行趋势</h2><p>保留最近 30 天的同步延迟、读取和增改删行数。</p></div>
      <div class="header-actions metric-range-actions">
        <button class="ghost" on:click={() => setMetricRange("24h")}>24小时</button>
        <button class="ghost" on:click={() => setMetricRange("7d")}>7天</button>
        <button class="ghost" on:click={() => setMetricRange("30d")}>30天</button>
        <button class="ghost icon-text" on:click={loadMetrics}><RefreshCw size={15}/>查询</button>
      </div>
    </div>
    <div class="metric-query-row">
      <label>开始时间<input type="datetime-local" step="1" bind:value={metricFrom} /></label>
      <label>结束时间<input type="datetime-local" step="1" bind:value={metricTo} /></label>
    </div>
    {#if metricError}<div class="inline-error">{metricError}</div>{/if}
    {#if metricPoints.length === 0}
      <div class="empty-state trend-empty"><strong>暂无历史指标</strong><p>增量同步产生新事务后，会按分钟写入趋势数据。</p></div>
    {:else}
      <div class="trend-grid">
        <div class="trend-card">
          <div class="trend-card-head"><strong>同步延迟</strong><span>峰值 {delayText(maxDelay)}</span></div>
          <svg viewBox="0 0 600 160" class="trend-chart" role="img" aria-label="同步延迟趋势">
            <line x1="24" y1="138" x2="576" y2="138" />
            <line x1="24" y1="30" x2="24" y2="138" />
            <polyline points={delayPolyline} />
          </svg>
        </div>
        <div class="trend-card">
          <div class="trend-card-head"><strong>行数变化</strong><span>峰值 {compactNumber(maxRows)} 行</span></div>
          <svg viewBox="0 0 600 160" class="trend-chart bar-chart" role="img" aria-label="增改删读取行数">
            <line x1="24" y1="138" x2="576" y2="138" />
            {#each metricPoints as point, index}
              {@const total = metricRowTotal(point)}
              {@const x = chartX(index) - 3}
              {@const readH = (Number(point.read_rows || 0) * 108) / maxRows}
              {@const insertH = (Number(point.insert_rows || 0) * 108) / maxRows}
              {@const updateH = (Number(point.update_rows || 0) * 108) / maxRows}
              {@const deleteH = (Number(point.delete_rows || 0) * 108) / maxRows}
              {#if total > 0}
                <rect class="read" x={x} y={138 - readH} width="6" height={readH} />
                <rect class="insert" x={x} y={138 - readH - insertH} width="6" height={insertH} />
                <rect class="update" x={x} y={138 - readH - insertH - updateH} width="6" height={updateH} />
                <rect class="delete" x={x} y={138 - readH - insertH - updateH - deleteH} width="6" height={deleteH} />
              {/if}
            {/each}
          </svg>
          <div class="trend-legend"><span class="read">读取</span><span class="insert">新增</span><span class="update">更新</span><span class="delete">删除</span></div>
        </div>
      </div>
      <div class="trend-foot">范围：{metricTime(metricPoints[0]?.time)} 至 {metricTime(metricPoints[metricPoints.length - 1]?.time)}</div>
    {/if}
  </section>
  <section class="workspace-panel detail-section"><div class="card-header"><div><h2>同步进度</h2><p>新增表会先独立初始化并追平主链路，再自动合并。</p></div></div>
    <table class="data-table"><thead><tr><th>源表</th><th>目标表</th><th>阶段</th><th>初始化进度</th><th>已初始化 / 总行数</th><th>说明</th></tr></thead><tbody>
      {#each task.task_tables || [] as table}<tr><td>{table.source_table}</td><td>{table.target_table}</td><td><span class={`pill ${table.sync_state === "failed" ? "danger" : table.sync_state === "active" ? "success" : "muted"}`}>{stateText(table.sync_state)}</span></td><td><div class="progress-cell"><div class="progress-track"><span style={`width:${Math.min(100, table.progress_percent || 0)}%`}></span></div><strong>{(table.progress_percent || 0).toFixed(1)}%</strong></div></td><td>{table.snapshot_processed || 0} / {table.snapshot_total || 0}</td><td>{table.progress_message || "-"}</td></tr>{/each}
    </tbody></table>
  </section>
  <section class="workspace-panel detail-section"><div class="card-header"><div><h2>同步信息</h2></div></div><div class="detail-info-grid"><div><span>同步类型</span><strong>{task.sync_type === "full_cdc" ? "全量 + CDC" : task.sync_type === "cdc" ? "Binlog CDC" : "全量"}</strong></div><div><span>最近成功</span><strong>{task.last_success_at ? new Date(task.last_success_at).toLocaleString() : "-"}</strong></div><div><span>预警发送群</span><strong>{task.alert_channel?.name || "未配置"}</strong></div><div><span>当前阶段开始</span><strong>{task.phase_started_at ? new Date(task.phase_started_at).toLocaleString() : "-"}</strong></div></div></section>
  <section class="workspace-panel detail-section">
    <div class="card-header">
      <div><h2>数据修复</h2><p>按当前字段映射和忽略字段执行源端到目标端的一致性对比与补数。</p></div>
      {#if canManage}
        <div class="header-actions">
          {#if runningJob}<button class="ghost icon-text" disabled={repairBusy} on:click={() => cancelRepair(runningJob)}><X size={15}/>取消</button>{/if}
          <button class="ghost icon-text" disabled={repairBusy || !!runningJob} on:click={startCompare}><ShieldAlert size={15}/>全量对比</button>
        </div>
      {/if}
    </div>
    {#if canManage}
      <div class="repair-toolbar">
        <label>截止字段
          {#if timeColumns.length > 0}
            <select bind:value={cutoffColumn}>{#each timeColumns as column}<option value={column}>{column}</option>{/each}</select>
          {:else}
            <input bind:value={cutoffColumn} placeholder="例如 LastUpdateTime" />
          {/if}
        </label>
        <label>截止时间<input type="datetime-local" step="1" bind:value={cutoffTime} /></label>
      </div>
      {#if cutoffColumnError}<div class="inline-error">{cutoffColumnError}</div>{/if}
    {/if}
    {#if repairError}<div class="inline-error">{repairError}</div>{/if}
    <table class="data-table repair-table">
      <thead><tr><th>类型</th><th>状态</th><th>进度</th><th>差异</th><th>已补数</th><th>说明</th><th>开始时间</th>{#if canManage}<th>操作</th>{/if}</tr></thead>
      <tbody>
        {#if repairJobs.length === 0}<tr class="empty-row repair-empty-row"><td colspan={canManage ? 8 : 7}><div class="empty-state repair-empty"><span class="empty-icon"><ShieldAlert size={24} /></span><strong>暂无数据修复任务</strong><p>发起全量对比后，可以根据差异一键补数。</p></div></td></tr>{/if}
        {#each repairJobs as job}
          <tr>
            <td>{jobTypeText(job.job_type)}</td>
            <td><span class={`pill ${job.status === "failed" ? "danger" : job.status === "success" ? "success" : "muted"}`}>{jobText(job.status)}</span></td>
            <td>{(job.progress_percent || 0).toFixed(1)}%</td>
            <td>{#if Number(job.diff_rows || 0) > 0}<button class="link-button" on:click={() => openDiffs(job)}>{job.diff_rows}</button>{:else}0{/if}</td>
            <td>{job.repaired_rows || 0}</td>
            <td>{job.error_detail || job.message || "-"}</td>
            <td>{job.started_at ? new Date(job.started_at).toLocaleString() : "-"}</td>
            {#if canManage}<td>{#if canRepairJob(job)}<button class="ghost icon-text" disabled={repairBusy || !!runningJob} on:click={() => startRepair(job)}><RotateCw size={14}/>补这次</button>{:else}-{/if}</td>{/if}
          </tr>
        {/each}
      </tbody>
    </table>
  </section>
</section>

{#if diffJob}
  <div class="modal-layer">
    <button class="modal-backdrop" aria-label="关闭" on:click={() => (diffJob = null)}></button>
    <div class="modal diff-modal">
      <div class="modal-header">
        <div><h3>差异明细</h3><p>{jobTypeText(diffJob.job_type)} · {diffJob.started_at ? new Date(diffJob.started_at).toLocaleString() : "-"}</p></div>
        <button class="ghost icon" on:click={() => (diffJob = null)}><X size={17} /></button>
      </div>
      {#if diffError}<div class="inline-error modal-error">{diffError}</div>{/if}
      <table class="data-table diff-table">
        <thead><tr><th>主键</th><th>类型</th><th>状态</th><th>字段差异</th></tr></thead>
        <tbody>
          {#if repairDiffs.length === 0}<tr class="empty-row"><td colspan="4"><div class="empty-state"><strong>暂无差异明细</strong></div></td></tr>{/if}
          {#each repairDiffs as diff}
            <tr>
              <td><strong>{diff.source_pk}</strong><span class="cell-sub">{diff.source_table} → {diff.target_table}</span></td>
              <td>{diffTypeText(diff.diff_type)}</td>
              <td>{diff.status}</td>
              <td>
                {#if changedFields(diff).length > 0}
                  <div class="field-diff-list">
                    {#each changedFields(diff) as field}
                      <div class="field-diff-row">
                        <strong>{field.source_field} → {field.target_field}</strong>
                        <span>源：{valueText(field.source_value)}</span>
                        <span>目标：{valueText(field.target_value)}</span>
                      </div>
                    {/each}
                  </div>
                {:else}
                  <span class="cell-sub">按当前字段映射回查已一致，可能是旧对比结果或数据已被补齐。</span>
                {/if}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
      <div class="pager">
        <button class="ghost" disabled={diffPage <= 1} on:click={() => openDiffs(diffJob, diffPage - 1)}>上一页</button>
        <span>{diffPage} / {diffTotalPages}</span>
        <button class="ghost" disabled={diffPage >= diffTotalPages} on:click={() => openDiffs(diffJob, diffPage + 1)}>下一页</button>
      </div>
    </div>
  </div>
{/if}
