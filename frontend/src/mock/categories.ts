import type { CategoryNode } from '@/types';

/**
 * 分类树 mock 数据。
 *
 * 结构与后端 `app/core/tool.py` 的 Category 枚举对齐（web / pwn / reverse /
 * crypto / misc / forensics / mobile / blockchain / osint / ai），并额外拆分出
 * Stego（隐写）作为独立大类。支持最多 4 层子分类。
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
        ],
      },
      { id: 'crypto.hash', name: '哈希', icon: 'Hash', description: '哈希与相关攻击' },
      { id: 'crypto.auto-decode', name: '自动解码', icon: 'Zap', description: '暴力尝试所有常见解码方式' },
    ],
  },
  // 批 1 新增一级分类(ia.md §1.2 codec 拆分):先落 codec.base(base64 已迁入),其余编码工具批 2 平移
  {
    id: 'codec',
    name: '编码转换',
    icon: 'Binary',
    description: '无损格式转换:Base 家族、进制、转义、字符集等',
    children: [
      { id: 'codec.base', name: 'Base 家族', icon: 'Binary', description: 'Base2~Base92 全系列' },
    ],
  },
  {
    id: 'misc',
    name: 'Misc',
    icon: 'Puzzle',
    description: '杂项与综合',
    children: [
      // 批 10:分组 chips 按本段所含 L2(§11「other/demo/report 中本段所含」= other + demo):
      // 父页 /c/misc 顶部 chips(全部/其他/示例演示),「全部」= 域内全部 99 个工具
      // (misc.other 93 + misc.demo 4 + misc.report 2;批 11 的 47 个 other 工具与 report 2 个
      //  自批 2 起即可浏览,本批不新增可见范围);misc.report 保持子分类入口卡片(批 11 收口);
      // 2 条子分类路由保留;侧边栏 L2 节点保留。
      { id: 'misc.demo', name: '示例演示', icon: 'PlayCircle', segment: true },
      { id: 'misc.report', name: '报告 / 工具', icon: 'FileText' },
      // 音频隐写:WAV LSB 位平面提取等(2026-11 新增,入口卡片形式,工具挂 misc.audio)
      { id: 'misc.audio', name: '音频隐写', icon: 'Music', description: 'WAV LSB 位平面隐写提取与类型自动识别' },
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
    ],
  },
  {
    id: 'forensics',
    name: 'Forensics',
    icon: 'Search',
    description: '数字取证',
    children: [
      // 批 7:纯分组分类(本层无直挂工具)按「分组 chips」呈现(migration-plan §8,复用批 4 FilterChip 机制):
      // 父页 /c/forensics 顶部 chips(全部/内存取证/磁盘取证/日志分析),「全部」= 分类下全部 32 个工具;
      // 原流量分析 segment 已独立为顶级 traffic 分类(22 个工具迁出,id 不变,仅 categoryId 迁移);
      // 3 条子分类路由保留(直接访问 /c/forensics.disk 等仍可达);侧边栏 L2 节点保留。
      { id: 'forensics.memory', name: '内存取证', icon: 'MemoryStick', segment: true },
      { id: 'forensics.disk', name: '磁盘取证', icon: 'HardDrive', segment: true },
      { id: 'forensics.log', name: '日志分析', icon: 'ClipboardList', segment: true },
    ],
  },
  {
    id: 'stego',
    name: 'Stego',
    icon: 'Image',
    description: '隐写术',
    children: [
      // 批 8:纯分组分类(本层无直挂工具)按「分组 chips」呈现(migration-plan §9,同批 7 机制):
      // 父页 /c/stego 顶部 chips(全部/图片/音频/视频/文本),「全部」= 分类下全部 37 个工具
      // (34 占位 + 批 1 迁入的 3 个已实现);4 条子分类路由保留;侧边栏 L2 节点保留。
      { id: 'stego.image', name: '图片隐写', icon: 'Image', segment: true },
      { id: 'stego.audio', name: '音频隐写', icon: 'Music', segment: true },
      { id: 'stego.video', name: '视频隐写', icon: 'Film', segment: true },
      { id: 'stego.text', name: '文本隐写', icon: 'Type', segment: true },
    ],
  },
  {
    id: 'osint',
    name: 'OSINT',
    icon: 'Eye',
    description: '开源情报',
    children: [
      // 批 9:纯分组分类按「分组 chips」呈现(§10):父页 /c/osint chips(全部/社交账号/地理位置/图片情报/域名 / 网络),
      // 「全部」= 域内全部 6 个工具;4 条子分类路由保留;侧边栏 L2 节点保留。
      { id: 'osint.social', name: '社交账号', icon: 'Users', segment: true },
      { id: 'osint.geo', name: '地理位置', icon: 'MapPin', segment: true },
      { id: 'osint.image', name: '图片情报', icon: 'Camera', segment: true },
      { id: 'osint.domain', name: '域名 / 网络', icon: 'Globe2', segment: true },
    ],
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
