<script>
  import { onMount } from "svelte";
  import { CircleAlert, CircleCheck, X } from "lucide-svelte";
  import { request } from "./api.js";
  import Sidebar from "./components/Sidebar.svelte";
  import Topbar from "./components/Topbar.svelte";
  import LoginPage from "./pages/LoginPage.svelte";
  import ConnectionsPage from "./pages/ConnectionsPage.svelte";
  import TasksPage from "./pages/TasksPage.svelte";
  import TaskDetailPage from "./pages/TaskDetailPage.svelte";
  import LogsPage from "./pages/LogsPage.svelte";
  import ConnectionModal from "./components/ConnectionModal.svelte";
  import TaskModal from "./components/TaskModal.svelte";
  import UserModal from "./components/UserModal.svelte";
  import PasswordModal from "./components/PasswordModal.svelte";
  import UsersPage from "./pages/UsersPage.svelte";
  import AlertsPage from "./pages/AlertsPage.svelte";
  import AlertModal from "./components/AlertModal.svelte";

  let apiError = "";
  let apiInfo = "";
  let messageTimer;
  let toastKey = 0;
  let token = localStorage.getItem("token") || "";
  let currentUser = JSON.parse(localStorage.getItem("current-user") || "null") || {};
  let view = token ? "tasks" : "login";
  let sidebarCollapsed = localStorage.getItem("sidebar-expanded") !== "true";
  let theme = localStorage.getItem("theme") || "dark";
  let showConnectionModal = false;
  let showTaskModal = false;
  let showUserModal = false;
  let showPasswordModal = false;
  let showAlertModal = false;
  $: isAdmin = currentUser.role === "admin";

  let loginForm = {
    username: "",
    password: ""
  };

  let connections = [];
  let taskConnections = [];
  let connectionPage = 1;
  let connectionPageSize = 10;
  let connectionTotal = 0;
  let editingConnectionId = null;
  let connectionForm = {
    name: "",
    type: "mysql",
    usage: "source",
    host: "",
    port: 3306,
    database: "",
    username: "",
    password: "",
    charset: "utf8mb4",
    max_idle: 10,
    max_open: 100,
    status: 1
  };

  let alertChannels = [];
  let taskAlertChannels = [];
  let alertPage = 1;
  let alertPageSize = 10;
  let alertTotal = 0;
  let editingAlertId = null;
  let alertForm = { name: "", robot_id: "", status: 1 };

  let tasks = [];
  let taskPage = 1;
  let taskPageSize = 10;
  let taskTotal = 0;
  let editingTaskId = null;
  let taskPrecheckResult = null;
  let savingTask = false;
  let taskForm = {
    name: "",
    source_db: "",
    source_table: "",
    target_db: "",
    target_table: "",
    table_mappings: [],
	  sync_type: "full_cdc",
    schedule_type: "manual",
    interval_minutes: 5,
    cron_expression: "",
    field_mappings: [],
    status: 1,
    alert_channel_id: "",
    alert_delay_ms: 5000
  };

  let logs = [];
  let logTaskId = "";
  let logPage = 1;
  let logPageSize = 10;
  let logTotal = 0;

  let users = [];
  let userPage = 1;
  let userPageSize = 10;
  let userTotal = 0;
  let editingUserId = null;
  let userForm = { username: "", password: "", email: "", role: "viewer", status: 1 };
  let passwordForm = { current_password: "", new_password: "", confirm_password: "" };

  function toggleSidebar() {
    sidebarCollapsed = !sidebarCollapsed;
    localStorage.setItem("sidebar-expanded", String(!sidebarCollapsed));
  }

  function toggleTheme() {
    theme = theme === "dark" ? "light" : "dark";
    localStorage.setItem("theme", theme);
    document.documentElement.dataset.theme = theme;
  }

  onMount(() => {
    if (token) {
      loadProfile();
      loadConnections();
      loadTaskConnections();
      loadTasks();
      loadAlertChannels();
      loadTaskAlertChannels();
    }
    document.documentElement.dataset.theme = theme;
  });

  function setMessage(message, type) {
	clearTimeout(messageTimer);
    apiError = type === "error" ? message : "";
    apiInfo = type === "info" ? message : "";
	toastKey += 1;
	if (message) {
	  messageTimer = setTimeout(() => {
		apiError = "";
		apiInfo = "";
	  }, 3600);
	}
  }

  async function login() {
    try {
      setMessage("", "info");
      const data = await request("/api/auth/login", {
        method: "POST",
        body: loginForm
      });
      token = data.token;
      localStorage.setItem("token", token);
      currentUser = { id: data.user_id, username: data.username, role: data.role };
      localStorage.setItem("current-user", JSON.stringify(currentUser));
      view = "tasks";
      await loadConnections();
      await loadTaskConnections();
      await loadTasks();
      await loadAlertChannels();
      await loadTaskAlertChannels();
    } catch (error) {
      setMessage(error.message, "error");
    }
  }

  function logout() {
    token = "";
    localStorage.removeItem("token");
    localStorage.removeItem("current-user");
    currentUser = {};
    view = "login";
    connections = [];
    taskConnections = [];
    tasks = [];
    logs = [];
    users = [];
    alertChannels = [];
    taskAlertChannels = [];
  }

  async function loadProfile() {
    try {
      currentUser = await request("/api/profile", { token });
      localStorage.setItem("current-user", JSON.stringify(currentUser));
    } catch (error) {
      logout();
    }
  }

  async function loadUsers() {
    if (!isAdmin) return;
    try {
      const data = await request("/api/users", { token, params: { page: userPage, page_size: userPageSize } });
      users = data.data;
      userTotal = data.total;
    } catch (error) { setMessage(error.message, "error"); }
  }

  function resetUserForm() {
    editingUserId = null;
    userForm = { username: "", password: "", email: "", role: "viewer", status: 1 };
  }

  function openUserModal(user = null) {
    if (user) {
      editingUserId = user.id;
      userForm = { username: user.username, password: "", email: user.email || "", role: user.role, status: user.status };
    } else resetUserForm();
    showUserModal = true;
  }

  function closeUserModal() { showUserModal = false; resetUserForm(); }

  async function saveUser() {
    try {
      const wasEditing = !!editingUserId;
      const payload = { email: userForm.email.trim(), role: userForm.role, status: Number(userForm.status) };
      if (editingUserId) {
        await request(`/api/users/${editingUserId}`, { method: "PUT", token, body: payload });
      } else {
        await request("/api/users", { method: "POST", token, body: { ...payload, username: userForm.username.trim(), password: userForm.password } });
      }
      closeUserModal(); setMessage(wasEditing ? "用户已更新" : "用户已创建", "info"); await loadUsers();
    } catch (error) { setMessage(error.message, "error"); }
  }

  async function deleteUser(user) {
    if (!window.confirm(`确认删除用户 ${user.username} 吗？`)) return;
    try { await request(`/api/users/${user.id}`, { method: "DELETE", token }); setMessage("用户已删除", "info"); await loadUsers(); }
    catch (error) { setMessage(error.message, "error"); }
  }

  function openPasswordModal() {
    passwordForm = { current_password: "", new_password: "", confirm_password: "" };
    showPasswordModal = true;
  }

  async function changePassword() {
    if (passwordForm.new_password !== passwordForm.confirm_password) { setMessage("两次输入的新密码不一致", "error"); return; }
    try {
      await request("/api/profile/password", { method: "PUT", token, body: { current_password: passwordForm.current_password, new_password: passwordForm.new_password } });
      showPasswordModal = false; logout();
    } catch (error) { setMessage(error.message, "error"); }
  }

  function changeView(nextView) {
    if (nextView === "users") loadUsers();
    if (nextView === "alerts") loadAlertChannels();
	if (nextView === "logs") loadLogs();
    view = nextView;
  }

  function resetConnectionForm() {
    editingConnectionId = null;
    connectionForm = {
      name: "",
      type: "mysql",
      usage: "source",
      host: "",
      port: 3306,
      database: "",
      username: "",
      password: "",
      charset: "utf8mb4",
      max_idle: 10,
      max_open: 100,
      status: 1
    };
  }

  function openConnectionModal(mode, connection) {
    if (mode === "new") {
      resetConnectionForm();
    } else if (connection) {
      startEditConnection(connection);
    }
    showConnectionModal = true;
  }

  function closeConnectionModal() {
    showConnectionModal = false;
    resetConnectionForm();
  }

  async function loadConnections() {
    try {
      const data = await request("/api/db/connections", {
        token,
        params: {
          page: connectionPage,
          page_size: connectionPageSize
        }
      });
      connections = data.data;
      connectionTotal = data.total;
    } catch (error) {
      setMessage(error.message, "error");
    }
  }

  async function loadTaskConnections() {
    try {
      const data = await request("/api/db/connections", { token, params: { page: 1, page_size: 100 } });
      taskConnections = data.data;
    } catch (error) { setMessage(error.message, "error"); }
  }

  function startEditConnection(connection) {
    editingConnectionId = connection.id;
    connectionForm = {
      name: connection.name,
      type: connection.type,
      usage: connection.usage || "both",
      host: connection.host,
      port: connection.port,
      database: connection.database,
      username: connection.username,
      password: "",
      charset: connection.charset || "utf8mb4",
      max_idle: connection.max_idle || 10,
      max_open: connection.max_open || 100,
      status: connection.status
    };
  }

  async function saveConnection() {
    try {
      const payload = {
        name: connectionForm.name.trim(),
        type: connectionForm.type,
        usage: connectionForm.usage,
        host: connectionForm.host.trim(),
        port: Number(connectionForm.port),
        database: connectionForm.database.trim(),
        username: connectionForm.username.trim(),
        password: connectionForm.password,
        charset: (connectionForm.charset || "utf8mb4").trim(),
        max_idle: Number(connectionForm.max_idle) || 10,
        max_open: Number(connectionForm.max_open) || 100
      };

      if (editingConnectionId) {
        if (!payload.password) {
          delete payload.password;
        }
        await request(`/api/db/connections/${editingConnectionId}`, {
          method: "PUT",
          token,
          body: payload
        });
        setMessage("连接已更新", "info");
      } else {
        delete payload.status;
        await request("/api/db/connections", {
          method: "POST",
          token,
          body: payload
        });
        setMessage("连接已创建", "info");
      }

      closeConnectionModal();
      await Promise.all([loadConnections(), loadTaskConnections()]);
    } catch (error) {
      setMessage(error.message, "error");
    }
  }

  async function deleteConnection(connection) {
    const confirmed = window.confirm(`确认删除连接 ${connection.name} 吗？`);
    if (!confirmed) {
      return;
    }
    try {
      await request(`/api/db/connections/${connection.id}`, {
        method: "DELETE",
        token
      });
      setMessage("连接已删除", "info");
      await Promise.all([loadConnections(), loadTaskConnections()]);
    } catch (error) {
      setMessage(error.message, "error");
    }
  }

  async function testConnection(connection) {
    try {
      await request(`/api/db/connections/${connection.id}/test`, {
        method: "POST",
        token
      });
      setMessage(`连接 ${connection.name} 测试成功`, "info");
    } catch (error) {
      setMessage(error.message, "error");
    }
  }

  function resetTaskForm() {
    editingTaskId = null;
    taskForm = {
      name: "",
      source_db: "",
      source_table: "",
      target_db: "",
      target_table: "",
      table_mappings: [],
      sync_type: "full_cdc",
      schedule_type: "manual",
      interval_minutes: 5,
      cron_expression: "",
      field_mappings: [],
      status: 1,
      alert_channel_id: "",
      alert_delay_ms: 5000
    };
  }

  function openTaskModal(mode, task) {
    taskPrecheckResult = null;
    if (mode === "new") {
      resetTaskForm();
    } else if (task) {
      startEditTask(task);
    }
    showTaskModal = true;
  }

  function closeTaskModal() {
    showTaskModal = false;
    resetTaskForm();
    taskPrecheckResult = null;
  }

  async function loadTasks() {
    try {
      const data = await request("/api/sync/tasks", {
        token,
        params: {
          page: taskPage,
          page_size: taskPageSize
        }
      });
      tasks = data.data;
      taskTotal = data.total;
    } catch (error) {
      setMessage(error.message, "error");
    }
  }

  function startEditTask(task) {
    editingTaskId = task.id;
    taskForm = {
      name: task.name,
      source_db: task.source_db,
      source_table: task.source_table,
      target_db: task.target_db,
      target_table: task.target_table,
      table_mappings: (task.task_tables?.length ? task.task_tables : [{ source_table: task.source_table, target_table: task.target_table, field_mapping: task.field_mapping || {} }]).map((table) => ({ source_table: table.source_table, target_table: table.target_table, field_mapping: table.field_mapping || {}, ignored_fields: table.ignored_fields || [], type_mismatch_ignores: table.type_mismatch_ignores || [] })),
      sync_type: task.sync_type,
	  schedule_type: "manual",
      interval_minutes: task.interval_minutes || 5,
      cron_expression: task.cron_expression || "",
      field_mappings: Object.entries(task.field_mapping || {}).map(([source, target]) => ({ source, target })),
      status: task.status,
      alert_channel_id: task.alert_channel_id || "",
      alert_delay_ms: (task.alert_delay_seconds || 0) * 1000
    };
  }

  async function saveTask() {
    try {
      savingTask = true;
      const normalizeFieldMapping = (mapping = {}) => {
        const fieldMapping = {};
        for (const [rawSource, rawTarget] of Object.entries(mapping || {})) {
          const source = String(rawSource || "").trim();
          const target = String(rawTarget || "").trim();
          if (!source && !target) continue;
          if (!source || !target) throw new Error("字段映射的源字段和目标字段必须同时填写");
          if (source !== target) fieldMapping[source] = target;
        }
        return fieldMapping;
      };
      const tableMappings = taskForm.table_mappings || [];
      if (!tableMappings.length) throw new Error("请至少选择一张同步表");
      const firstFieldMapping = normalizeFieldMapping(tableMappings[0].field_mapping);

      const payload = {
        name: taskForm.name.trim(),
        source_db: taskForm.source_db.trim(),
        source_table: taskForm.source_table.trim(),
        target_db: taskForm.target_db.trim(),
        target_table: taskForm.target_table.trim(),
        sync_type: taskForm.sync_type,
	    schedule_type: "manual",
        interval_minutes: Number(taskForm.interval_minutes) || 0,
        cron_expression: taskForm.cron_expression.trim(),
        field_mapping: firstFieldMapping,
        status: Number(taskForm.status),
        alert_channel_id: taskForm.alert_channel_id ? Number(taskForm.alert_channel_id) : 0,
        alert_delay_ms: Number(taskForm.alert_delay_ms) || 0,
        alert_on_error: true
      };

	  payload.tables = tableMappings.map((table) => ({
	    source_table: table.source_table.trim(),
	    target_table: table.target_table.trim(),
	    field_mapping: normalizeFieldMapping(table.field_mapping),
	    ignored_fields: table.ignored_fields || [],
	    type_mismatch_ignores: table.type_mismatch_ignores || []
	  }));
	  payload.source_table = payload.tables[0].source_table;
	  payload.target_table = payload.tables[0].target_table;

      if (editingTaskId) {
        await request(`/api/sync/tasks/${editingTaskId}`, {
          method: "PUT",
          token,
          body: payload
        });
      } else {
        const createdTask = await request("/api/sync/tasks", {
          method: "POST",
          token,
          body: payload
        });
        editingTaskId = createdTask.id;
      }
      taskPrecheckResult = await request(`/api/sync/tasks/${editingTaskId}/precheck`, { method: "POST", token });
      setMessage(taskPrecheckResult.passed ? "任务预检查通过并已启用" : "任务已保存，但预检查存在阻断项", taskPrecheckResult.passed ? "info" : "error");
      await loadTasks();
    } catch (error) {
      setMessage(error.message, "error");
    } finally {
      savingTask = false;
    }
  }

  async function deleteTask(task) {
    try {
      await request(`/api/sync/tasks/${task.id}`, {
        method: "DELETE",
        token
      });
      setMessage("任务已删除", "info");
      await loadTasks();
    } catch (error) {
      setMessage(error.message, "error");
    }
  }

  async function pauseTask(task) {
	try { await request(`/api/sync/tasks/${task.id}/pause`, { method: "POST", token }); setMessage("任务已暂停", "info"); await loadTasks(); }
	catch (error) { setMessage(error.message, "error"); }
  }

  async function resumeTask(task) {
	try { await request(`/api/sync/tasks/${task.id}/resume`, { method: "POST", token }); setMessage("任务已开始", "info"); await loadTasks(); }
	catch (error) { setMessage(error.message, "error"); }
  }

  async function updateTaskCheckpoint(task, checkpoint) {
	try { await request(`/api/sync/tasks/${task.id}/checkpoint`, { method: "PUT", token, body: checkpoint }); setMessage("Binlog 位点已修改", "info"); await loadTasks(); }
	catch (error) { setMessage(error.message, "error"); throw error; }
  }

  async function loadLogs() {
    try {
	  const data = await request(`/api/sync/logs`, {
        token,
        params: {
		  task_id: logTaskId || 0,
          page: logPage,
          page_size: logPageSize
        }
      });
      logs = data.data;
      logTotal = data.total;
    } catch (error) {
      setMessage(error.message, "error");
    }
  }

  async function loadAlertChannels() {
    try {
      const data = await request("/api/alerts/channels", { token, params: { page: alertPage, page_size: alertPageSize } });
      alertChannels = data.data;
      alertTotal = data.total;
    } catch (error) { setMessage(error.message, "error"); }
  }

  async function loadTaskAlertChannels() {
    try {
      const data = await request("/api/alerts/channels", { token, params: { page: 1, page_size: 100, enabled_only: true } });
      taskAlertChannels = data.data;
    } catch (error) { setMessage(error.message, "error"); }
  }

  function openAlertModal(channel = null) {
    editingAlertId = channel?.id || null;
    alertForm = channel ? { name: channel.name, robot_id: "", status: channel.status } : { name: "", robot_id: "", status: 1 };
    showAlertModal = true;
  }

  function closeAlertModal() { showAlertModal = false; editingAlertId = null; }

  async function saveAlert() {
    try {
      const wasEditing = !!editingAlertId;
      const payload = { name: alertForm.name.trim(), robot_id: alertForm.robot_id.trim(), status: Number(alertForm.status) };
      if (!payload.name || (!wasEditing && !payload.robot_id)) { setMessage("请填写发送群名称和企业微信机器人 ID", "error"); return; }
      if (wasEditing) await request(`/api/alerts/channels/${editingAlertId}`, { method: "PUT", token, body: payload });
      else await request("/api/alerts/channels", { method: "POST", token, body: payload });
      closeAlertModal(); setMessage(wasEditing ? "预警发送群已更新" : "预警发送群已创建", "info"); await Promise.all([loadAlertChannels(), loadTaskAlertChannels()]);
    } catch (error) { setMessage(error.message, "error"); }
  }

  async function testAlert(channel) {
    try { await request(`/api/alerts/channels/${channel.id}/test`, { method: "POST", token }); setMessage(`测试消息已发送到 ${channel.name}`, "info"); }
    catch (error) { setMessage(error.message, "error"); }
  }

  async function deleteAlert(channel) {
    if (!window.confirm(`确认删除预警发送群 ${channel.name} 吗？`)) return;
    try { await request(`/api/alerts/channels/${channel.id}`, { method: "DELETE", token }); setMessage("预警发送群已删除", "info"); await Promise.all([loadAlertChannels(), loadTaskAlertChannels()]); }
    catch (error) { setMessage(error.message, "error"); }
  }
