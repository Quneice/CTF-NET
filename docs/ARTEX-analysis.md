# ARTEX 1.0.4 功能分析 与 CTF-NET 落地计划

> 分析对象：本地部署的 ARTEX v1.0.4（http://localhost:8787，用户名 ARTEX / 密码 12345678，
> 源码 C:\Users\HP\Desktop\ARTEX-1.0.4，jaykwok/ARTEX 自维护版：Go 后端 + Next.js 前端 + 内嵌 SQLite）。
> 落地目标：CTF-NET（Wails 桌面应用，原生 AI 引擎 internal/aiagent）。
> 日期：2026-11-12。

## 一、部署实例概况

- Next.js 控制台，登录后重定向 `/function/tasks`；后端 API 全部挂在 `/api/*`，共 **243 条路由**（`server/server.go` + 各 handler 文件注册）。
- 认证：JWT（`POST /api/auth/login`，用户名固定 `ARTEX`）。
- 数据：主业务 SQLite + 独立流量 SQLite，内嵌于单二进制；无外部数据库。
- 前端页面（`web/src/app`）：

| 分组 | 页面 |
|---|---|
| (auth) | /login、/setup（首启设管理员密码） |
| (main)/function | tasks（+detail）、findings（+detail）、assets、traffic、workspace、commands、llm-records、sync |
| (main)/system | agents（+detail）、intercept（+approvals）、llm、logs、mcp、notify、settings、skills、tools |
| (main) | dashboard（总览/Token 消耗/活动流）、chat（人在环路对话） |

## 二、功能域全景（API + 源码）

### 1. 任务（Task）域 —— `/api/tasks*`
- 任务 CRUD + 元数据（description/目标/hint/llm profile）；**分类**（task-categories 重命名/批量归类）；**模板**（task-templates 预置参数）；**批量控制**（control/batch：启动/暂停/停止）；**归档**（archive/restore/delete，批量 + 异步队列）。
- 任务级：goals 目标 CRUD（人工新增可"复活"已耗尽任务）、constraints 约束 CRUD（allow/deny 注入 planner/worker）、scope 范围 CRUD、assets 挂接、**coverage/coverage-graph**（资产覆盖：已测高亮/未测）、intent-assets、LLM 配置覆盖（updateTaskLLMPrompts + resolution）。
- 意图级：`POST /api/tasks/{id}/intents/{iid}/control`（暂停/继续/停止单条意图）、`messages`（给运行中的 worker 发消息=steer）、`rerun`（重跑单条 blocked/exhausted）、`rerun-blocked`（批量）。
- 引擎事件：`POST /api/tasks/{id}/control`（start/pause/resume/stop）、`GET /api/tasks/{id}/chat/status` + `/chat/stop`（任务内人在环路对话）。

### 2. 探索图 / 会话 —— `/api/exploration*`
- `graph` / `nodes`：资产、intent（frontier/工作节点）、fact、goal、hint、finding 的**节点+边图**（db nodes+edges，边类型 yields/derived_from/spawns/proves…）。
- `frontier`：当前可探索前沿；`intents`：意图列表；`main-sessions`（主 Agent 会话切换）；`activity`（+history/stream/详情）：全量活动流 SSE；`tokens`/`tokenStats`：token 消耗统计；`gc`：图垃圾回收。

### 3. 发现（Finding）域 —— `/api/exploration/findings*`
- 列表/分组（groups）/资产树（asset-tree）/统计；详情 + **lineage（血缘）**；`deepen`（深化调查）；**retests 复测**（发起复测会话/记录结果/自动改状态 fixed）。
- PATCH 手工修正；DELETE；`export` 导出（按模板出报告）；`finding-traffic`：漏洞↔流量证据绑定（bind/order/body 原文）。
- 引擎侧 finding_workflow.go / finding_recorder.go / finalizer.go：发现案例评审（case review）、去重合并（duplicates/merge 建议 + 人工确认）、报告书写（update_finding_report/CaseReport）、复测流程（record_finding_retest_result → fixed 状态机）。

