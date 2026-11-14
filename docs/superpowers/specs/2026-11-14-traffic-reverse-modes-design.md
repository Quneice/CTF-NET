# 设计:新增流量分析模式与逆向模式(方案 A·纯模式壳)

日期:2026-11-14
状态:已实现(2026-11-14,终审通过)
规模:小改动(纯前端数据层,不新增后端工具)

## 意图

在现有 CTF / 渗透测试 / 模型安全三模式基础上,新增**流量分析模式**与**逆向模式**两个工作台,让流量工具从取证的二级 segment 独立成域,逆向分类获得专属模式入口。

## 方案取舍

| 方案 | 内容 | 结论 |
|---|---|---|
| A 纯模式壳 | 仅模式配置 + 分类树重组现有占位工具 | **选定**——便宜、零依赖,后续真工具按域渐进补 |
| B 壳 + Go 真工具 | 另实现 pcap 解析、ELF 解析等 | 本次不做,留待后续按需排期 |
| C 外挂 tshark/Ghidra | 集成重型外部工具链 | 否决——违背单 exe 便携哲学 |

## 目标

1. 模式选择页出现 5 张卡片;新卡片可进入对应工作台,侧边栏按新分类树展示。
2. 流量分析模式:新建 `traffic.*` 顶级分类树,22 个 `forensics.network` 工具迁入 4 个子分类。
3. 逆向模式:以现有 `reverse` 一级分类为根(静态/动态/去混淆,14 工具)。
4. 沿用共享分类惯例:逆向模式共享 `codec` + `data`。

## 非目标

- 不实现任何新后端 Go 工具(pcap/ELF 解析等全部留占位)。
- 不动 CTF 模式 forensics 其余 segment(内存/磁盘/日志)。
- 不引入外部依赖;不改 Go 后端任何代码。

## 约束

- 工具 id 一律不变(保 `/t/:toolId` URL),仅改 `categoryId` 归属。
- 旧 URL `/c/forensics.network` 加路由重定向(沿用既往迁移的重定向惯例)。
- README 模式描述同步(3 模式 → 5 模式)。

## 设计决策(我的判断,可纠正)

- **模式 id**:`'traffic'` / `'reverse'`。与分类 id `reverse` 同名不冲突(模式与分类是两个命名空间,`AppMode` 联合类型只影响 `MODE_CONFIG` 键)。
- **traffic 子分类**(22 工具按职能分 4 组):
  - `traffic.parse` 流量解析(3):pcap-analyze、strip-headers、dechunk
  - `traffic.protocol` 协议解码(12):tcp、udp、ipv4-header、ipv6、icmp、dns-extract、doh、http-request、netbios、protobuf、varint、parse-uri
  - `traffic.fingerprint` 指纹识别(4):ja3、hassh、user-agent、mac-format
  - `traffic.assist` 辅助处理(3):ip-range、group-ip、defang
- **forensics.network segment 删除**(工具已全部迁走,空壳无意义);forensics 分类 chips 从 4 组变 3 组(内存/磁盘/日志)。
- **CTF 模式 categoryRoots 追加 `'traffic'`**:流量题本是 CTF 取证子域,共享根(同 web/codec 被 ctf+pentest 共享的先例)。
- **逆向模式 categoryRoots**:`['reverse', 'codec', 'data']`。
- **模式卡片**:traffic → icon `Radio`,描述"流量取证:pcap 解析、协议解码、流量指纹与提取";reverse → icon `FileCode2`,描述"逆向工程:静态分析、动态调试、去混淆,共享编码与数据处理工具"。两图标已在 `lib/icons.tsx` 映射,零新增。

## 已核实的事实(实现零返工依据)

- 路由全泛化(`/c/:categoryId`、`/t/:toolId`),新分类/工具无需注册路由(`router/index.tsx:45,56`)。
- ModeSelectPage 遍历 `Object.keys(MODE_CONFIG)` 渲染卡片,数据驱动零改动。
- 模式不持久化(`useModeStore`,每次启动回模式选择页),无迁移问题。
- `categoriesForMode` / `toolsForMode` 纯函数按 categoryRoots 计算,自动生效。

## 留给实现

1. `mock/modes.ts`:`AppMode` 加 `'traffic' | 'reverse'`,`MODE_CONFIG` 加两条;ctf.roots 追加 `'traffic'`。
2. `mock/categories.ts`:加 4 个 `traffic.*` 顶级节点;删 `forensics.network`。
3. `mock/tools.ts`:22 个工具 `categoryId` 从 `forensics.network.*` 归属迁到对应 `traffic.*`(id 不动)。
4. `router/index.tsx`:`/c/forensics.network` → `/c/traffic` 重定向 1 条。
5. README「三模式」措辞与功能总览同步。
6. 验证:`npm run build` + 手动过一遍 5 模式切换、工具页 URL 直达、旧 URL 重定向。

## 测试口径

- 5 张模式卡片全部可进入、退出、再进入。
- 流量模式侧边栏 4 个子分类,22 工具逐一点开参数表单正常(占位提示符合预期)。
- `/t/forensics.network.pcap-analyze` 直达仍有效(id 未变);`/c/forensics.network` 跳转到 `/c/traffic`。
- CTF 模式 forensics 只剩 3 组 chips;逆向模式可见 reverse+codec+data 三根。
