<script>
  import { onMount } from "svelte";
  import { Cpu, Database, HardDrive, MemoryStick, RefreshCw, Save } from "lucide-svelte";
  import { request } from "../api.js";

  export let token = "";
  export let canManage = false;
  export let channels = [];
  export let onLoadChannels = () => {};

  let metrics = null;
  let setting = { enabled: true, alert_channel_id: "", cpu_threshold: 85, memory_threshold: 85, disk_threshold: 90, goroutine_threshold: 20000 };
  let error = "";
  let saving = false;

  const fmtPercent = (value) => `${Number(value || 0).toFixed(1)}%`;
  const fmtBytes = (value) => {
    const units = ["B", "KB", "MB", "GB", "TB"];
    let size = Number(value || 0);
    let index = 0;
    while (size >= 1024 && index < units.length - 1) { size /= 1024; index += 1; }
    return `${size.toFixed(index === 0 ? 0 : 1)} ${units[index]}`;
  };

  async function loadMetrics() {
    try { metrics = await request("/api/server/metrics", { token }); error = ""; }
    catch (err) { error = err.message; }
  }

  async function loadSetting() {
    try {
      const data = await request("/api/server/monitor-setting", { token });
      setting = {
        enabled: !!data.enabled,
        alert_channel_id: data.alert_channel_id || "",
        cpu_threshold: data.cpu_threshold || 85,
        memory_threshold: data.memory_threshold || 85,
        disk_threshold: data.disk_threshold || 90,
        goroutine_threshold: data.goroutine_threshold || 20000
      };
    } catch (err) { error = err.message; }
  }

  async function saveSetting() {
    saving = true;
    try {
      await request("/api/server/monitor-setting", { method: "PUT", token, body: { ...setting, enabled: setting.enabled === true || setting.enabled === "true", alert_channel_id: setting.alert_channel_id ? Number(setting.alert_channel_id) : 0 } });
      await loadSetting();
      error = "";
    } catch (err) { error = err.message; }
    finally { saving = false; }
  }

  onMount(() => {
    loadMetrics();
    loadSetting();
    onLoadChannels();
    const timer = setInterval(loadMetrics, 5000);
    return () => clearInterval(timer);
  });
</script>

<section class="workspace-panel">
  <div class="card-header">
    <div class="header-actions"><button class="ghost icon-text" on:click={loadMetrics}><RefreshCw size={15}/>刷新</button></div>
  </div>
  {#if error}<div class="inline-error">{error}</div>{/if}
  <div class="metric-grid server-metric-grid">
    <div class="metric-card"><span><Cpu size={16}/>CPU</span><strong>{fmtPercent(metrics?.cpu_percent)}</strong><small>{metrics?.num_cpu || 0} 核</small></div>
    <div class="metric-card"><span><MemoryStick size={16}/>内存</span><strong>{fmtPercent(metrics?.memory_percent)}</strong><small>{fmtBytes(metrics?.memory_used)} / {fmtBytes(metrics?.memory_total)}</small></div>
    <div class="metric-card"><span><HardDrive size={16}/>磁盘</span><strong>{fmtPercent(metrics?.disk_percent)}</strong><small>{fmtBytes(metrics?.disk_used)} / {fmtBytes(metrics?.disk_total)}</small></div>
    <div class="metric-card"><span><Database size={16}/>服务进程</span><strong>{metrics?.goroutines || 0}</strong><small>进程内存 {fmtBytes(metrics?.process_memory)}</small></div>
  </div>

  <section class="monitor-section">
    <div class="card-header compact"><div><h2>性能预警</h2></div>{#if canManage}<button class="icon-text" disabled={saving} on:click={saveSetting}><Save size={15}/>{saving ? "保存中..." : "保存设置"}</button>{/if}</div>
    <div class="form-grid monitor-form">
      <label><span>启用预警</span><select bind:value={setting.enabled} disabled={!canManage}><option value={true}>启用</option><option value={false}>停用</option></select></label>
      <label><span>预警发送群</span><select bind:value={setting.alert_channel_id} disabled={!canManage}><option value="">不发送预警</option>{#each channels as channel}<option value={channel.id}>{channel.name}</option>{/each}</select></label>
      <label><span>CPU 阈值 (%)</span><input type="number" min="1" max="100" bind:value={setting.cpu_threshold} disabled={!canManage}/></label>
      <label><span>内存阈值 (%)</span><input type="number" min="1" max="100" bind:value={setting.memory_threshold} disabled={!canManage}/></label>
      <label><span>磁盘阈值 (%)</span><input type="number" min="1" max="100" bind:value={setting.disk_threshold} disabled={!canManage}/></label>
      <label><span>Goroutine 阈值</span><input type="number" min="100" bind:value={setting.goroutine_threshold} disabled={!canManage}/></label>
    </div>
  </section>

  <section class="monitor-section">
    <div class="card-header compact"><div><h2>数据库连接池</h2></div></div>
    <table class="data-table">
      <thead><tr><th>连接</th><th>打开</th><th>使用中</th><th>空闲</th><th>最大打开</th><th>等待次数</th><th>等待耗时</th></tr></thead>
      <tbody>
        {#each metrics?.db_pools || [] as pool}
          <tr><td>{pool.name}</td><td>{pool.open}</td><td>{pool.in_use}</td><td>{pool.idle}</td><td>{pool.max_open || "-"}</td><td>{pool.wait_count}</td><td>{pool.wait_duration_ms} ms</td></tr>
        {/each}
        {#if !metrics?.db_pools?.length}<tr class="empty-row"><td colspan="7">暂无连接池数据</td></tr>{/if}
      </tbody>
    </table>
  </section>
</section>
