# 高优先占位实现批次 Implementation Plan

> REQUIRED SUB-SKILL: superpowers:executing-plans.

**Goal:** 实现上一批占位中解题频率最高的 5 项:USB 键盘流量还原、PNG 宽高修复、ZIP 伪加密修复、SSTI 19 引擎前端接线、Volatility 封装(如外部依赖可用)。

**架构:** 前三个为 Go 纯净实现(traffic/misc 分类,Runner+TDD 合成样本);SSTI 接线仅前端(tools.ts 补 inputSchema+task realBackend,后端 ssti_base.go 已在);统一 build/exe/终审。

## 任务
1. **USB 键盘流量还原** `traffic.usb-keyboard`:pcap 解析(全局头/linktype 189·220/记录头)→ URB data 区 8B HID 报文(修饰键+保留+6键码)→ HID 码表还原按键(Shift 大写/符号、Enter/Space/Backspace)。测试:合成 pcap(64B URB 头+8B HID)还原 "flag{usb}"。
2. **PNG 宽高修复** `misc.png-fix`:IHDR 宽高被改而 CRC 未改 → 爆破宽×高(≤8192,重算 CRC 匹配存储值),输出正确尺寸+修复 PNG artifact。测试:合成 PNG 改坏宽高,修复还原。
3. **ZIP 伪加密修复** `misc.zip-unfake`:local(PK0304 off6)与 central(PK0102 off8)flag bit0 不一致或双向清零可解 → 修复输出新 zip,验证 archive/zip 可开。测试:合成真加密标志位 zip。
4. **SSTI 前端接线**:tools.ts 19 个引擎条目(现 14+既有 5)补 inputSchema(template/mode 等参数,对照后端 ParamsAttr)+ invoke task realBackend `/api/tools/web.ssti_<engine>/run`。
5. **README/exe/终审**。

## Global Constraints
- 工具 id 用占位批次已生成的 id(前端条目改 invoke,不新建)。
- 每工具 TDD(合成样本)RED→GREEN;提交中文+Co-Authored-By。
