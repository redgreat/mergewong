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
  const delayText = (seconds=0) => `${(seconds * 1000).toLocaleString()} ms`;
  let repairJobs = [];
  let repairError = "";
  let cutoffColumn = "LastUpdateTime";
  let cutoffTime = "";
  let repairBusy = false;
	$: snapshotTotal = (task.task_tables || []).reduce((sum, table) => sum + Number(table.snapshot_total || 0), 0);
	$: snapshotProcessed = (task.task_tables || []).reduce((sum, table) => sum + Number(table.snapshot_processed || 0), 0);
	$: overallPercent = snapshotTotal > 0 ? Math.min(100, snapshotProcessed * 100 / snapshotTotal) : ((task.task_tables || []).every((table) => table.sync_state === "active") ? 100 : 0);
  $: latestCompare = repairJobs.find((job) => job.job_type === "compare" && job.status === "success" && job.diff_rows > 0);
  $: runningJob = repairJobs.find((job) => job.status === "running" || job.status === "canceling");
  async function loadRepairJobs() {
    if (!task.id || !token) return;
    try { repairJobs = await request(`/api/sync/tasks/${task.id}/repair/jobs`, { token }); repairError = ""; }
    catch (err) { repairError = err.message; }
  }
  async function startCompare() {
    if (!task.id || repairBusy) return;
    repairBusy = true;
    try {
      const body = { cutoff_column: cutoffColumn.trim(), cutoff_time: cutoffTime ? cutoffTime.replace("T", " ") + ":00" : "" };
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
    const timer = setInterval(() => { onRefresh(); loadRepairJobs(); }, 2000);
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
  <section class="workspace-panel detail-section"><div class="card-header"><div><h2>同步进度</h2><p>新增表会先独立初始化并追平主链路，再自动合并。</p></div></div>
    <table class="data-table"><thead><tr><th>源表</th><th>目标表</th><th>阶段</th><th>初始化进度</th><th>已初始化 / 总行数</th><th>说明</th></tr></thead><tbody>
      {#each task.task_tables || [] as table}<tr><td>{table.source_table}</td><td>{table.target_table}</td><td><span class={`pill ${table.sync_state === "failed" ? "danger" : table.sync_state === "active" ? "success" : "muted"}`}>{stateText(table.sync_state)}</span></td><td><div class="progress-cell"><div class="progress-track"><span style={`width:${Math.min(100, table.progress_percent || 0)}%`}></span></div><strong>{(table.progress_percent || 0).toFixed(1)}%</strong></div></td><td>{table.snapshot_processed || 0} / {table.snapshot_total || 0}</td><td>{table.progress_message || "-"}</td></tr>{/each}
    </tbody></table>
  </section>
  <section class="workspace-panel detail-section">
    <div class="card-header">
      <div><h2>数据修复</h2><p>按当前字段映射和忽略字段执行源端到目标端的一致性对比与补数。</p></div>
      {#if canManage}
        <div class="header-actions">
          {#if runningJob}<button class="ghost icon-text" disabled={repairBusy} on:click={() => cancelRepair(runningJob)}><X size={15}/>取消</button>{/if}
          <button class="ghost icon-text" disabled={repairBusy || !!runningJob} on:click={startCompare}><ShieldAlert size={15}/>全量对比</button>
          <button class="icon-text" disabled={repairBusy || !!runningJob || !latestCompare} on:click={() => startRepair(latestCompare)}><RotateCw size={15}/>一键补数</button>
        </div>
      {/if}
    </div>
    {#if canManage}
      <div class="repair-toolbar">
        <label>截止字段<input bind:value={cutoffColumn} placeholder="例如 LastUpdateTime" /></label>
        <label>截止时间<input type="datetime-local" bind:value={cutoffTime} /></label>
      </div>
    {/if}
    {#if repairError}<div class="inline-error">{repairError}</div>{/if}
    <table class="data-table">
      <thead><tr><th>类型</th><th>状态</th><th>进度</th><th>差异</th><th>已补数</th><th>说明</th><th>开始时间</th></tr></thead>
      <tbody>
        {#if repairJobs.length === 0}<tr class="empty-row"><td colspan="7">暂无数据修复任务</td></tr>{/if}
        {#each repairJobs as job}
          <tr>
            <td>{jobTypeText(job.job_type)}</td>
            <td><span class={`pill ${job.status === "failed" ? "danger" : job.status === "success" ? "success" : "muted"}`}>{jobText(job.status)}</span></td>
            <td>{(job.progress_percent || 0).toFixed(1)}%</td>
            <td>{job.diff_rows || 0}</td>
            <td>{job.repaired_rows || 0}</td>
            <td>{job.error_detail || job.message || "-"}</td>
            <td>{job.started_at ? new Date(job.started_at).toLocaleString() : "-"}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  </section>
  <section class="workspace-panel detail-section"><div class="card-header"><div><h2>同步信息</h2></div></div><div class="detail-info-grid"><div><span>同步类型</span><strong>{task.sync_type === "full_cdc" ? "全量 + CDC" : task.sync_type === "cdc" ? "Binlog CDC" : "全量"}</strong></div><div><span>最近成功</span><strong>{task.last_success_at ? new Date(task.last_success_at).toLocaleString() : "-"}</strong></div><div><span>预警发送群</span><strong>{task.alert_channel?.name || "未配置"}</strong></div><div><span>当前阶段开始</span><strong>{task.phase_started_at ? new Date(task.phase_started_at).toLocaleString() : "-"}</strong></div></div></section>
</section>
