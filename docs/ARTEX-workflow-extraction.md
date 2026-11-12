# ARTEX 核心工作流提取与集成（CTF-NET）

本文档记录从 ARTEX v1.0.4（`agent/` + `server/` 引擎）提取的**多 Agent 工作流 / 编排 / 角色分配 / 代理访问**机制，映射到 CTF-NET 原生引擎 `internal/aiagent` 的实现状态。

## 1. 角色分配（多 Agent）

| ARTEX 机制 | CTF-NET 实现 | 状态 |
|---|---|---|
| MainAgent（观察 graph_overview/list_*，舵向 add_hint/add_intent/steer_work/set_goals/set_constraints；目标达成时反问是否登记新目标） | 对话助手 + 工作台人工控制（steer/rerun/pause/resume）；对话层 main-agent 工具集（graph_overview/set_goals 等）**未实现** | 部分：人工控制闭环已有（task_control.go），模型侧 main-agent 工具待做 |
| goals 角色：任务创建时提取约束（自包含具体值）+ 目标分解 | `roles.go DecomposeGoals`：约束/范围写入黑板 `Constraints`/`Scope`，目标拆 2~5 条可验证子目标（`SetGoals`）；失败保留原始目标兜底 | ✅ 2026-11-12 |
| 6 角色（mainagent/planner/worker/goals/auto/pentest） | 3 角色：planner/worker/goals（goals 只在任务开始跑一次）；约束块注入 planner/worker 提示词（`constraintBlock`，最高优先级） | ✅（chat 助手≈mainagent 的观察/舵向面） |
| prompt override（文件覆盖 + 代码所有尾巴） | 未实现 | 待做 |

## 2. 编排（事件驱动意图循环）

| ARTEX 机制 | CTF-NET 实现 | 状态 |
|---|---|---|
| TriggerEvents（done+finding+goal+hint 带 detail）→ debounce 800ms → planner 轮（drainTriggers 进提示词，round markers） | 黑板变更唤醒通道（无事件类型语义）；planner 轮由 situationBlock 全量态势驱动 | 部分：唤醒已有，**触发事件类型+防抖未实现** |
| 持久 TodoStore（串行链） | todo_write 工具存在但**不持久化**（会话内覆盖式清单） | 部分 |
| planner 决策规则（与 open/running/recent_done 判重、blocked/exhausted 先看 trace、深度优先于广度、多样性、串行链只派下一步、硬地板"产出 0 意图=任务停摆"） | 全部在 plannerSystemPrompt 纪律中（判重/看 trace/深度优先/串行链）；硬地板由 orchestrator "新轮无产出→exhausted" 兜底 | ✅（提示词级） |
| worker 纪律（intent 进 system、边发现边写回、一条意图做完即停、合并事实） | workerSystemPrompt 已含全部纪律 +「边发现边写回」强化（2026-11-12） | ✅ |
| runIntent：per-work ctx + 命名 AbortCause（paused_by_user/killed_by_planner/…）+ steerBox（PreToolUse 钩子拦截工具调用）+ emptyTurnNudge(2) + model_error 重试 2×3s | 意图级 ctx（intentCancels）+ 取消→意图回 open 重领（暂停/舵向两路，等效 paused 状态映射）；空转提醒 2 轮注入（agent.go）；model_error 整轮重试 2×3s（agent.go）；steer 经「中止在途回合→重领开场注入」实现（等效 steerBox 阻塞语义） | ✅ 2026-11-12 |
| 状态映射：err→blocked、max_turns/timeout→exhausted、pause→paused、settling→exhausted、killed→stopped | err→blocked（取消中止除外→open）、TurnLimit→exhausted、任务级→timeout/stopped | ✅ |
| 任务级 deadline 协调器（settling→停止领取→90s 排水→最终 planner 轮→done/timeout 守卫） | 未实现（直接墙钟 timeout） | 待做 |
| finalizer（pending 异步工具 / 探测过但没写回 → 2 次重试写回；settlement 预算 10 轮 + 每角色 wrapup 词） | 未实现 | 待做 |
| verifyGoal（证据确认、覆盖百分比量化、shell 目标需 shell 发现）+ allGoalsVerified 完成时复核 | prove_goal 证据要点已有；量化目标/完成复核未实现 | 部分 |

## 3. 多 Agent 代理访问

| ARTEX 机制 | CTF-NET 实现 | 状态 |
|---|---|---|
| proxyEnv：HTTP_PROXY/HTTPS_PROXY/ALL_PROXY(+小写)+NODE_USE_ENV_PROXY+SSL_CERT_FILE 等注入 Bash 子进程 | `proxy.go ExecWithProxyEnv`：cmd `set "K=V"&&` 链 / POSIX `export`，tools/ai 两入口（task/chat）各包一次；LLM 与 http_request 本就走 Go Transport 代理 | ✅ 2026-11-12 |
| WebFetchProxy + CA 证书（MITM 录制） | 未实现（CTF-NET 无流量录制） | P2 待做 |
| WebSearchProxy 独立出口 | 未实现（无 web_search） | P2 待做 |

## 4. 工作流闭环

| ARTEX 机制 | CTF-NET 实现 | 状态 |
|---|---|---|
| 目标分解 + 约束提取（见上） | ✅ | |
| prove_goal 证据链 | ✅（evidence_summary 必填） | |
| finding_workflow 流量绑定 / retester 复测 / 审计 / notify | 未实现 | P2 待做（docs/ARTEX-analysis.md 有规划） |
| 按天 token 记账 / 探索图 | ✅（P0 工作台） | |

## 待做清单（按优先级）

1. **TriggerEvents**：黑板写入带事件类型（done/finding/goal/hint），planner 轮提示词注入已触发事件（替代全量态势的重复阅读），800ms 防抖合并。
2. **deadline 协调器**：settling 阶段停止领取新意图 → 排水在途回合（90s）→ 最终 planner 轮收尾 → done/timeout。
3. **finalizer + wrapup**：worker 会话结束但「有工具探测无写回」时注入结算提示重试写回；每角色 wrapup 词（worker：写回+一句话总结；planner：落结论）。
4. **prompt override**：`<DataDir>/aiagent/prompts/{planner,worker,goals}.md` 存在时覆盖内置角色卡（代码尾巴约束块仍强制追加）。
5. **main-agent 工具集**（对话层）：graph_overview/list_findings/list_facts/list_assets/add_intent/set_goals/set_constraints/steer_work 挂到 chat 助手 +「目标全部达成时反问是否登记新目标」判定。
6. **持久 TodoStore**：todo_write 落黑板字段，planner 轮态势注入未完成项。

## 实现要点备忘

- 约束块注入顺序：worker/planner 系统提示词末尾（`constraintBlock`，无约束时空串，fake 测试按「规划者/执行者」标记分类不受影响）。
- goals 提示词不得含「规划者/执行者」字样（fake LLM 分类串类坑）。
- 意图级取消：runWorker 用 `context.WithCancel(runCtx)`，取消后 settle 回 open（仅当任务仍在 running——任务停止时取消不应复活意图）；暂停循环每轮 cancelInFlight（幂等）。
- 空转提醒：思考-only 轮不结束会话（continue），第 2 轮注入提醒并重新计数，maxTurns 兜底防无限空转。
