# HTTP 代理拦截与重放(Burp 风格)Implementation Plan

**Goal:** 渗透测试模块新增顶级功能——HTTP 代理:捕获流量 → 拦截修改 → 放行/丢弃 → 历史记录 → 重放(等价 Burp Proxy/Intercept + Repeater 的核心环路)。

**架构:**

```
internal/proxy/            # Go 引擎(纯标准库:net/http + hijack,无第三方依赖)
  server.go                #   ServeHTTP:捕获(原始报文快照)→ 拦截挂起(channel)→ 转发 → 回写 + 历史
  history.go               #   内存环形历史(上限 500,新→旧)
  server_test.go           #   TDD:捕获/放行/丢弃/修改放行/重放/CONNECT 隧道
proxy_api.go               # Wails 绑定:ProxyStart/Stop/Status/SetIntercept/History/GetRecord/Decide/Replay
                           #   拦截事件 proxy:intercept(EventsEmit,载荷=Summary 小体,前端按 id 拉全量)
frontend/
  src/components/proxy/ProxyWorkspace.tsx   # 工作区:控制栏(端口/启停/拦截开关)+ 3 页签(拦截/历史/Repeater)
  src/pages/ToolPage.tsx                    # workspace 分发 +'proxy-interceptor' 分支
  src/mock/categories.ts                    # +pentest.proxy 代理拦截顶级分类(icon Waypoints)
  src/mock/tools.ts                         # +workspace 工具行(pentest.proxy.intercept)
  src/lib/backend.ts                        # 绑定封装 + proxy:intercept 事件订阅
  wailsjs/go/main/App.js|App.d.ts|models.ts # 生成绑定手写同步(wails build 会再生成,内容一致)
```

**API 契约(Go → JS):**

| 绑定 | 入参 | 返回 |
| --- | --- | --- |
| ProxyStart | port int | error |
| ProxyStop | — | error |
| ProxyStatus | — | {running, port, intercept} |
| ProxySetIntercept | on bool | error |
| ProxyHistory | offset, limit int | {total, items[]} (Summary 无报文) |
| ProxyGetRecord | id string | Record 全量(raw_request/raw_response) |
| ProxyDecide | id, action(forward/drop/modified), modified string | error |
| ProxyReplay | raw string | Record |
| 事件 proxy:intercept | — | Summary |

## 任务

1. **Go 引擎(TDD)**:捕获(snapshot 原始报文)→ 拦截开关为开时挂起(channel,等 UI 决策)→ forward/drop/modified 三动作;转发用 http.Transport 直连、不跟随重定向(CheckRedirect=ErrUseLastResponse);CONNECT 走 TCP 隧道(hijack + 双向拷贝,不解密——TLS MITM 见 P2);历史内存环(500 上限);Stop 时排空 pending(未决请求按 Drop 收尾)。测试:httptest 目标回显、拦截三动作、重放二次命中、CONNECT 隧道贯通、历史封顶。
2. **绑定层** proxy_api.go(App 单例 + sync.Once;事件回调挂 app.ctx)。
3. **前端工作区**:控制栏 + 拦截/历史/Repeater 三页签;拦截页 = 可编辑请求报文 + 放行/丢弃/修改并放行;历史页 = 列表(方法/主机/状态/耗时)+ 详情 + 发送到 Repeater;Repeater 页 = 报文编辑 + 发送 + 响应。无 Wails 时显示「需要桌面运行时」(同 fingerprint-db)。
4. **接线**:categories/tools/modes/README + wailsjs 生成绑定手写同步 + tsc/build/go test 全绿。

## 阶段划分(本批 = P1)

- **P1(本批)**:HTTP 捕获/拦截/修改/丢弃/放行 + 历史 + Repeater + CONNECT 直通隧道。
- **P2(后续)**:TLS MITM(内置 CA 签发+按域证书,HTTPS 解密拦截)、历史 SQLite 持久化、作用域规则(Burp Scope)、请求/响应高亮搜索、全局代理链(设置页 proxy 联动)、自动改包规则(host 重写等)。
- **明确不做**:WebSocket/HTTP2 拦截、Intruder 批量爆破(可复用 packer_fuzzer/jsfinder 思路另批)。

## Global Constraints

- 纯标准库实现代理引擎,不引第三方(项目惯例:核心引擎零新依赖)。
- 工作区不占工具注册表(同 fingerprint-db 先例);状态由 workspace → implemented 自动推导(真实实现,不伪装)。
- TDD:RED→GREEN;绑定契约 = Go 结构体 snake_case json tag + Wails 值返回(同 FPDBListResult 先例)。
- 拦截挂起无 UI 决策时请求保持挂起(与 Burp 行为一致);Stop 时统一 Drop 收尾。