</script>

{#if !token}
  <LoginPage {loginForm} onLogin={login} />
  {#if apiError || apiInfo}
    <div class="toast" class:error={!!apiError} class:info={!!apiInfo} role="status" aria-live="polite">
      <span class="toast-icon">{#if apiError}<CircleAlert size={18} />{:else}<CircleCheck size={18} />{/if}</span>
      <span>{apiError || apiInfo}</span>
      <button class="toast-close" type="button" aria-label="关闭提示" on:click={() => setMessage("", "info")}><X size={15} /></button>
    </div>
  {/if}
{:else}
<div class="layout" data-theme={theme}>
  <Sidebar
    {token}
    {view}
    setView={changeView}
    {sidebarCollapsed}
    toggleSidebar={toggleSidebar}
    role={currentUser.role}
  />

  <main class="content">
    <Topbar {view} {token} {theme} user={currentUser} onToggleTheme={toggleTheme} onChangePassword={openPasswordModal} {logout} />

    {#key toastKey}
      {#if apiError || apiInfo}
        <div class="toast" class:error={!!apiError} class:info={!!apiInfo} role="status" aria-live="polite">
          <span class="toast-icon">{#if apiError}<CircleAlert size={18} />{:else}<CircleCheck size={18} />{/if}</span>
          <span>{apiError || apiInfo}</span>
          <button class="toast-close" type="button" aria-label="关闭提示" on:click={() => setMessage("", "info")}><X size={15} /></button>
        </div>
      {/if}
    {/key}

    {#if view === "connections"}
      <ConnectionsPage
        canManage={isAdmin}
        connections={connections}
        {tasks}
        {connectionPage}
        {connectionPageSize}
        {connectionTotal}
        onPrev={() => { connectionPage -= 1; loadConnections(); }}
        onNext={() => { connectionPage += 1; loadConnections(); }}
        onOpenNew={() => openConnectionModal("new")}
        onEdit={(c) => openConnectionModal("edit", c)}
        onTest={(c) => testConnection(c)}
        onDelete={(c) => deleteConnection(c)}
        onRefresh={() => Promise.all([loadConnections(), loadTaskConnections()])}
      />
    {:else if view === "tasks"}
      <TasksPage
        canManage={isAdmin}
        tasks={tasks}
        {taskPage}
        {taskPageSize}
        {taskTotal}
        onPrev={() => { taskPage -= 1; loadTasks(); }}
        onNext={() => { taskPage += 1; loadTasks(); }}
        onOpenNew={() => openTaskModal("new")}
        onEdit={(t) => openTaskModal("edit", t)}
		onPause={(t) => pauseTask(t)}
		onResume={(t) => resumeTask(t)}
        onUpdateCheckpoint={(t, checkpoint) => updateTaskCheckpoint(t, checkpoint)}
        onDelete={(t) => deleteTask(t)}
        onRefresh={loadTasks}
        onDetail={(t) => { logTaskId = String(t.id); view = "task_detail"; loadTasks(); }}
      />
    {:else if view === "task_detail"}
      <TaskDetailPage
        task={tasks.find(t => String(t.id) === String(logTaskId)) || {}}
        onBack={() => { view = "tasks"; }}
        onRefresh={loadTasks}
      />
    {:else if view === "logs"}
      <LogsPage
        {tasks}
        bind:logTaskId
        {logs}
        {logPage}
        {logPageSize}
        {logTotal}
        onChangeTask={loadLogs}
        onPrev={() => { logPage -= 1; loadLogs(); }}
        onNext={() => { logPage += 1; loadLogs(); }}
        onRefresh={loadLogs}
      />
    {:else if view === "alerts"}
      <AlertsPage
        channels={alertChannels} {alertPage} {alertPageSize} {alertTotal} canManage={isAdmin}
        onPrev={() => { alertPage -= 1; loadAlertChannels(); }}
        onNext={() => { alertPage += 1; loadAlertChannels(); }}
        onOpenNew={() => openAlertModal()}
        onEdit={openAlertModal} onTest={testAlert} onDelete={deleteAlert}
        onRefresh={() => Promise.all([loadAlertChannels(), loadTaskAlertChannels()])}
      />
    {:else if view === "users" && isAdmin}
      <UsersPage
        {users} {userPage} {userPageSize} {userTotal}
        currentUserId={currentUser.id}
        onPrev={() => { userPage -= 1; loadUsers(); }}
        onNext={() => { userPage += 1; loadUsers(); }}
        onOpenNew={() => openUserModal()}
        onEdit={openUserModal}
        onDelete={deleteUser}
        onRefresh={loadUsers}
      />
    {/if}

    <ConnectionModal
      open={showConnectionModal}
      editing={!!editingConnectionId}
      form={connectionForm}
      onClose={closeConnectionModal}
      onSave={saveConnection}
    />
    <TaskModal
      open={showTaskModal}
      editing={!!editingTaskId}
      form={taskForm}
      {token}
      connections={taskConnections}
      alertChannels={taskAlertChannels}
      precheckResult={taskPrecheckResult}
      saving={savingTask}
      onClose={closeTaskModal}
      onSave={saveTask}
    />
    <UserModal open={showUserModal} editing={!!editingUserId} form={userForm} onClose={closeUserModal} onSave={saveUser} />
    <PasswordModal open={showPasswordModal} form={passwordForm} onClose={() => (showPasswordModal = false)} onSave={changePassword} />
    <AlertModal open={showAlertModal} editing={!!editingAlertId} form={alertForm} onClose={closeAlertModal} onSave={saveAlert} />
  </main>
</div>
{/if}