### 4. 资产 / 企业域 —— `/api/assets`、`/api/companies`
- 资产：类型（root_domain/domain/ip/端口/服务/endpoint…）、props、按 host 删除、counts 统计、DSL 查询（list_assets 工具）。
- 企业（公司）：company + scope（域名/IP/CIDR/ICP 备案/关键词），域名自动认领归并；reattribute。
- 覆盖：`untested assets`（范围内未测资产清单，供 agent 补测）。
- 资产同步：`/api/sync/scopesentry/*`（第三方 ScopeSentry 项目/任务同步）。

### 5. 流量录制域 —— `/api/traffic*`
- 录制代理（浏览器走代理抓包 → 存流量 SQLite：request/response/blob）；`hosts` 聚合；`search/get/blob`（truncated @blob sha256 引用机制）；`exchange`；删除（单条/按 host/全部）。
- **browser/import**：浏览器扩展上报导入；**agent_traffic_binding / credential_traffic_binding** 设置：漏洞/凭据自动绑定流量证据。
- 引擎工具：traffic_search / traffic_get / traffic_blob / bind_finding_traffic / get_finding_traffic —— agent 可直接检索已录制流量作为证据。

### 6. 对话域 —— `/api/chat`、`/api/conversations*`
- **任务内聊天**：人在环路（主 Agent 观察+steer），消息带审批卡片（拦截审批在对话中可处理）、mentions（@任务/@资产检索）、文件上传（落到任务工作目录 uploads/）、停止。
- **独立对话（Conversations）**：多会话列表/重命名/profile（模型/角色卡）/批量删除；消息列表/发送/停止/详情（seq 粒度）；task chat 与 conversations 是两套。
- side-question 引擎（sidequestion/）：agent 请求→人工提问卡→SSE 事件流 /cancel（人在环路提问机制）。

