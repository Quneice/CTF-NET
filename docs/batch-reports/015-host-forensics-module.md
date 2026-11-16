# 批 15 报告 · 主机取证模块拆分(forensics 提升为独立模式)

- 生成时间:2026-11-15
- 范围:用户需求「把有关主机取证的功能单独拆出一个模块」——按流量分析先例(traffic 自 forensics.network 拆出、独立模式可见),将 forensics 域(内存取证/磁盘取证/日志分析)提升为独立模块「主机取证」,新增第六个模式卡片。
- 批次性质:**结构拆分批** —— 零工具迁移、零功能变更;仅模式配置 + 分类显示名。

## 1. 改动文件

| 文件 | 改动 |
| --- | --- |
| `frontend/src/mock/modes.ts` | AppMode 联合类型 +`'hostforensics'`;MODE_CONFIG +「主机取证模式」条目(icon Server,categoryRoots = forensics/codec/data,同 traffic/reverse 共享编码与数据处理) |
| `frontend/src/mock/categories.ts` | forensics 顶级节点更名「主机取证」(原 Forensics),icon Search→Server,description 更新;**id 保持 `forensics` 不变** |
| `README.md` | 五模式工作台 → 六模式工作台,补「主机取证模式」说明 |
| `docs/batch-reports/015-host-forensics-module.md` | 本报告 |

未改动:tools.ts(工具 id/categoryId 零迁移)、router(分类 id 未变,全部旧路由 /c/forensics.* 继续有效)、icons.tsx(Server 已在注册表)、Go 后端。

## 2. 拆分口径(同 traffic 先例,逐条对齐)

| 维度 | traffic 先例 | 本批 forensics |
| --- | --- | --- |
| 分类 id | 新顶级 id `traffic`(22 工具迁入,工具 id 不变) | **沿用 id `forensics`**(已是顶级,零迁移) |
| 独立模式 | 流量分析模式可见 | 主机取证模式可见(第六模式) |
| CTF 共享 | CTF 模式共享该根 | CTF 模式共享该根(取证题本是 CTF 常驻题型) |
| 共享助手 | 共享编码/数据工具(codec/data) | 同(codec/data) |
| 子分类 | parse/protocol/fingerprint/assist | memory/disk/log(既有,不动) |

## 3. 验收清单(逐项结果)

| # | 验收项 | 结果 |
| --- | --- | --- |
| 1 | `npx tsc --noEmit` | ✅ 0 错误 |
| 2 | `npm run build` | ✅ 构建通过 |
| 3 | 模式卡渲染 | ✅ ModeSelectPage 遍历 `Object.keys(MODE_CONFIG)` 自动派生第六卡(主机取证模式,工具数 = forensics+codec+data) |
| 4 | 侧边栏/分类树 | ✅ CategoryTree/Sidebar 经 `categoriesForMode` 自动派生,新模式下展示 forensics 三子类 |
| 5 | 旧路由/旧模式回归 | ✅ 分类 id 未变:/c/forensics、/c/forensics.disk、CTF 模式内取证分类全部照常 |
| 6 | 工具零迁移 | ✅ tools.ts 未改动(874 工具 id/categoryId 全不变) |
| 7 | 图标 | ✅ Server 已在 lucide import 与 iconMap(批 13 前既有) |

## 4. 实现说明

1. **为什么不动工具**:forensics 在分类整改后已是顶级分类(内存/磁盘/日志三个 segment),「拆模块」的本质是把它从「CTF 模式内 14 分类之一」提升为「独立模式入口」,与 traffic 当年从 forensics.network 拆出的性质一致;分类 id 保持 `forensics` 使 70 个取证工具、旧路由、批 7 的 chips 分组机制全部零迁移。
2. **模式选择页自动派生**:无需改 ModeSelectPage/HomePage/Topbar/Sidebar——全部经 MODE_CONFIG 数据驱动。
3. **模式内助手分类**:codec(编码转换)+ data(数据处理)与 traffic/reverse 两模式同配置,保证取证工作流里编解码/文本处理顺手可得。

## 5. 假设与剩余风险

1. **假设(待用户纯文本确认)**:主机取证模式与 CTF 模式**共享** forensics 根(同 traffic/reverse 先例),CTF 模式仍是 14 大分类;若用户希望从 CTF 模式**摘除**取证分类(纯独立模式),只需从 MODE_CONFIG.ctf.categoryRoots 删除 'forensics' 一行。
2. **假设**:模块边界 = 内存取证 + 磁盘取证 + 日志分析 全部;forensics.disk 内混入的 CyberChef 系通用文件工具(压缩解压/正则提取等 ~20 个)本批不迁移——它们属于取证工作流常用件。若需「纯主机取证」口径,后续批可把这些通用件迁往 data/misc。
3. **ia.md/inventory 滞后**:同批 13/14 口径,未同步(建议单独重建批)。
4. **交互验证**:模式卡与侧边栏渲染需 `wails dev` 人工复核。
