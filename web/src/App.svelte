<script>
  import { onDestroy, onMount } from "svelte";
  import { request } from "./api.js";
  import Sidebar from "./components/Sidebar.svelte";
  import Topbar from "./components/Topbar.svelte";
  import LoginPage from "./pages/LoginPage.svelte";
  import ConnectionsPage from "./pages/ConnectionsPage.svelte";
  import TasksPage from "./pages/TasksPage.svelte";
  import LogsPage from "./pages/LogsPage.svelte";
  import ConnectionModal from "./components/ConnectionModal.svelte";
  import TaskModal from "./components/TaskModal.svelte";

  let apiError = "";
  let apiInfo = "";
  let token = localStorage.getItem("token") || "";
  let view = token ? "connections" : "login";
  let sidebarCollapsed = false;
  let sidebarManual = false;
  let showConnectionModal = false;
  let showTaskModal = false;

  let loginForm = {
    username: "",
    password: ""
  };

  let connections = [];
  let connectionPage = 1;
  let connectionPageSize = 10;
  let connectionTotal = 0;
  let editingConnectionId = null;
  let connectionForm = {
    name: "",
    type: "mysql",
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

  let tasks = [];
  let taskPage = 1;
  let taskPageSize = 10;
  let taskTotal = 0;
  let editingTaskId = null;
  let taskForm = {
    name: "",
    source_db: "",
    source_table: "",
    target_db: "",
    target_table: "",
    sync_type: "full",
    incremental_key: "",
    cron_expression: "",
    field_mapping_json: "{}",
    status: 1
  };

  let logs = [];
  let logTaskId = "";
  let logPage = 1;
  let logPageSize = 10;
  let logTotal = 0;

  function applySidebarState() {
    if (!sidebarManual) {
      sidebarCollapsed = window.innerWidth < 1100;
    }
    if (window.innerWidth >= 1100) {
      sidebarManual = false;
    }
  }

  function toggleSidebar() {
    sidebarCollapsed = !sidebarCollapsed;
    sidebarManual = true;
  }

  onMount(() => {
    if (token) {
      loadConnections();
      loadTasks();
    }
    applySidebarState();
    window.addEventListener("resize", applySidebarState);
  });

  onDestroy(() => {
    window.removeEventListener("resize", applySidebarState);
  });

  function setMessage(message, type) {
    apiError = type === "error" ? message : "";
    apiInfo = type === "info" ? message : "";
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
      view = "connections";
      await loadConnections();
      await loadTasks();
    } catch (error) {
      setMessage(error.message, "error");
    }
  }

  function logout() {
    token = "";
    localStorage.removeItem("token");
    view = "login";
    connections = [];
    tasks = [];
    logs = [];
  }

  function resetConnectionForm() {
    editingConnectionId = null;
    connectionForm = {
      name: "",
      type: "mysql",
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

  function startEditConnection(connection) {
    editingConnectionId = connection.id;
    connectionForm = {
      name: connection.name,
      type: connection.type,
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
        host: connectionForm.host.trim(),
        port: Number(connectionForm.port),
        database: connectionForm.database.trim(),
        username: connectionForm.username.trim(),
        password: connectionForm.password,
        charset: (connectionForm.charset || "utf8mb4").trim(),
        max_idle: Number(connectionForm.max_idle) || 10,
        max_open: Number(connectionForm.max_open) || 100,
        status: Number(connectionForm.status)
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
        await request("/api/db/connections", {
          method: "POST",
          token,
          body: payload
        });
        setMessage("连接已创建", "info");
      }

      closeConnectionModal();
      await loadConnections();
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
      await loadConnections();
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
      sync_type: "full",
      incremental_key: "",
      cron_expression: "",
      field_mapping_json: "{}",
      status: 1
    };
  }

  function openTaskModal(mode, task) {
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
      sync_type: task.sync_type,
      incremental_key: task.incremental_key || "",
      cron_expression: task.cron_expression || "",
      field_mapping_json: JSON.stringify(task.field_mapping || {}, null, 2),
      status: task.status
    };
  }

  async function saveTask() {
    try {
      let fieldMapping = {};
      const mappingText = taskForm.field_mapping_json.trim();
      if (mappingText) {
        fieldMapping = JSON.parse(mappingText);
      }

      const payload = {
        name: taskForm.name.trim(),
        source_db: taskForm.source_db.trim(),
        source_table: taskForm.source_table.trim(),
        target_db: taskForm.target_db.trim(),
        target_table: taskForm.target_table.trim(),
        sync_type: taskForm.sync_type,
        incremental_key: taskForm.sync_type === "incremental" ? taskForm.incremental_key.trim() : "",
        cron_expression: taskForm.cron_expression.trim(),
        field_mapping: fieldMapping,
        status: Number(taskForm.status)
      };

      if (editingTaskId) {
        await request(`/api/sync/tasks/${editingTaskId}`, {
          method: "PUT",
          token,
          body: payload
        });
        setMessage("任务已更新", "info");
      } else {
        await request("/api/sync/tasks", {
          method: "POST",
          token,
          body: payload
        });
        setMessage("任务已创建", "info");
      }

      closeTaskModal();
      await loadTasks();
    } catch (error) {
      setMessage(error.message, "error");
    }
  }

  async function deleteTask(task) {
    const confirmed = window.confirm(`确认删除任务 ${task.name} 吗？`);
    if (!confirmed) {
      return;
    }
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

  async function executeTask(task) {
    try {
      await request(`/api/sync/tasks/${task.id}/execute`, {
        method: "POST",
        token
      });
      setMessage("任务已开始执行", "info");
      await loadTasks();
    } catch (error) {
      setMessage(error.message, "error");
    }
  }

  async function loadLogs() {
    if (!logTaskId) {
      logs = [];
      logTotal = 0;
      return;
    }

    try {
      const data = await request(`/api/sync/tasks/${logTaskId}/logs`, {
        token,
        params: {
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
</script>

<div class="layout">
  <Sidebar
    {token}
    {view}
    setView={(v) => (view = v)}
    {connectionTotal}
    {taskTotal}
    {logTotal}
    {sidebarCollapsed}
    toggleSidebar={toggleSidebar}
    logout={logout}
  />

  <main class="content">
    <Topbar {view} {token} />

    {#if apiError}
      <div class="alert error">{apiError}</div>
    {/if}
    {#if apiInfo}
      <div class="alert info">{apiInfo}</div>
    {/if}

    {#if view === "login"}
      <LoginPage {loginForm} onLogin={login} />
    {:else if view === "connections"}
      <ConnectionsPage
        {connections}
        {connectionPage}
        {connectionPageSize}
        {connectionTotal}
        onPrev={() => { connectionPage -= 1; loadConnections(); }}
        onNext={() => { connectionPage += 1; loadConnections(); }}
        onOpenNew={() => openConnectionModal("new")}
        onEdit={(c) => openConnectionModal("edit", c)}
        onTest={(c) => testConnection(c)}
        onDelete={(c) => deleteConnection(c)}
      />
    {:else if view === "tasks"}
      <TasksPage
        {tasks}
        {taskPage}
        {taskPageSize}
        {taskTotal}
        onPrev={() => { taskPage -= 1; loadTasks(); }}
        onNext={() => { taskPage += 1; loadTasks(); }}
        onOpenNew={() => openTaskModal("new")}
        onEdit={(t) => openTaskModal("edit", t)}
        onExecute={(t) => executeTask(t)}
        onDelete={(t) => deleteTask(t)}
      />
    {:else if view === "logs"}
      <LogsPage
        {tasks}
        {logTaskId}
        {logs}
        {logPage}
        {logPageSize}
        {logTotal}
        onChangeTask={loadLogs}
        onPrev={() => { logPage -= 1; loadLogs(); }}
        onNext={() => { logPage += 1; loadLogs(); }}
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
      connections={connections}
      onClose={closeTaskModal}
      onSave={saveTask}
    />
  </main>
</div>

 
