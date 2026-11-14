# 移位变换工具(逆向模块)Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 逆向模块新增「移位变换」工具:常数/递增/位循环/位移/字符串循环位移五类移位,三域(mod256/可打印/字母),双向互逆,用户指定 flag 格式的参数爆破。

**Architecture:** 纯前端三层——`lib/shift.ts` 纯函数引擎(可 node 断言)→ `components/tool/ShiftWorkspace.tsx` UI(ToolPage workspace 分发)→ mock 挂载(reverse 新子分类 reverse.transform)。零后端、零新依赖。

**Tech Stack:** React + TS(既有);node 26 直跑 .ts 断言(type stripping)。

**Spec:** 本文件即设计(对话已批准:用户指定爆破格式;挂逆向模块)。

## Global Constraints

- 纯前端 local 工具,不改 Go、不加依赖。
- 爆破格式串由用户输入(必填,占位 flag{),前缀/任意位置两种匹配。
- 验证口径:lib 纯函数 node 断言 RED→GREEN + npm run build + 手动清单;终审独立子代理。

## Review Focus

1. **递增移位语义** — 用户用例 `abc` b=1 s=1 → `bdf`(第 i 位 +(b+i·s))→ Task 1 断言钉死。
2. **域换行正确性** — 字母域 z+2→b 保大小写、可打印域 ~+1→空格 → Task 1 断言。
3. **爆破对称性** — 双向参数对称枚举必须覆盖加密参数(先加密再爆破能还原)→ Task 1 断言(inc b=2 s=3 加密后爆破命中含原文)。
4. **不可逆模式解密入口** — shl/shr 解密按钮置灰且不崩溃 → Task 2 手动清单。
5. **非 ASCII 输入** — 中文/超 0xFF 码点在 byte 域按 0xFF 截断处理不崩溃,UI 无异常 → Task 1 断言 + Task 2 手动。

---

### Task 1: lib/shift.ts 纯函数引擎(TDD)

**Files:**
- Test: `/tmp/shift_test.mjs`(node 断言,不入库)
- Create: `frontend/src/lib/shift.ts`

**Interfaces:**
- Produces(Task 2 消费):
  - `type ShiftMode = 'const' | 'inc' | 'rol' | 'ror' | 'shl' | 'shr' | 'rotl' | 'rotr'`
  - `type ShiftDomain = 'byte' | 'printable' | 'alpha'`
  - `interface ShiftOpts { mode: ShiftMode; domain: ShiftDomain; n: number }`(n 复用为 N/b/移位量;inc 额外 `s: number`——定为 `interface ShiftOpts { mode; domain; n; s?: number }`,inc 时 n=b 起始、s=步长)
  - `applyShift(input: string, o: ShiftOpts): string`
  - `invertShift(o: ShiftOpts): ShiftOpts | null`(shl/shr 返回 null)
  - `describeShift(o: ShiftOpts): string`(爆破结果参数描述)
  - `bruteForceShift(cipher: string, format: string, anywhere: boolean): { opts: ShiftOpts; plain: string; desc: string }[]`(format 空串返回 [])
  - 枚举范围:const n∈[-32,32]×3 域;inc n=b∈[-16,16]×s∈[-4,4]×3 域;rol/ror n∈[1,7];shl/shr n∈[1,7];rotl/rotr k∈[0,输入长-1]

- [ ] Step 1: 写 node 断言文件(引用 lib/shift.ts),先跑看失败(模块/函数不存在)
- [ ] Step 2: 实现 lib/shift.ts(域换行:alpha 仅字母 mod26 保大小写非字母不动;printable 以 0x20 为基 mod95 仅对可打印字符,非可打印原样;byte 用 charCodeAt & 0xFF)
- [ ] Step 3: 断言全过:`abc`b1s1→`bdf`;`a1Bz`N2alpha→`c1Db`;`~`N1printable→空格;ROL 0x81 n1=0x03;`abcd` rotl1→`bcda`;invert 复合恒等(可逆模式);inc b2s3 加密→爆破(前缀 flag{)命中含原文;`好` byte 域不崩溃;format 空→[]
- [ ] Step 4: Commit `feat: 移位变换纯函数引擎(五模式三域双向+格式爆破)`

### Task 2: ShiftWorkspace UI + 三处接线

**Files:**
- Create: `frontend/src/components/tool/ShiftWorkspace.tsx`
- Modify: `frontend/src/mock/categories.ts`(reverse children 加 `reverse.transform`「变换还原」icon Binary)
- Modify: `frontend/src/mock/tools.ts`(条目 `reverse.transform.shift` name 移位变换 workspace 'shift',tags ['逆向','移位','local'])
- Modify: `frontend/src/pages/ToolPage.tsx`(workspace 分发 `'shift'`)

**Interfaces:**
- Consumes: Task 1 全部导出。
- UI:输入 textarea;模式单选 5 组(常数/递增/位循环/位移/字符串旋转);域单选 3(位/字符串模式禁用);参数 n(与 s);方向 加密/解密切换(解密=invertShift,shl/shr 禁用);输出 文本+HEX 双视图;爆破区:格式输入(必填占位 flag{)+任意位置勾选+运行+命中列表(点击回填参数与输入)。
- [ ] Step 1: 实现 UI(输入即算,useMemo)
- [ ] Step 2: 接线三处 + `npm run build` → exit 0
- [ ] Step 3: 手动清单(dev):五模式各试一组;解密回环(加密→点解密→原文);shl 解密置灰;爆破 `dnci{|hvw|`?——现场用 inc b1s1 加密 flag{test} 后爆破应命中;中文输入不崩
- [ ] Step 4: Commit `feat: 移位变换工作区上线(reverse.transform 分类)`

### Task 3: README + wails build + 终审

- [ ] Step 1: README 工具表加 reverse.transform.shift 行
- [ ] Step 2: wails build 出 exe;桌面冒烟
- [ ] Step 3: Commit;独立子代理终审(包:2c8d502..HEAD 增量,Review Focus 五条);发现分级处置
