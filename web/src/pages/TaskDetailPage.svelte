<script>
  import { onMount } from "svelte";
  import { ArrowLeft, Database, Gauge, RefreshCw, RotateCw, ShieldAlert, Workflow, X, ChevronLeft, ChevronRight } from "lucide-svelte";
  import { request } from "../api.js";
  export let task = {};
  export let token = "";
  export let canManage = false;
  export let onBack = () => {};
  export let onRefresh = () => {};
  const stateText = (state) => ({ pending:"等待初始化", initializing:"全量初始化", snapshot_completed:"全量完成", catching_up:"增量追数", active:"同步中", failed:"失败" }[state] || state || "等待初始化");
  const runtimeText = (state) => ({ pending:"待预检查", initializing:"全量初始化", catching_up:"增量追数", cdc_running:"增量同步中", paused:"暂停", stopped:"停止", completed:"完成", failed:"失败" }[state] || state);
  const jobText = (status) => ({ running:"执行中", canceling:"取消中", canceled:"已取消", success:"完成", failed:"失败" }[status] || status || "-");
  const jobTypeText = (type, job) => {
    if (type === "repair") return "补数";
    if (type === "compare" && (job?.cutoff_time || job?.cutoff_from)) return "按时间段补数";
    if (type === "compare") return "全量对比";
    return type;
  };
  const diffTypeText = (type) => ({ missing_target:"目标缺少", missing_source:"源端缺少", mismatch:"字段不一致" }[type] || type || "-");
  const delayText = (seconds=0) => {
    if (seconds <= 0) return "0 ms";
    if (seconds < 60) return `${(seconds * 1000).toLocaleString()} ms`;
    if (seconds < 3600) {
      const m = Math.floor(seconds / 60);
      const s = seconds % 60;
      return s > 0 ? `${m} 分 ${s} 秒` : `${m} 分钟`;
    }
    if (seconds < 86400) {
      const h = Math.floor(seconds / 3600);
      const m = Math.floor((seconds % 3600) / 60);
      return m > 0 ? `${h} 小时 ${m} 分` : `${h} 小时`;
    }
    const d = Math.floor(seconds / 86400);
    const h = Math.floor((seconds % 86400) / 3600);
    return h > 0 ? `${d} 天 ${h} 小时` : `${d} 天`;
  };
  const chartLeft = 46;
  const chartRight = 576;
  const chartTop = 20;
  const chartBottom = 126;
  const chartHeight = chartBottom - chartTop;
  const chartWidth = chartRight - chartLeft;
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
  let metricRange = "24h";
  let activeDelayIndex = null;
  let activeRowsIndex = null;
  let loadedMetricTaskId = 0;
  let repairBusy = false;
  let detailRefreshing = false;
  let showCancelConfirm = false;
  let pendingCancelJob = null;
  let showFullCompareConfirm = false;
  let showTimeCompareModal = false;
  let compareTimeFrom = "";
  let compareTimeTo = "";
  let timeCompareError = "";
  let showJobDetail = null;
  // 按时间段补数弹窗配置
  let compareTables = [];
  let compareTablePage = 1;
  let compareTablePageSize = 10;
  let compareAllColumns = {};
  let compareTableIdCounter = 0;
	$: snapshotTotal = (task.task_tables || []).reduce((sum, table) => sum + Number(table.snapshot_total || 0), 0);
	$: snapshotProcessed = (task.task_tables || []).reduce((sum, table) => sum + Number(table.snapshot_processed || 0), 0);
	$: overallPercent = snapshotTotal > 0 ? Math.min(100, snapshotProcessed * 100 / snapshotTotal) : ((task.task_tables || []).every((table) => table.sync_state === "active") ? 100 : 0);
  $: runningJob = repairJobs.find((job) => job.status === "running" || job.status === "canceling");
  $: diffTotalPages = Math.max(1, Math.ceil(diffTotal / diffPageSize));
  $: maxDelay = Math.max(1, ...metricPoints.map((point) => Number(point.delay_seconds || 0)));
  $: maxRows = Math.max(1, ...metricPoints.map((point) => Number(point.total_rows || metricRowTotal(point))));
  $: delayPolyline = metricPoints.map((point, index) => `${chartX(index)},${chartY(Number(point.delay_seconds || 0), maxDelay)}`).join(" ");
  $: delayTicks = buildTicks(maxDelay, delayText);
  $: rowTicks = buildTicks(maxRows, (value) => `${compactNumber(value)} 行`);
  $: xAxisLabels = buildXAxisLabels(metricPoints);
  $: delayPoint = metricPoints[resolveActiveIndex(activeDelayIndex)] || null;
  $: rowsPoint = metricPoints[resolveActiveIndex(activeRowsIndex)] || null;
  $: if (task.id && token && loadedMetricTaskId !== task.id) {
    loadedMetricTaskId = task.id;
    setMetricRange("24h", false);
    loadMetrics();
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
    metricRange = range;
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
      activeDelayIndex = metricPoints.length > 0 ? metricPoints.length - 1 : null;
      activeRowsIndex = metricPoints.length > 0 ? metricPoints.length - 1 : null;
      metricError = "";
    } catch (err) {
      metricError = err.message;
      metricPoints = [];
      activeDelayIndex = null;
      activeRowsIndex = null;
    }
  }
  function chartX(index) {
    if (metricPoints.length <= 1) return chartLeft + chartWidth / 2;
    return chartLeft + (index * chartWidth) / (metricPoints.length - 1);
  }
  function chartY(value, max) {
    return chartBottom - (Number(value || 0) * chartHeight) / Math.max(1, Number(max || 0));
  }
  function metricRowTotal(point) {
    return Number(point.insert_rows || 0) + Number(point.update_rows || 0) + Number(point.delete_rows || 0) + Number(point.read_rows || 0);
  }
  function buildTicks(maxValue, formatter) {
    const max = Math.max(1, Number(maxValue || 0));
    const values = [max, Math.ceil(max / 2), 0];
    return [...new Set(values)].sort((a, b) => b - a).map((value) => ({ value, label: formatter(value) }));
  }
  function buildXAxisLabels(points) {
    if (!points?.length) return [];
    const candidates = [0, Math.floor((points.length - 1) / 2), points.length - 1];
    return [...new Set(candidates)].map((index) => ({ index, label: metricAxisTime(points[index]?.time) }));
  }
  function metricAxisTime(value) {
    if (!value) return "-";
    const date = new Date(value);
    return `${String(date.getMonth() + 1).padStart(2, "0")}/${String(date.getDate()).padStart(2, "0")} ${String(date.getHours()).padStart(2, "0")}:${String(date.getMinutes()).padStart(2, "0")}`;
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
  function barWidth() {
    if (metricPoints.length <= 1) return 14;
    return Math.max(8, Math.min(18, chartWidth / Math.max(6, metricPoints.length * 2.6)));
  }
  function hitWidth() {
    if (metricPoints.length <= 1) return chartWidth;
    return Math.max(18, chartWidth / metricPoints.length);
  }
  function resolveActiveIndex(index) {
    if (metricPoints.length === 0) return -1;
    if (index === null || index === undefined) return metricPoints.length - 1;
    return Math.max(0, Math.min(metricPoints.length - 1, index));
  }
  function selectDelayPoint(index) {
    activeDelayIndex = index;
  }
  function selectRowsPoint(index) {
    activeRowsIndex = index;
  }
  function handleChartKeydown(event, selectPoint, index) {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      selectPoint(index);
    }
  }
  async function refreshDetail(refreshMetrics = true) {
    if (detailRefreshing) return;
    detailRefreshing = true;
    try {
      await Promise.all([
        Promise.resolve(onRefresh()),
        loadRepairJobs(),
        refreshMetrics ? loadMetrics() : Promise.resolve()
      ]);
    } finally {
      detailRefreshing = false;
    }
  }
  function canRepairJob(job) {
    return canManage && job.job_type === "compare" && job.status === "success" && Number(job.diff_rows || 0) > 0;
  }
  async function loadRepairJobs() {
    if (!task.id || !token) return;
    try { repairJobs = await request(`/api/sync/tasks/${task.id}/repair/jobs`, { token }); repairError = ""; }
    catch (err) { repairError = err.message; }
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
  function confirmFullCompare() {
    showFullCompareConfirm = true;
  }
  async function startFullCompare() {
    if (!task.id || repairBusy) return;
    showFullCompareConfirm = false;
    repairBusy = true;
    try {
      await request(`/api/sync/tasks/${task.id}/repair/compare`, { method: "POST", token, body: {} });
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
  function confirmCancel(job) {
    pendingCancelJob = job;
    showCancelConfirm = true;
  }
  async function doCancel() {
    if (!pendingCancelJob || repairBusy) return;
    repairBusy = true;
    showCancelConfirm = false;
    try {
      await request(`/api/sync/repair/jobs/${pendingCancelJob.id}/cancel`, { method: "POST", token });
      await loadRepairJobs();
      pendingCancelJob = null;
    } catch (err) { repairError = err.message; }
    finally { repairBusy = false; }
  }
  function repairTablePageCount() {
    return Math.max(1, Math.ceil(compareTables.length / compareTablePageSize));
  }
  $: repairTableTotalPages = repairTablePageCount();
  $: repairTablePageItems = compareTables.slice((compareTablePage - 1) * compareTablePageSize, compareTablePage * compareTablePageSize);

  function yesterdayLocalString() {
    const d = new Date();
    d.setDate(d.getDate() - 1);
    const y = d.getFullYear();
    const m = String(d.getMonth() + 1).padStart(2, "0");
    const day = String(d.getDate()).padStart(2, "0");
    const h = String(d.getHours()).padStart(2, "0");
    const min = String(d.getMinutes()).padStart(2, "0");
    const s = String(d.getSeconds()).padStart(2, "0");
    return `${y}-${m}-${day}T${h}:${min}:${s}`;
  }
  function nowLocalString() {
    const d = new Date();
    const y = d.getFullYear();
    const m = String(d.getMonth() + 1).padStart(2, "0");
    const day = String(d.getDate()).padStart(2, "0");
    const h = String(d.getHours()).padStart(2, "0");
    const min = String(d.getMinutes()).padStart(2, "0");
    const s = String(d.getSeconds()).padStart(2, "0");
    return `${y}-${m}-${day}T${h}:${min}:${s}`;
  }
  async function openTimeCompare() {
    if (!task.id || repairBusy) return;
    repairBusy = true;
    try {
      const tables = task.task_tables || [];
      const items = [];
      for (let i = 0; i < tables.length; i++) {
        const t = tables[i];
        items.push({ index: i, source_table: t.source_table, target_table: t.target_table, task_table_id: t.id, included: true, cutoffColumn: "", allColumns: [], loadingColumns: false, colsLoaded: false });
      }
      compareTables = items;
      compareTablePage = 1;
      compareTimeFrom = yesterdayLocalString();
      compareTimeTo = nowLocalString();
      timeCompareError = "";
      showTimeCompareModal = true;
      repairBusy = false;
      // preload columns for all tables so rows that are pre-checked have their dropdown ready
      for (const item of items) {
        if (!item.colsLoaded && !item.loadingColumns) {
          loadCompareTableColumns(item);
        }
      }
    } catch (err) { repairError = err.message; repairBusy = false; }
  }

  // 加载指定表的时间字段
  async function loadCompareTableColumns(tableItem) {
    if (!task.id || tableItem.loadingColumns || tableItem.colsLoaded) return;
    tableItem.loadingColumns = true;
    // force Svelte to pick up the loading state
    compareTables = compareTables;
    try {
      const connName = task.source_db;
      const schema = await request(`/api/db/${encodeURIComponent(connName)}/table/${encodeURIComponent(tableItem.source_table)}/schema`, { token });
      const columns = schema.columns || schema || [];
      const cols = columns.filter(c => {
        const t = (c.type || c.DATA_TYPE || "").toLowerCase();
        const n = (c.name || c.Field || c.COLUMN_NAME || "").toLowerCase();
        return t.includes("time") || t.includes("date") || t.includes("timestamp") || n.includes("time") || n.includes("date");
      }).map(c => c.name || c.Field || c.COLUMN_NAME || "");
      tableItem.allColumns = cols;
      if (cols.length > 0) tableItem.cutoffColumn = cols[0];
      tableItem.colsLoaded = true;
    } catch (err) { /* ignore */ }
    finally {
      tableItem.loadingColumns = false;
      compareTables = compareTables;
    }
  }

  // 翻页时自动加载可见表的时间字段
  $: if (showTimeCompareModal && repairTablePageItems.length > 0) {
    for (const item of repairTablePageItems) {
      if (!item.colsLoaded && !item.loadingColumns) {
        loadCompareTableColumns(item);
      }
    }
  }

  function validateTimeRange(from, to) {
    if (!from || !to) return "请填写开始和结束时间";
    const fd = new Date(from);
    const td = new Date(to);
    const now = new Date();
    if (isNaN(fd.getTime()) || isNaN(td.getTime())) return "时间格式无效";
    if (fd >= td) return "开始时间必须早于结束时间";
    if (fd > now || td > now) return "时间不能大于当前时间";
    return "";
  }
  async function startTimeCompare() {
    if (!task.id || repairBusy) return;
    // 前端验证
    const errMsg = validateTimeRange(compareTimeFrom, compareTimeTo);
    if (errMsg) {
      timeCompareError = errMsg;
      return;
    }
    const included = compareTables.filter(t => t.included && t.cutoffColumn);
    if (included.length === 0) {
      timeCompareError = "请至少选择一张参与对比的表并选择时间字段";
      return;
    }
    repairBusy = true;
    showTimeCompareModal = false;
    try {
      const tableCutoffs = {};
      for (const item of included) {
        tableCutoffs[item.task_table_id] = item.cutoffColumn;
      }
      const from = new Date(compareTimeFrom);
      const to = new Date(compareTimeTo);
      // 设置结束时间为当天 23:59:59
      to.setHours(23, 59, 59, 999);
      const body = { table_cutoffs: tableCutoffs, cutoff_time: to.toISOString(), cutoff_from: from.toISOString() };
      await request(`/api/sync/tasks/${task.id}/repair/compare`, { method: "POST", token, body });
      await loadRepairJobs();
      onRefresh();
    } catch (err) { repairError = err.message; }
    finally { repairBusy = false; }
  }
	onMount(() => {
    loadRepairJobs();
    loadMetrics();
    let ticks = 0;
    const timer = setInterval(() => {
      ticks += 1;
      refreshDetail(ticks % 5 === 0);
    }, 2000);
    return () => clearInterval(timer);
  });
</script>

<section class="task-detail-page">
  <div class="detail-heading"><div><button class="ghost icon-text" on:click={onBack}><ArrowLeft size={16}/>返回任务</button><h2>{task.name}</h2><p>{task.source_db} → {task.target_db}</p></div><button class="ghost icon-text" on:click={() => refreshDetail(true)}><RefreshCw size={15}/>刷新</button></div>
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
        <button class:active-filter={metricRange === "24h"} class="ghost" on:click={() => setMetricRange("24h")}>24小时</button>
        <button class:active-filter={metricRange === "7d"} class="ghost" on:click={() => setMetricRange("7d")}>7天</button>
        <button class:active-filter={metricRange === "30d"} class="ghost" on:click={() => setMetricRange("30d")}>30天</button>
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
          {#if delayPoint}
            <div class="trend-inspector"><span>{metricTime(delayPoint.time)}</span><span>延迟 {delayText(delayPoint.delay_seconds)}</span><span>速率 {(Number(delayPoint.rows_per_second || 0)).toFixed(1)} 行/秒</span></div>
          {/if}
          <svg viewBox="0 0 600 160" class="trend-chart" role="img" aria-label="同步延迟趋势">
            {#each delayTicks as tick}
              <line class="grid-line" x1={chartLeft} y1={chartY(tick.value, maxDelay)} x2={chartRight} y2={chartY(tick.value, maxDelay)} />
              <text class="axis-label" x={chartLeft - 8} y={chartY(tick.value, maxDelay) + 4} text-anchor="end">{tick.label}</text>
            {/each}
            <line x1={chartLeft} y1={chartBottom} x2={chartRight} y2={chartBottom} />
            <line x1={chartLeft} y1={chartTop} x2={chartLeft} y2={chartBottom} />
            <polyline points={delayPolyline} />
            {#each metricPoints as point, index}
              <circle class:active-point={index === resolveActiveIndex(activeDelayIndex)} class="trend-point" cx={chartX(index)} cy={chartY(Number(point.delay_seconds || 0), maxDelay)} r="4" />
              <rect class="trend-hit" x={chartX(index) - hitWidth() / 2} y={chartTop} width={hitWidth()} height={chartBottom - chartTop + 16} role="button" tabindex="0" aria-label={`查看 ${metricTime(point.time)} 的延迟详情`} on:mouseenter={() => selectDelayPoint(index)} on:click={() => selectDelayPoint(index)} on:keydown={(event) => handleChartKeydown(event, selectDelayPoint, index)} />
            {/each}
            {#each xAxisLabels as label}
              <text class="axis-label axis-time" x={chartX(label.index)} y="150" text-anchor="middle">{label.label}</text>
            {/each}
          </svg>
        </div>
        <div class="trend-card">
          <div class="trend-card-head"><strong>行数变化</strong><span>峰值 {compactNumber(maxRows)} 行</span></div>
          {#if rowsPoint}
            <div class="trend-inspector rows-inspector"><span>{metricTime(rowsPoint.time)}</span><span>总计 {compactNumber(rowsPoint.total_rows || metricRowTotal(rowsPoint))} 行</span><span>读取 {compactNumber(rowsPoint.read_rows)} 行</span><span>新增 {compactNumber(rowsPoint.insert_rows)} 行</span><span>更新 {compactNumber(rowsPoint.update_rows)} 行</span><span>删除 {compactNumber(rowsPoint.delete_rows)} 行</span></div>
          {/if}
          <svg viewBox="0 0 600 160" class="trend-chart bar-chart" role="img" aria-label="增改删读取行数">
            {#each rowTicks as tick}
              <line class="grid-line" x1={chartLeft} y1={chartY(tick.value, maxRows)} x2={chartRight} y2={chartY(tick.value, maxRows)} />
              <text class="axis-label" x={chartLeft - 8} y={chartY(tick.value, maxRows) + 4} text-anchor="end">{tick.label}</text>
            {/each}
            <line x1={chartLeft} y1={chartBottom} x2={chartRight} y2={chartBottom} />
            <line x1={chartLeft} y1={chartTop} x2={chartLeft} y2={chartBottom} />
            {#each metricPoints as point, index}
              {@const total = Number(point.total_rows || metricRowTotal(point))}
              {@const x = chartX(index) - barWidth() / 2}
              {@const readH = (Number(point.read_rows || 0) * chartHeight) / maxRows}
              {@const insertH = (Number(point.insert_rows || 0) * chartHeight) / maxRows}
              {@const updateH = (Number(point.update_rows || 0) * chartHeight) / maxRows}
              {@const deleteH = (Number(point.delete_rows || 0) * chartHeight) / maxRows}
              {#if total > 0}
                <rect class:active-bar={index === resolveActiveIndex(activeRowsIndex)} class="read" x={x} y={chartBottom - readH} width={barWidth()} height={readH} />
                <rect class:active-bar={index === resolveActiveIndex(activeRowsIndex)} class="insert" x={x} y={chartBottom - readH - insertH} width={barWidth()} height={insertH} />
                <rect class:active-bar={index === resolveActiveIndex(activeRowsIndex)} class="update" x={x} y={chartBottom - readH - insertH - updateH} width={barWidth()} height={updateH} />
                <rect class:active-bar={index === resolveActiveIndex(activeRowsIndex)} class="delete" x={x} y={chartBottom - readH - insertH - updateH - deleteH} width={barWidth()} height={deleteH} />
              {/if}
              <rect class="trend-hit" x={chartX(index) - hitWidth() / 2} y={chartTop} width={hitWidth()} height={chartBottom - chartTop + 16} role="button" tabindex="0" aria-label={`查看 ${metricTime(point.time)} 的行数详情`} on:mouseenter={() => selectRowsPoint(index)} on:click={() => selectRowsPoint(index)} on:keydown={(event) => handleChartKeydown(event, selectRowsPoint, index)} />
            {/each}
            {#each xAxisLabels as label}
              <text class="axis-label axis-time" x={chartX(label.index)} y="150" text-anchor="middle">{label.label}</text>
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
          <button class="ghost icon-text" disabled={repairBusy || !!runningJob} on:click={confirmFullCompare}><ShieldAlert size={15}/>全量对比</button>
          <button class="ghost icon-text" disabled={repairBusy || !!runningJob} on:click={openTimeCompare}><RefreshCw size={15}/>按时间段补数</button>
        </div>
      {/if}
    </div>

    {#if repairError}<div class="inline-error">{repairError}</div>{/if}
    <table class="data-table repair-table">
      <thead><tr><th>类型</th><th>状态</th><th>进度</th><th>差异</th><th>已补数</th><th>说明</th><th>开始时间</th>{#if canManage}<th>操作</th>{/if}</tr></thead>
      <tbody>
        {#if repairJobs.length === 0}<tr class="empty-row repair-empty-row"><td colspan={canManage ? 8 : 7}><div class="empty-state repair-empty"><span class="empty-icon"><ShieldAlert size={24} /></span><strong>暂无数据修复任务</strong><p>发起全量对比后，可以根据差异一键补数。</p></div></td></tr>{/if}
        {#each repairJobs as job}
          <tr>
            <td>{jobTypeText(job.job_type, job)}</td>
            <td><span class={`pill ${job.status === "failed" ? "danger" : job.status === "success" ? "success" : "muted"}`}>{jobText(job.status)}</span></td>
            <td>{(job.progress_percent || 0).toFixed(1)}%</td>
            <td>{#if Number(job.diff_rows || 0) > 0}<button class="link-button" on:click={() => openDiffs(job)}>{job.diff_rows}</button>{:else}0{/if}</td>
            <td>{job.repaired_rows || 0}</td>
            <td>{job.error_detail || job.message || "-"}{#if job.cutoff_from || job.cutoff_time}<button class="link-button" on:click={() => showJobDetail = job}>详情</button>{:else if job.cutoff_column || job.cutoff_time}<span class="cell-sub">{#if job.cutoff_column}{job.cutoff_column}{/if}{#if job.cutoff_time} ≤ {new Date(job.cutoff_time).toLocaleString()}{/if}</span>{/if}</td>
            <td>{job.started_at ? new Date(job.started_at).toLocaleString() : "-"}</td>
            {#if canManage}<td>{#if canRepairJob(job)}<button class="ghost icon-text" disabled={repairBusy || !!runningJob} on:click={() => startRepair(job)}><RotateCw size={14}/>补这次</button>{:else if job.status === "running" || job.status === "canceling"}<button class="ghost icon-text" disabled={repairBusy} on:click={() => confirmCancel(job)}><X size={14}/>取消</button>{:else}-{/if}</td>{/if}
          </tr>
        {/each}
      </tbody>
    </table>
  </section>
</section>

{#if showCancelConfirm && pendingCancelJob}
  <div class="modal-layer">
    <button class="modal-backdrop" aria-label="关闭" on:click={() => { showCancelConfirm = false; pendingCancelJob = null; }}></button>
    <div class="modal confirm-modal confirm-compare-modal">
      <div class="confirm-compare-body">
        <h3>确认取消</h3>
        <p>当前 {pendingCancelJob.job_type === "compare" ? "对比" : "补数"} 任务正在执行，取消后发现的差异仍可查看。</p>
      </div>
      <div class="confirm-compare-footer">
        <button class="ghost" on:click={() => { showCancelConfirm = false; pendingCancelJob = null; }}>返回</button>
        <button class="primary" on:click={doCancel}>确认取消</button>
      </div>
    </div>
  </div>
{/if}

{#if showFullCompareConfirm}
  <div class="modal-layer">
    <button class="modal-backdrop" aria-label="关闭" on:click={() => { showFullCompareConfirm = false; }}></button>
    <div class="modal confirm-modal confirm-compare-modal">
      <div class="confirm-compare-body">
        <h3>确认全量对比</h3>
        <p>将对所有同步表逐行比对源端和目标端数据。</p>
        <p class="confirm-compare-warn">表数据量大时可能耗时较长，且会暂停正在运行的 CDC 同步任务。</p>
      </div>
      <div class="confirm-compare-footer">
        <button class="ghost" on:click={() => { showFullCompareConfirm = false; }}>取消</button>
        <button class="primary" on:click={startFullCompare}>确认发起</button>
      </div>
    </div>
  </div>
{/if}

{#if showTimeCompareModal}
  <div class="modal-layer">
    <button class="modal-backdrop" aria-label="关闭" on:click={() => { showTimeCompareModal = false; }}></button>
    <div class="modal time-compare-modal">
      <div class="modal-header">
        <div><h3>按时间段补数</h3>
        <p>选择时间范围，勾选参与对比的表并为每表选择时间字段</p></div>
        <button class="ghost icon" on:click={() => { showTimeCompareModal = false; }}><X size={17} /></button>
      </div>
      <div class="time-range-inputs">
        <label><span>开始时间</span><input type="datetime-local" step="1" bind:value={compareTimeFrom} /></label>
        <label><span>结束时间</span><input type="datetime-local" step="1" bind:value={compareTimeTo} /></label>
      </div>
      {#if timeCompareError}<div class="inline-error" style="margin:0 1.25rem 0.5rem">{timeCompareError}</div>{/if}
      <table class="data-table">
        <thead><tr><th style="width:2.5rem" aria-label="参与"></th><th>源表</th><th>目标表</th><th>时间字段</th></tr></thead>
        <tbody>
          {#each repairTablePageItems as item}
            <tr>
              <td style="text-align:center"><input type="checkbox" bind:checked={item.included} on:change={() => { if (item.included) loadCompareTableColumns(item); }} /></td>
              <td>{item.source_table}</td>
              <td>{item.target_table}</td>
              <td>
                {#if item.loadingColumns}
                  <span class="cell-sub">加载中...</span>
                {:else if !item.colsLoaded}
                  <span class="cell-sub">未加载</span>
                {:else if item.allColumns.length === 0}
                  <span class="cell-sub">无时间字段</span>
                {:else}
                  <select bind:value={item.cutoffColumn}>
                    {#each item.allColumns as col}<option value={col}>{col}</option>{/each}
                  </select>
                {/if}
              </td>
            </tr>
          {/each}
          {#if compareTables.length === 0}
            <tr class="empty-row"><td colspan="4"><div class="empty-state"><strong>暂无同步表</strong></div></td></tr>
          {/if}
        </tbody>
      </table>
      <div class="pager">
        <button class="ghost" disabled={compareTablePage <= 1} on:click={() => { compareTablePage -= 1; }}><ChevronLeft size={14} />上一页</button>
        <span>{compareTablePage} / {repairTableTotalPages}</span>
        <button class="ghost" disabled={compareTablePage >= repairTableTotalPages} on:click={() => { compareTablePage += 1; }}>下一页<ChevronRight size={14} /></button>
      </div>
      <div class="modal-actions">
        <button class="ghost" on:click={() => { showTimeCompareModal = false; }}>取消</button>
        <button class="primary" disabled={!compareTables.some(t => t.included && t.cutoffColumn)} on:click={startTimeCompare}>确认发起</button>
      </div>
    </div>
  </div>
{/if}

{#if showJobDetail}
  <div class="modal-layer">
    <button class="modal-backdrop" aria-label="关闭" on:click={() => (showJobDetail = null)}></button>
    <div class="modal compact-modal">
      <div class="modal-header">
        <h3>对比详情</h3>
        <p>{jobTypeText(showJobDetail.job_type, showJobDetail)}</p>
        <button class="ghost icon" on:click={() => (showJobDetail = null)}><X size={17} /></button>
      </div>
      <div class="detail-info-grid">
        <div><span>开始时间</span><strong>{showJobDetail.cutoff_from ? new Date(showJobDetail.cutoff_from).toLocaleString() : "不限"}</strong></div>
        <div><span>结束时间</span><strong>{showJobDetail.cutoff_time ? new Date(showJobDetail.cutoff_time).toLocaleString() : "不限"}</strong></div>
        <div style="grid-column:1/-1"><span>参与表</span><strong>{showJobDetail.table_cutoffs ? Object.keys(showJobDetail.table_cutoffs).length + " 张表" : "全部表"}</strong></div>
        {#if showJobDetail.table_cutoffs}
          <div style="grid-column:1/-1">
            <table class="data-table" style="margin-top:0.5rem">
              <thead><tr><th>表 ID</th><th>时间字段</th></tr></thead>
              <tbody>
                {#each Object.entries(showJobDetail.table_cutoffs) as [tableId, col]}
                  <tr><td>{tableId}</td><td>{col}</td></tr>
                {/each}
              </tbody>
            </table>
          </div>
        {/if}
      </div>
      <div class="actions"><button on:click={() => (showJobDetail = null)}>关闭</button></div>
    </div>
  </div>
{/if}

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
