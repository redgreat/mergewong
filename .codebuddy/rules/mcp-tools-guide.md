# Git MCP 工具规则

## 核心规则
1. 先 `list_repos` 拿 `repo_id`（整数），禁止猜测。
2. `ref` 可传分支/标签/commit SHA，默认 HEAD。
3. 权限受 Access Key 约束，无权限报错属正常。
4. 大文件用 `start_line`/`end_line` 分段读。

## 工具清单

| 工具 | 必填参数 | 用途 |
|------|----------|------|
| `list_repos` | - | 列仓库，拿 repo_id |
| `list_branches` | repo_id | 列分支 |
| `list_tags` | repo_id | 列标签 |
| `list_tree` | repo_id | 列目录树（path/ref/recursive 可选） |
| `read_file` | repo_id, path | 读文件（ref/行号范围可选） |
| `git_log` | repo_id | 提交历史（path/max_count/since/until 可选） |
| `git_show` | repo_id, commit_sha | 查看提交详情含 diff |
| `git_diff` | repo_id, ref_a, ref_b | 比较两版本（path 可选） |
| `git_blame` | repo_id, path | 每行修改者（行号范围可选） |
| `git_grep` | repo_id, pattern | 正则搜代码（path/ref/ignore_case 可选） |
| `analyze_code` | repo_id, path, question | 大模型分析代码 |
| `review_diff` | repo_id, ref_a, ref_b | 大模型审查差异 |