### 7. Agent 管理域 —— `/api/agents*`
- 内置多 Agent（goals 目标拆解 / planner / mainagent / finalizer / finding_workflow…），每个：名称/描述/角色/**max_turns/run_seconds/web_search/interactive_shell/llm_profile_id/触发模式（serial/parallel/merge）**、MCP/skill/tool 数量。
- **Prompt 管理**：自定义 prompt（save/reset/**版本历史** prompts）、变量（variables）、预览（preview，真渲染模板）；**wrapup**（通用收尾 + task-timeout 专用收尾提示）。
- **Triggers**：事件触发器（LLM 记录事件等触发 agent 动作）。
- Visibility：资源可见性开关（全局/任务级）。

### 8. LLM 域 —— `/api/llm*`
- **Profiles**：多模型配置（provider/model/参数），任务可指定 profile，可切换激活（`/profiles/active`）。
- **Pool**：多 key 轮换池（llmpool/ 包：健康检查/故障转移/重置）。
- **retry-policy**：重试策略自定义；vision（视觉模型配置）；models 列表。
- **records**：全部 LLM 调用记录（输入/输出/token/耗时，可删除）；by-model/tasks 聚合；`llm_record` 设置开关。

### 9. 拦截审批域 —— `/api/intercept*`
- **规则**：内置+自定义（match_target=tool_input，regex 匹配，action=deny/ask，message，timeout 规则），优先级/启停。
- **审批流**：pending 队列 → 人工 decide（通过/拒绝）→ history（详情+执行结果）；**judge LLM**：用 LLM 自动预判审批（配置+用量统计）；tool-config：哪些工具触发拦截。
- 任务级审批列表；对话中审批卡片。

### 10. 工具/技能/MCP 域
- **Skills**（skills/ 目录）：agentskills.io 规范（SKILL.md+scripts），上传/删除/元数据/目录浏览/文件 CRUD（内置编辑器）、usage 统计、missing（agent 想调但不存在）；引擎工具 create_skill/update_skill_file。
- **自定义工具**（`/api/tools/custom`）：shell/command/script 三类，创建/测试/修改/删除；内置工具描述可改（PUT /api/tools/{key}）+ reset。
- **MCP**：stdio/http(Streamable HTTP)/sse 三种 transport，启用/禁用，tools 列表/refresh；引擎工具 create_mcp/update_mcp。
- **commands**：命令调用记录 + stats（按工具聚合次数）。

### 11. 通知 / 审计 / 其他
- **notify**：多渠道（webhook 等）CRUD/测试/deliveries 投递记录/重试/digest 摘要间隔。
- **审计 audit**；**设置**：web_search（brave/deepseek）、global_proxy、egress_probe、noa_compaction、credential_agent 等；python detect。
- **workspace**：任务工作目录文件浏览器（list/read/write/mkdir/delete/download/upload）。
- **report**：任务报告页；**logs**：后端日志（实时 SSE + 历史）；**update**：一键自更新（check/apply/rollback/stream）。
- **gc**、**health/stats**：仪表盘数据源。

### 12. 引擎层要点（agent/ 包）
- **MainAgent**：人在环路主 Agent（只读观察 + steer_work/直接高优先级意图注入），与 planner 分离。
- **Planner**：意图生成循环 + 态势读取（graph_overview）。
- **Coldgraph/cold-digest**：冷节点压缩（纯图算法：hot/cold 判定、连通分组、同父单例拯救）+ expand_digest。
- **deferred**：延迟工具调用（agent 规划后置动作）；**compaction**：上下文压缩（含 noa 实验性压缩）；**finalizer**：终态收尾（写结论/归档发现）。
- **constraints**：约束系统（allow/deny 注入 planner+worker）。
- 工具生态 59 个（见 docs/artex-probe/tools.json）：图读写（graph_overview/node_detail/expand_digest）、意图（add_intent/pause_task/kill_work/steer_work）、发现全流程（report_finding/update_finding_report/finding case/merge/retest）、资产（insert_assets/list_assets/list_untested_assets/delete_assets_by_host）、任务（spawn_task 子任务/list_tasks/worker trace 检索）、流量（traffic_search/get/blob）、技能/MCP/自定义工具自举（create_skill/create_mcp/create_custom_tool）、企业范围（add_company_scope/list_companies）、情报（record_intelligence 凭据记录）、目标/约束（set_goals/set_constraints/add_hint/add_task_hint/add_task_scope）。

## 三、CTF-NET 现状对照（差距矩阵）

CTF-NET 已有（原生引擎 internal/aiagent）：planner/worker 双角色、意图队列、黑板（goals/intents/assets/facts/findings/activities，task.json 落盘）、YAML 配方工具、证据纪律、ai.pentest.task/chat 双工具、实时流式对话（ARTEX 对话引擎原样照搬：norma + transcript + captureRun）、后台任务+task_status、轮次分段渲染、打断补充。

| # | ARTEX 功能 | CTF-NET 现状 | 差距 | 优先级 |
|---|---|---|---|---|
| 1 | 探索图（节点+边：yields/derived_from/spawns/proves） | 黑板是**扁平列表**，无边模型 | 需加 Edge 模型 + Graph 视图 | **P0** |
| 2 | 任务详情页（会话/工具调用/意图控制/图） | 只有 job 轮询进度条 | 需任务详情页 + 意图控制 API | **P0** |
| 3 | 发现页（列表/详情/血缘/导出/复测） | findings 只在 task.json，无页面 | 聚合 + 页面 + 导出 | **P0** |
| 4 | 资产页 + 覆盖（untested） | assets 只在 task.json | 聚合 + 页面 | **P0** |
| 5 | 仪表盘（总览/Token 消耗/活动流） | 无 | 统计聚合 + 页面 | **P0** |
| 6 | Token 用量统计（任务级/按模型/日聚合） | LLM 调用不记账 | Usage 累计落盘 + 统计 | **P0** |
| 7 | 意图控制（pause/rerun/steer/messages） | 只读 get_intent_trace | 黑板加控制钩子 + 编排循环支持 | **P0** |
| 8 | 拦截审批（规则/pending/judge LLM） | exec_command 硬编码黑名单 | 拦截框架（规则+审批卡+对话集成） | **P1** |
| 9 | LLM Profiles/Pool/retry-policy/vision | 单一模型配置 | 多 profile 配置 + 任务级选择 | **P1** |
| 10 | LLM 调用记录页（records/tokens） | 无 | 落记录 + 页面 | **P1** |
| 11 | Skills 管理（SKILL.md 规范+编辑器+usage） | YAML 配方（recipe.go，另一套格式） | 是否引入 skill 体系（与配方并存/融合） | **P1** |
| 12 | 自定义工具 CRUD（shell/command/script） | 配方 YAML 部分覆盖 | 运行时创建 + 测试 + 持久化 | **P1** |
| 13 | MCP 远程工具（stdio/http/sse） | 无 | MCP 客户端 | **P1** |
| 14 | 任务分类/模板/批量控制/归档 | 无 | 任务元数据 + 归档（软删除已有惯例） | **P1** |
| 15 | 流量录制（代理抓包+检索+证据绑定） | 无 | 录制代理 + 存储 + 检索 + agent 工具 | **P2** |
| 16 | 对话增强：conversations/mentions/上传/停止/审批卡 | 有对话（单会话流式）；无多会话/mentions/上传 | 会话管理增强 | **P2** |
| 17 | 发现深化/复测/案例合并（deepen/retests/case review） | 无 | 发现工作流（依赖图模型） | **P2** |
| 18 | 企业资产域（company+scope 认领归并） | projects 概念不同 | 可选（映射到 projects） | **P2** |
| 19 | 通知渠道/审计/报告页/日志页 | 无 | 通知（webhook）+日志页 | **P2** |
| 20 | 工作区文件浏览器 | 无 | 任务工作目录浏览 | **P2** |
| 21 | Agent 管理（prompt 版本/triggers/wrapup/visibility） | 提示词写死在 prompt.go | prompt 可配置化 | **P3** |
| 22 | 资产同步 ScopeSentry / 自更新 / 浏览器导入 | 无 | 第三方集成（桌面应用价值低） | **P3** |
| 23 | 目标/约束 CRUD（人工复活任务） | 无 UI | 任务级 goals/constraints 管理 | **P3** |
| 24 | side-questions（人在环路提问卡） | 无 | 依赖对话 UI 深化 | **P3** |

## 四、落地计划

**P0（本轮）——让 AI 渗透"看得见、控得住"**：
1. 黑板加边模型（Edge）+ Usage 记账 + 控制钩子（pause/steer/rerun）
2. 跨任务聚合统计层（stats.go：任务列表/发现/资产/token/活动流）
3. Wails 绑定（ai_api.go：列表/详情/控制/统计）
4. 前端 4 页：AI 仪表盘 / 任务详情（意图控制+探索图 SVG）/ 发现 / 资产 + 路由/侧边栏

**P1（下轮候选）**：拦截审批框架、LLM Profiles+调用记录、任务分类/归档、自定义工具/Skills UI、MCP 客户端。

**P2+**：流量录制、发现复测/案例合并、conversations 多会话、通知/审计/日志页、工作区浏览器。

## 五、关键设计取舍（假设，待用户确认）

1. **图模型**：CTF-NET 黑板加轻量 Edge 表（From/To/Rel），不引入 ARTEX 的 db nodes+edges 全量持久化——task.json 直接内嵌，单文件落盘与现有备份/删除惯例一致。
2. **拦截审批**：与 CTF-NET 现有 Executor 权限体系结合，规则引擎做在 aiagent 层（exec_command/http_request 前置检查），审批卡走现有 chat:* 事件流（复用打断/事件基建）。
3. **Skills vs 配方**：暂不引入 agentskills.io 体系，P1 先把自定义工具 CRUD 做出来（与 YAML 配方同一执行模型），避免两套并存。
4. **流量录制**：P2 做内置录制代理（Go 实现，参考 ARTEX traffic 包），不依赖浏览器扩展。
5. **仪表盘/统计**：不引入图表库，先用统计卡+列表+CSS 条形图（保持前端零新依赖）。
