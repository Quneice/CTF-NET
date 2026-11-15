import type { CategoryNode } from '@/types';

/**
 * 分类树 mock 数据。
 *
 * 结构与后端 `app/core/tool.py` 的 Category 枚举对齐（web / pwn / reverse /
 * crypto / misc / forensics / mobile / blockchain / osint / ai），并额外拆分出
 * (2026-11-15 整改:隐写并入 Misc,Stego 顶级分类已撤销)。支持最多 4 层子分类,侧边栏限深一层。
 *
 * `icon` 使用 lucide-react 的组件名（字符串），在侧边栏由 iconMap 映射为真实组件。
 */
export const categories: CategoryNode[] = [
  {
    id: 'web',
    name: 'Web',
    icon: 'Globe',
    description: 'Web 安全、注入、前端攻击面',
    children: [
      { id: 'web.basic', name: '基础', icon: 'Wrench', description: 'HTTP、Cookie、目录扫描等基础工具' },
      { id: 'web.sql', name: 'SQL 注入', icon: 'Database', description: 'SQLi 探测与利用' },
      { id: 'web.xss', name: 'XSS', icon: 'Zap', description: '跨站脚本攻击' },
      { id: 'web.ssrf', name: 'SSRF', icon: 'Network', description: '服务端请求伪造' },
      { id: 'web.rce', name: 'RCE', icon: 'Terminal', description: '命令注入与远程代码执行' },
      { id: 'web.xxe', name: 'XXE', icon: 'FileWarning', description: 'XML 外部实体注入' },
      { id: 'web.deserialize', name: '反序列化', icon: 'Boxes', description: '不安全反序列化' },
      { id: 'web.ssti', name: 'SSTI', icon: 'Braces', description: '模板注入' },
      { id: 'web.upload', name: '文件上传', icon: 'Upload', description: '文件上传绕过' },
      { id: 'web.lfi', name: 'LFI / 路径穿越', icon: 'FolderSearch', description: '本地文件包含' },
      { id: 'web.php', name: 'PHP 专项', icon: 'FileCode', description: '弱类型、Magic Hash、逻辑绕过' },
    ],
  },
  {
    id: 'crypto',
    name: 'Crypto',
    icon: 'KeyRound',
    description: '密码学:古典密码、现代密码、哈希、自动解码(编码转换 → codec)',
    children: [
      {
        id: 'crypto.classical',
        name: '古典密码',
        icon: 'ScrollText',
        description: '凯撒、维吉尼亚、仿射等',
        children: [
          // ia.md §4:4 个 3 段节点降级为分类页筛选 chips(路由保留,侧边栏节点保留)
          { id: 'crypto.classical.caesar', name: '凯撒', icon: 'LetterText', segment: true },
          { id: 'crypto.classical.vigenere', name: '维吉尼亚', icon: 'LetterText', segment: true },
          { id: 'crypto.classical.affine', name: '仿射', icon: 'LetterText', segment: true },
          { id: 'crypto.classical.keyed', name: '带密钥古典密码', icon: 'KeyRound', description: 'Hill、Enigma、Fernet 等', segment: true },
        ],
      },
      {
        id: 'crypto.modern',
        name: '现代密码',
        icon: 'Lock',
        description: 'RSA、AES、ECC',
        children: [
          {
            id: 'crypto.modern.rsa',
            name: 'RSA',
            icon: 'ShieldCheck',
            // 批 5:5 个 4 段节点删除(22 个工具扁平化挂到本节点,tags 保留技术类别);
            // 旧 URL /c/crypto.modern.rsa.{5 段} 经 router 重定向到本节点(规则 8)。
            description: 'RSA 加解密与各类攻击',
          },
          { id: 'crypto.modern.aes', name: 'AES', icon: 'ShieldCheck' },
          { id: 'crypto.modern.ecc', name: 'ECC', icon: 'ShieldCheck' },
          { id: 'crypto.modern.dh', name: 'DH', icon: 'Handshake' },
        ],
      },
      { id: 'crypto.hash', name: '哈希', icon: 'Hash', description: '哈希与相关攻击' },
      { id: 'crypto.auto-decode', name: '自动解码', icon: 'Zap', description: '暴力尝试所有常见解码方式' },
    ],
  },
  // 批 1 新增一级分类(ia.md §1.2 codec 拆分):codec 细化为 8 个子类(2026-11-15 布局整改)
  {
    id: 'codec',
    name: '编码转换',
    icon: 'Binary',
    description: '无损格式转换:Base 家族、进制、趣味编码、序列化等',
    children: [
      { id: 'codec.base', name: 'Base 家族', icon: 'Binary', segment: true, description: 'Base2~Base92 全系列' },
      { id: 'codec.radix', name: '进制与字符码', icon: 'Hash', segment: true, description: '进制转换、字符码、字节序' },
      { id: 'codec.fun', name: '趣味编码', icon: 'Sparkles', segment: true, description: 'JSFuck/与佛论禅/ROT 系等迷因编码' },
      { id: 'codec.textenc', name: '文本与传输编码', icon: 'Braces', segment: true, description: 'UU/UTF-7/HTML 实体/URL 转义类' },
      { id: 'codec.serial', name: '数据序列化', icon: 'Boxes', segment: true, description: 'ASN.1/CBOR/TLV/MessagePack 等' },
      { id: 'codec.hex', name: 'Hex 工具', icon: 'FileSearch', segment: true, description: '十六进制查看与 Hexdump' },
      { id: 'codec.charset', name: '字符集与规范化', icon: 'Type', segment: true, description: '字符集编解码、Unicode 规范化' },
      { id: 'codec.barcode', name: '条码二维码', icon: 'QrCode', segment: true, description: '二维码/条形码解码' },
    ],
  },
  {
    id: 'misc',
    name: 'Misc',
    icon: 'Puzzle',
    description: '杂项、隐写与文本数据处理',
    children: [
      // 2026-11-15 布局整改:隐写并入 Misc(原 stego 顶级分类撤销);misc.other 94 工具细化拆分(80 迁出,余 14);
      // misc.demo/misc.report(框架演示,无用)删除;全部子类以 chips 形式在 /c/misc 右侧筛选。
      { id: 'misc.stego.image', name: '图片隐写', icon: 'Image', segment: true },
      { id: 'misc.audio', name: '音频隐写', icon: 'Music', segment: true },
      { id: 'misc.stego.video', name: '视频隐写', icon: 'Film', segment: true },
      { id: 'misc.stego.text', name: '文本隐写', icon: 'Type', segment: true },
      { id: 'misc.jail', name: '沙箱逃逸', icon: 'DoorClosed', segment: true, description: 'pyjail/bashjail 沙箱与受限环境逃逸' },
      { id: 'misc.textproc', name: '文本处理', icon: 'Wrench', segment: true, description: '排序/去重/替换/正则等' },
      { id: 'misc.datastat', name: '数据与位运算', icon: 'Activity', segment: true, description: '集合/位运算/统计/距离/熵' },
      { id: 'misc.datetime', name: '日期时间', icon: 'ScrollText', segment: true, description: '时间戳解析与格式转换' },
      { id: 'misc.format', name: '格式化美化', icon: 'FileCode', segment: true, description: 'JSON/XML/SQL/代码美化' },
      { id: 'misc.encoding', name: '编码杂项', icon: 'Binary', segment: true, description: 'UUID/TOTP/单位换算/权限解析等' },
      { id: 'misc.compress', name: '压缩与文件', icon: 'Folder', segment: true, description: 'ZIP/RAR 破解、Git 泄露、文件头' },
      { id: 'misc.visualize', name: '可视化', icon: 'LayoutGrid', segment: true, description: '密度图/散点/折线/热力图' },
      { id: 'misc.other', name: '其他', icon: 'MoreHorizontal', segment: true },
    ],
  },
  {
    id: 'reverse',
    name: 'Reverse',
    icon: 'FileCode2',
    description: '逆向工程',
    children: [
      { id: 'reverse.static', name: '静态分析', icon: 'FileSearch' },
      { id: 'reverse.dynamic', name: '动态分析', icon: 'Activity' },
      { id: 'reverse.deobfuscate', name: '去混淆', icon: 'Eraser' },
      // 变换还原:逆出变换逻辑后的套用/还原小工具(移位等),工具页 workspace 分发渲染
      { id: 'reverse.transform', name: '变换还原', icon: 'Binary', description: '移位等变换的套用与爆破还原' },
      // 逆向 Wiki:知识库工作区(常用函数/进制 ASCII/套路笔记),工具页由 workspace 分发渲染
      { id: 'reverse.wiki', name: '逆向 Wiki', icon: 'NotebookPen', description: '逆向知识库:函数速查、进制 ASCII、套路笔记' },
    ],
  },
  {
    id: 'pwn',
    name: 'Pwn',
    icon: 'Bug',
    description: '二进制漏洞利用',
    children: [
      // 批 9:纯分组分类(本层无直挂工具)按「分组 chips」呈现(migration-plan §10,同批 7/8 机制):
      // 父页 /c/pwn 顶部 chips(全部/栈溢出/堆利用/格式化字符串/Shellcode),「全部」= 域内全部 11 个工具
      // (10 个 pwn.* + 批 9 迁入的 misc.other.de-bruijn);4 条子分类路由保留;侧边栏 L2 节点保留。
      { id: 'pwn.stack', name: '栈溢出', icon: 'Layers', segment: true },
      { id: 'pwn.heap', name: '堆利用', icon: 'Blocks', segment: true },
      { id: 'pwn.format', name: '格式化字符串', icon: 'Percent', segment: true },
      { id: 'pwn.shellcode', name: 'Shellcode', icon: 'Code2', segment: true },
      { id: 'pwn.kernel', name: '内核', icon: 'Cpu', segment: true },
    ],
  },
  {
    id: 'forensics',
    name: '主机取证',
    icon: 'Server',
    description: '主机取证:内存取证、磁盘取证、日志分析',
    children: [
      // 批 15:本域从 CTF 模式内分类提升为独立模块「主机取证」(独立模式可见;CTF 模式共享该根,
      // 同 traffic 先例)。分类 id 保持 forensics → 工具 id/categoryId 零迁移,旧路由全部保留。
      // 批 7:纯分组分类(本层无直挂工具)按「分组 chips」呈现(migration-plan §8,复用批 4 FilterChip 机制):
      // 父页 /c/forensics 顶部 chips(全部/内存取证/磁盘取证/日志分析),「全部」= 分类下全部工具;
      // 原流量分析 segment 已独立为顶级 traffic 分类(22 个工具迁出,id 不变,仅 categoryId 迁移);
      // 3 条子分类路由保留(直接访问 /c/forensics.disk 等仍可达);侧边栏 L2 节点保留。
      { id: 'forensics.memory', name: '内存取证', icon: 'MemoryStick', segment: true },
      { id: 'forensics.disk', name: '磁盘取证', icon: 'HardDrive', segment: true },
      { id: 'forensics.log', name: '日志分析', icon: 'ClipboardList', segment: true },
    ],
  },
  // 2026-11-15 布局整改:stego 顶级分类撤销(隐写并入 Misc);osint 过度细分扁平化(6 工具直挂)。
  {
    id: 'osint',
    name: 'OSINT',
    icon: 'Eye',
    description: '开源情报',
  },
  {
    id: 'blockchain',
    name: 'Blockchain',
    icon: 'Coins',
    description: '区块链安全',
    children: [
      // 批 9:纯分组分类按「分组 chips」呈现(§10):父页 /c/blockchain chips(全部/链上分析/智能合约/钱包),
      // 「全部」= 域内全部 7 个工具;3 条子分类路由保留;侧边栏 L2 节点保留。
      { id: 'blockchain.chain', name: '链上分析', icon: 'Link2', segment: true },
      { id: 'blockchain.contract', name: '智能合约', icon: 'FileCode', segment: true },
      { id: 'blockchain.wallet', name: '钱包', icon: 'Wallet', segment: true },
    ],
  },
  {
    id: 'mobile',
    name: 'Mobile',
    icon: 'Smartphone',
    description: '移动安全',
    children: [
      // 批 9:纯分组分类按「分组 chips」呈现(§10):父页 /c/mobile chips(全部/Android/iOS),
      // 「全部」= 域内全部 4 个工具;2 条子分类路由保留;侧边栏 L2 节点保留。
      { id: 'mobile.android', name: 'Android', icon: 'Smartphone', segment: true },
      { id: 'mobile.ios', name: 'iOS', icon: 'Smartphone', segment: true },
    ],
  },
  {
    id: 'ai',
    name: 'AI',
    icon: 'Brain',
    description: 'AI 辅助与提示词安全',
    children: [
      // 批 9:纯分组分类按「分组 chips」呈现(§10):父页 /c/ai chips(全部/提示词安全/AI 分析),
      // 「全部」= 域内全部 3 个工具;2 条子分类路由保留;侧边栏 L2 节点保留。
      { id: 'ai.prompt', name: '提示词安全', icon: 'MessageSquareText', segment: true },
      { id: 'ai.analysis', name: 'AI 分析', icon: 'Sparkles', segment: true },
    ],
  },
  {
    id: 'data',
    name: '数据处理',
    icon: 'Database',
    description: '数据清洗、格式转换、去重统计与提取分析',
    children: [
      // 与 osint/ai 同为纯分组分类：父页 /c/data 顶部 chips(全部/数据清洗/格式转换/数据提取/统计与频次/数据爬虫)，
      // 「全部」= 域内全部工具；5 条子分类路由保留；侧边栏 L2 节点保留。
      { id: 'data.clean', name: '数据清洗', icon: 'Eraser', segment: true },
      { id: 'data.convert', name: '格式转换', icon: 'LayoutGrid', segment: true },
      { id: 'data.extract', name: '数据提取', icon: 'FileSearch', segment: true },
      { id: 'data.stats', name: '统计与频次', icon: 'Activity', segment: true },
      { id: 'data.crawl', name: '数据爬虫', icon: 'Network', segment: true },
    ],
  },
  // 渗透测试域：5 个原「渗透测试」子分类提升为顶级分类（渗透主项已去除），
  // 分类 id 保持不变，工具挂载（categoryId）不受影响。
  { id: 'pentest.recon', name: '信息收集', icon: 'Globe', description: '渗透测试·信息收集' },
  { id: 'pentest.scan', name: '资产探测', icon: 'Radar', description: '渗透测试·资产探测' },
  { id: 'pentest.vuln', name: '漏洞检测', icon: 'Bug', description: '渗透测试·漏洞检测' },
  { id: 'pentest.arsenal', name: '轻武器库', icon: 'Swords', description: '渗透测试·轻武器库' },
  { id: 'pentest.tunnel', name: '内网穿透', icon: 'Network', description: '渗透测试·隧道与内网穿透' },
  { id: 'pentest.assist', name: '辅助工具', icon: 'Wrench', description: '渗透测试·辅助工具' },
  // 木马管理域(批 16):Webshell 连接管理、木马生成免杀、监听会话、检测痕迹。
  // 纯分组分类按「分组 chips」呈现(同 pwn/forensics 机制):父页 /c/pentest.trojan 顶部 chips,
  // 「全部」= 域内全部工具;4 条子分类路由保留;侧边栏 L2 节点保留。
  {
    id: 'pentest.trojan',
    name: '木马管理',
    icon: 'Skull',
    description: '渗透测试·木马管理:Webshell 连接管理、木马生成免杀、监听会话、检测痕迹',
    children: [
      { id: 'pentest.trojan.webshell', name: 'Webshell 管理', icon: 'Ghost', segment: true, description: '连接管理/命令执行/文件与数据库/隧道' },
      { id: 'pentest.trojan.generate', name: '生成与免杀', icon: 'Wand2', segment: true, description: '一句话/EXE/Loader 生成、混淆与免杀自检' },
      { id: 'pentest.trojan.listener', name: '监听与会话', icon: 'RadioTower', segment: true, description: '监听器/会话/任务下发/流量伪装' },
      { id: 'pentest.trojan.detect', name: '检测与痕迹', icon: 'ScanSearch', segment: true, description: 'Webshell 检测/YARA/痕迹清理/持久化' },
    ],
  },
  // 代理拦截域(批 17):Burp 风格 HTTP 代理(捕获/拦截修改/放行丢弃/历史/重放),workspace 工具直挂本节点
  { id: 'pentest.proxy', name: '代理拦截', icon: 'Waypoints', description: '渗透测试·HTTP 代理拦截与重放(Burp 风格)' },
  // AI 渗透：一级分类（自动化渗透任务 + 对话式渗透），
  // 与 pentest.* 同级的独立顶级节点，两种模式均可见。
  { id: 'ai.pentest', name: 'AI 渗透', icon: 'Bot', description: 'AI 自动化渗透与对话式渗透测试' },
  // 模型安全：大模型安全测试域（越狱 / 提示注入 / 投毒 / 后门），
  // 与 pentest.* 同级顶级节点，模型安全模式可见。
  { id: 'aisec.jailbreak', name: '越狱攻击', icon: 'Bomb', description: '模型安全·越狱与安全边界测试' },
  { id: 'aisec.injection', name: '提示注入', icon: 'MessageSquareText', description: '模型安全·提示注入与指令劫持' },
  { id: 'aisec.poison', name: '数据投毒', icon: 'Database', description: '模型安全·投毒筛查与后门样本生成' },
  { id: 'aisec.backdoor', name: '模型后门', icon: 'HardDrive', description: '模型安全·模型文件后门与恶意载荷检测' },
  { id: 'aisec.adversarial', name: '对抗样本', icon: 'Swords', description: '模型安全·对抗样本生成与鲁棒性评估' },
  // 流量分析域:原 forensics.network 二级 segment 独立成顶级分类
  //(流量分析模式可见;CTF 模式共享该根——流量题本是取证子域)。
  // 22 个工具由 forensics.network 迁入(工具 id 不变,仅 categoryId 迁移)。
  {
    id: 'traffic',
    name: '流量分析',
    icon: 'Radio',
    description: '流量取证与协议分析',
    children: [
      { id: 'traffic.parse', name: '流量解析', icon: 'FileSearch', description: 'PCAP 文件与原始流量解析' },
      { id: 'traffic.protocol', name: '协议解码', icon: 'Network', description: 'TCP/UDP/DNS/ICMP 等协议字段解码' },
      { id: 'traffic.fingerprint', name: '流量指纹', icon: 'Search', description: 'JA3/HASSH/User-Agent 等指纹生成' },
      { id: 'traffic.assist', name: '辅助处理', icon: 'Wrench', description: 'IP 段聚合、MAC 格式化、Defang 等' },
    ],
  },
];
