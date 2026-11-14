/**
 * 逆向 Wiki —— 分类定义 + 内置速查数据(种子)。
 *
 * 分类 id 与 internal/wiki/reverse_seed.go 的 ReverseBuiltinSeed 一一对应;
 * 内置数据在桌面端由后端首次打开时灌入 reverse_wiki.db,在浏览器开发模式
 * 由 lib/reverseWiki.ts 首次使用时灌入 localStorage,两者内容保持一致。
 * markdown 约定:代码块用 4 空格缩进,内容不含反引号(与 Go 侧 raw string 对齐)。
 */
import type { WikiCategory, WikiSeedEntry } from './redteamWiki';

export const REVERSE_WIKI_CATEGORIES: WikiCategory[] = [
  { id: 'algo', name: '算法与特征识别', icon: 'Binary', description: 'TEA/RC4/Base64 变表、哈希常数' },
  { id: 'tools', name: '工具操作', icon: 'Wrench', description: 'IDA / Ghidra / x64dbg 速查' },
  { id: 'antidebug', name: '反调试与对抗', icon: 'ShieldCheck', description: '调试检测手法与绕过' },
  { id: 'unpack', name: '壳与脱壳', icon: 'Boxes', description: 'UPX / ESP 定律 / 壳特征' },
  { id: 'formats', name: '文件格式', icon: 'FileCode', description: 'ELF / PE / pyc / APK 要点' },
  { id: 'functions', name: '常用函数速查', icon: 'Terminal', description: 'MSVC CRT / Win32 API / libc 见名知义' },
  { id: 'ascii', name: '进制与 ASCII', icon: 'Search', description: '码点对照、小端读法、文件头' },
  { id: 'notes', name: '套路与笔记', icon: 'NotebookPen', description: '解题流程与个人踩坑记录' },
];

/** 按 id 取分类(未知返回 undefined) */
export function reverseCategoryById(id: string): WikiCategory | undefined {
  return REVERSE_WIKI_CATEGORIES.find((c) => c.id === id);
}

export const REVERSE_WIKI_SEED: WikiSeedEntry[] = [
  // —— 算法与特征识别 ——
  {
    category: 'algo',
    title: 'TEA/XTEA 特征',
    content: `## 识别要点

- 常数 **delta = 0x9E3779B9**(黄金分割)几乎必现;累加变量 sum 每轮 += delta。
- 32 轮循环,v0/v1 交替更新,移位组合 (v1<<4)/(v1>>5) 与密钥 k0-k3 异或相加:

    v0 += ((v1 << 4) + k0) ^ (v1 + sum) ^ ((v1 >> 5) + k1);
    v1 += ((v0 << 4) + k2) ^ (v0 + sum) ^ ((v0 >> 5) + k3);

- XTEA 多了轮密钥索引 (sum & 3)、(sum >> 11 & 3);XXTEA 变长块。
- 解密即逆序轮、sum 从 delta*32 回退。密钥 128 位,常见硬编码在 .data 段 16 字节。`,
  },
  {
    category: 'algo',
    title: 'RC4 结构特征',
    content: `## 识别要点

- KSA(密钥调度):256 次循环内 j = (j + S[i] + key[i % keylen]) & 0xFF 后交换 S[i]/S[j]。
- PRGA(出流):i = (i+1) & 0xFF; j = (j+S[i]) & 0xFF; 交换; K = S[(S[i]+S[j]) & 0xFF]。
- 特征常量:0x100 / 0xFF、S 盒初始化为 0..255 顺序填充的循环。
- 常见魔数 S 盒首字节校验、密钥常在字符串附近;变体:RC4-drop[n](丢弃前 n 字节)。`,
  },
  {
    category: 'algo',
    title: 'Base64 变表识别',
    content: `## 识别要点

- 标准表(下划线记法)ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/,填充 = 。
- 变表题:在 strings/数据段找一段 **64 字符长、大小写数字混合、字符不重复** 的串,即为自定义表。
- 换表后密文特征不变(4:3 长度比、尾部 =);解法:把密文按位置映射回标准表再解。
- 兄弟编码:Base32(A-Z2-7)、Base58(去 0OIl)、Base85/xxencoding;URL 变体 -_ 替换 +/。`,
  },
  {
    category: 'algo',
    title: 'MD5/SHA 初始常数',
    content: `## 初始状态(小端写法按实现而定)

- MD5 四寄存器:0x67452301, 0xEFCDAB89, 0x98BADCFE, 0x10325476;T 表含 0xD76AA478 等 64 个。
- SHA1 五寄存器:0x67452301, 0xEFCDAB89, 0x98BADCFE, 0x10325476, 0xC3D2E1F0;轮常数 0x5A827999 / 0x6ED9EBA1 / 0x8F1BBCDC / 0xCA62C1D6。
- SHA256 初始 H:0x6a09e667, 0xbb67ae85, 0x3c6ef372, 0xa54ff53a...(K 表首项 0x428a2f98)。
- 题目里看到这些常数直接定位哈希函数;注意 SM3(0x7380166F)与 SHA3(Keccak, 0x01 舍入)区分。`,
  },
  // —— 工具操作 ——
  {
    category: 'tools',
    title: 'IDA 快捷键速查',
    content: `## 高频键位

- F5 反编译 / Tab 切换视图;N 重命名;Y 改类型;X 交叉引用;G 跳地址。
- ESC 回退;Ctrl+Z 撤销;分号 注释;Ins/A 加入数组。
- Shift+F12 字符串窗口;Ctrl+F 搜索;Alt+T 文本搜索;Alt+B 二进制搜索。
- D 切换数据类型(字节/字/双字);C 转代码;U 取消定义;P 建函数。
- Ctrl+S 保存 idb;Alt+E 入口点;Ctrl+P 跳函数。`,
  },
  {
    category: 'tools',
    title: 'IDA Python 常用片段',
    content: `## 速查

- 读/写内存字节:

    ea = idaapi.get_screen_ea()
    idaapi.patch_byte(ea, 0x90)

- 遍历函数与改名:

    for f in idautils.Functions():
        print(hex(f), idc.get_func_name(f))
    idc.set_name(ea, "new_name")

- 交互动解题(题库标准模板):

    def q(s): return idaapi.ask_str(0, 0, s)
    a = q("input:"); idaapi.msg("got %s\\n" % a)

- 搜索立即数:idaapi.find_binary / idc.find_text;脚本目录 %IDA%/plugins 或 Alt+F7 运行。`,
  },
  {
    category: 'tools',
    title: 'Ghidra 速查',
    content: `## 流程与要点

- 导入后 Auto-Analyze 全勾(尤其 Decompiler Parameter ID);F5 等价 = Decompiler 窗口自动有。
- L 重命名;T 改类型;R 引用;G 跳转;分号 注释。
- Script Manager(Ctrl+3 搜索)内置 GhidraScript;导出:C 语言 File -> Export Program。
- Headless 批处理:

    analyzeHeadless <proj_dir> <proj> -import <bin> -postScript MyScript.java

- 中文注释乱码:Options -> Encoding 改 UTF-8。`,
  },
  {
    category: 'tools',
    title: 'x64dbg 常用操作',
    content: `## 键位与命令

- F2 断点;F7 步入;F8 步过;F9 运行到断点;Ctrl+F9 执行到返回;Ctrl+G 跳转。
- 命令行:bp <addr>、bc 清断点、bp GetProcAddress(API 名断点)、Find 搜索。
- 硬件断点:bp 后 Set Hardware on access;内存断点:右键段 -> 断点。
- 常用场景:程序入口停(系统断点/EP),Follow in Dump 看数据,脚本 tab 记录。
- 配 Scylla 插件脱壳 dump + 修 IAT;中文界面自带。`,
  },
  // —— 反调试与对抗 ——
  {
    category: 'antidebug',
    title: 'IsDebuggerPresent',
    content: `## 原理与对抗

- kernel32!IsDebuggerPresent 读 PEB->BeingDebugged(TEB+0x30 -> PEB+0x2,x86 与 x64 同偏移 0x2)。
- 静态识别:直接 call IsDebuggerPresent,或内联读 fs:[0x30]/gs:[0x60] 后 test byte ptr [eax+2]。
- 绕过:x64dbg 插件(ScyllaHide 等) hook 返回 0;手动把 PEB 该字节改 0;或 NOP 掉检测分支。
- 姊妹 API:CheckRemoteDebuggerPresent(句柄版,可查父进程),NtSetInformationThread(HideFromDebugger)。`,
  },
  {
    category: 'antidebug',
    title: 'NtQueryInformationProcess',
    content: `## 调试检测主力

- ntdll 导出,ProcessInformationClass 关键值:
  - 7 (ProcessDebugPort):返回非 0 即被调试。
  - 0x1E (ProcessDebugObjectHandle):调试对象句柄非空即被调试。
  - 0x1F (ProcessDebugFlags):返回 0 表示被调试(注意反逻辑)。
- 识别:CallWindowProc 前的大 switch/比较;常配合 NtQuerySystemInformation(SystemKernelDebuggerInformation)。
- 对抗:hook 该函数改返回值,或断在 ntdll 导出直接改 eax。`,
  },
  {
    category: 'antidebug',
    title: '时间差检测',
    content: `## 原理

- 断点/单步会把 INT3 异常交给调试器,执行耗时暴增;程序取两时点差值判断。
- 常用源:rdtsc 指令(返回 64 位周期数)、GetTickCount、GetLocalTime、QueryPerformanceCounter。
- 识别:两处取时点间夹着关键逻辑,差值与阈值(常见 0x3E8/大数)比较。
- 对抗:让两次取值相等——在第二次取值处下断点把寄存器/返回值改回第一次;或跳过比较分支。`,
  },
  {
    category: 'antidebug',
    title: 'OutputDebugString 反调',
    content: `## 原理(x86 经典)

- 无调试器时 OutputDebugStringA 返回后 GetLastError 变为 ERROR_INVALID_PARAMETER(87);
  有调试器时错误码不同——程序据此判断。
- 另一变体:利用 INT 2D(调试器存在时吞掉异常,否则触发)。
- 对抗:ScyllaHide 自动处理;手动在调用后强行设置/清除 GetLastError。`,
  },
  {
    category: 'antidebug',
    title: '反反调试手法',
    content: `## 工具箱

- ScyllaHide(x64dbg 插件):勾 PEB BeingDebugged/NtQueryInformationProcess/OutputDebugString/块 rdtsc 等,一键绕大多数。
- IDA 侧:静态 NOP 掉检测分支最稳(改跳转为 jmp/nop);注意多重检测与异或校验自修改。
- VMP/Themida 深壳:先用 x64dbg trace 找 OEP,别硬刚反调试;必要时虚拟机+内核级 TitanHide。
- 题目常见组合:IsDebuggerPresent + 时间差 + 异常(INT3/INT1)三连,逐个击破。`,
  },
  // —— 壳与脱壳 ——
  {
    category: 'unpack',
    title: 'UPX',
    content: `## 识别与脱壳

- 特征:节区名 UPX0/UPX1/UPX2(或 UPX!),入口在 UPX1;strings 见 "UPX 3.9x" 版本串。
- 一键脱壳(未改壳):

    upx -d packed.exe -o unpacked.exe

- 改壳(改 magic/节名)时:用 x64dbg ESP 定律到 OEP,Scylla dump + 修 IAT。
- 嵌套 UPX:连续 upx -d;报 "CantUnpackVersion" 说明手改过,走手动。`,
  },
  {
    category: 'unpack',
    title: 'ESP 定律手脱',
    content: `## 步骤(通用压缩壳)

1. 载入 x64dbg,停在系统断点/EP(壳的 EP,不是程序)。
2. F8 单步,观察 ESP 寄存器首次显著变化(典型:pushad 后 ESP 下移)——右键 ESP -> 数据窗口跟随 -> 该地址下硬件访问断点(4 字节)。
3. F9 运行,断在 popad 之后(壳解压完还原寄存器),此时通常已到/接近 OEP。
4. 若停在壳内 DLL 调用,再 F8 几步直到程序真实代码(能看出主逻辑/有正常调用约定)。
5. Scylla(或 x64dbg 插件):选进程 -> IAT 自动搜索 -> 获取导入表 -> 修复转储 -> Dump。
6. 修好 IAT 的 dump 即可静态分析;必要时 LoadConfig/重定位手工补。`,
  },
  {
    category: 'unpack',
    title: '常见壳节区名特征',
    content: `## 速查表

- UPX0/UPX1 —— UPX;UPX! 魔数。
- .aspack/.adata —— ASPack;.adata 也见于其他。
- .petite —— Petite;.upx 变体。
- .themida / .taggant —— Themida(重型,虚拟化);配套 WinLicense。
- .vmp0/.vmp1 —— VMProtect;入口 push 进 vm。
- .nsp0/.nsp1 —— NsPacK;.enedec/.enigma1 —— Enigma。
- .mpress —— MPRESS;.pebundle —— PEBundle。
- 无名/乱名节 + 入口在最后一节 + 节属性 RWX —— 疑似壳/自解密。`,
  },
  // —— 文件格式 ——
  {
    category: 'formats',
    title: 'ELF 头要点',
    content: `## 关键字段(64 位)

- 魔数 0x7F 45 4C 46(\\x7fELF);EI_CLASS: 1=32 位 2=64 位;EI_DATA: 1=小端。
- e_type: 2=ET_EXEC(静态可执行) 3=ET_DYN(PIE/共享库);e_entry 入口虚拟地址;e_phoff/e_shoff。
- e_shstrndx 定节名表;工具:readelf -h / -S / -l、objdump -d、patchelf 改解释器。
- CTF 常考点:入口被改、节表被抹(重建)、静态链接大体积(glibc 静态,符号全在)。`,
  },
  {
    category: 'formats',
    title: 'PE 结构要点',
    content: `## 布局速记

- DOS 头 MZ,e_lfanew(0x3C)指向 PE 签名;NT 头:签名 PE + FileHeader + OptionalHeader。
- OptionalHeader:AddressOfEntryPoint(RVA)、ImageBase、SectionAlignment/FileAlignment、DataDirectory(导入表第 2 项、导出第 1 项)。
- 节表后跟节数据;.text 代码 .data 数据 .rdata 只读 .rsrc 资源 .reloc 重定位。
- 导入表(IAT)看程序用了哪些 API(判功能/壳);工具:CFF Explorer、pestudio、dumpbin /imports。`,
  },
  {
    category: 'formats',
    title: 'pyc 反编译流程',
    content: `## 版本与工具链

- pyc 头:4 字节 magic + 4 字节时间戳/flags;Python 3.7+ 头 16 字节(4 magic + 4 bit field + 8 时间/源大小)。
- 按版本选工具:
  - uncompyle6:2.4~3.8;pip install uncompyle6 后 uncompyle6 x.pyc > x.py。
  - decompyle3:3.7~3.8 效果更好。
  - pycdc(Decompyle++):全版本兜底,编不动时报错码也常够用。
- 坑:magic 不匹配报 Bad magic number——确认 Python 版本(文件前两字节查版本表);3.9+ 优先 pycdc。
- 无 pyc 只有 py:pyinstxtractor 解包 PyInstaller exe 得 pyc,再走上面链路(补 16 字节头)。`,
  },
  {
    category: 'formats',
    title: 'APK 逆向流程',
    content: `## 标准链路

1. 改 zip 后缀/unzip 直接解(dex/资源/签名在里面)。
2. jadx-gui 拖入:Java 源码级浏览(比 apktool 反编译快);搜字符串/URL/关键类。
3. apktool d app.apk:smali + 资源(AndroidManifest 权限/组件);改逻辑后 apktool b 回编 + 签名(apksigner / jarsigner)。
4. so 层(IDA 打开 lib/*.so)看 JNI:JNI 函数名 Java_包名_类名_方法名 逐段下划线。
5. 动态:frida -U -f 包名 -l hook.js(hook Java 层 Java.use / Native 层 Interceptor.attach);objection 一键。
6. 常见考点:so 白盒加密、资源文件伪装(stego 配合)、assets 里藏 dex/密钥。`,
  },
  // —— 常用函数速查 ——
  {
    category: 'functions',
    title: '_acrt_iob_func',
    content: `## MSVC CRT:标准流句柄

- 原型:FILE* __cdecl _acrt_iob_func(unsigned index)。
- index: 0=stdin, 1=stdout, 2=stderr——返回对应 FILE* 句柄。
- 出现场景:printf/puts/scanf 等标准流函数链条的第一步;DLL 里用 CRT 又想拿标准流时直接调它。
- 老版本对应 __acrt_iob_func 与 _iob;GCC/MinGW 侧为 __iob_func 或 __imp___acrt_iob_func。
- 逆向含认:看到它传 0/1/2,后面 vfprintf 系列就是在打 stdin/stdout/stderr。`,
  },
  {
    category: 'functions',
    title: '__stdio_common_vfprintf',
    content: `## MSVC CRT:printf 家族真身(UCRT)

- 原型(x64):

    int __stdio_common_vfprintf(
        unsigned __int64 options, FILE* stream,
        char const* format, __crt_locale_pointers* locale, va_list args)

- printf/vprintf/sprintf/snprintf/fprintf/_printf_s 全家在 UCRT 里都转发到它(options 区分 normal/_s/宽字符 __stdio_common_vfwprintf)。
- 逆向认法:一个函数被 printf 类大量调用、第五参是 va_list——就是它;改 format 参数常用于改输出。
- 旧 MSVC(CRT 而非 UCRT)对应 _vfprintf_ex/_vsnprintf;MinGW 对应 __mingw_vprintf。`,
  },
  {
    category: 'functions',
    title: 'memcpy/memset 族',
    content: `## 认法与变体

- memcpy(dst, src, n):不支持重叠;重叠场景应 memmove。
- memset(dst, val, n):常见 val=0 清缓冲;识别为 rep stosb/q。
- memcmp(a, b, n):比较,返回 0 相等——**CTF 高频**(比较 flag/密钥处,patch 它或看比较源)。
- 汇编:x64 常内联为 rep movsb / SIMD(__memcpy_avx_unaligned);Android so 里 __memcpy_chk。
- Win32 宏等价:RtlCopyMemory/RtlFillMemory = CopyMemory/FillMemory。`,
  },
  {
    category: 'functions',
    title: '__security_check_cookie',
    content: `## MSVC 栈保护(GS)

- 函数序言 mov eax, __security_cookie; xor eax, ebp; mov [ebp-4], eax;尾声 call __security_check_cookie 校验。
- cookie 被 XOR 进局部变量区,若栈溢出改写则校验失败走 __report_gsfailure/terminate。
- 新版 MSVC 以 __GSHandlerCheck(SEH 表驱动)替代逐函数 call,看到 SEH 元数据里的 GSHandler 即同义。
- 逆向含认:这是编译器生成的栈保护,不是业务逻辑,patch 时注意保持 cookie 一致(改字节别跨)。`,
  },
  {
    category: 'functions',
    title: 'strXX 字符串族',
    content: `## 速查

- strcpy/strcat:无长度检查,溢出题常客;安全版 strncpy/strncat(注意也不保证 \\x00 结尾)。
- strlen/strcmp/strncmp:比较逻辑高发区——**strcmp 返回 0 表相等**,CTF 常在此 patch。
- strstr(s, sub) 返回子串指针(NULL 未找到);strchr 找单字符。
- 宽字符族:wcscpy/wcslen/wcscmp(UTF-16,Windows API 配套);lstrcpy/lstrlen 是 Win32 宏同物。
- 安全 CRT:_s 后缀(strcpy_s)与 _chk(__strcpy_chk,GCC/Android 带 __bos)。`,
  },
  {
    category: 'functions',
    title: 'Win32 文件 API',
    content: `## 常用与认法

- CreateFileA/W:打开/创建文件与设备,返回 HANDLE;参数 (路径, 访问 GENERIC_READ/WRITE, 共享, 安全, CREATE_OPEN_ALWAYS, 属性, 0)。INVALID_HANDLE_VALUE(-1) 失败。
- ReadFile/WriteFile:(句柄, 缓冲, 长度, &实际长度, OVERLAPPED);CloseHandle 收尾。
- GetFileSize/SetFilePointer 定位;CreateFileMappingA + MapViewOfFile 内存映射(大文件/自解析)。
- 认法:字符串表里的文件名/扩展名(.dll/.dat/.key)常是这些 API 的参数,顺藤摸配置与密钥。
- 姊妹:DeleteFileA、MoveFileA、FindFirstFileA(目录遍历,勒软/搜集行为标志)。`,
  },
  {
    category: 'functions',
    title: 'Win32 注册表 API',
    content: `## 常用与认法

- RegOpenKeyExA/HKEY 打开:HKEY_LOCAL_MACHINE/HKEY_CURRENT_USER 常量 0x80000002/0x80000001。
- RegQueryValueExA 读值;(键名, 值名, 类型, 缓冲, &长度);RegSetValueExA 写值。
- RegCreateKeyExA 建;RegDeleteKeyA/RegDeleteValueA 删;RegCloseKey 关。
- 认法:软件自启动(Run 键)、持久化、配置读取;字符串 "Software\\\\xxx" 与 "Run" 是强特征。
- wow64 重定向:32 位进程访问 HKLM\\\\System32 会被转到 Wow6432Node,取证时注意。`,
  },
  {
    category: 'functions',
    title: 'Win32 进程 API',
    content: `## 常用与注入三件套

- CreateProcessA/W 起进程;(lpApplicationName, 命令行, ..., STARTUPINFO, &PROCESS_INFORMATION)。
- OpenProcess(访问权, 继承, PID) 拿句柄;PROCESS_ALL_ACCESS = 0x1F0FFF。
- **注入三件套**:VirtualAllocEx(目标进程申请) → WriteProcessMemory(写 DLL 路径/shellcode) → CreateRemoteThread(目标进程起线程,起点 LoadLibraryA)。看到三连即注入器。
- 枚举:CreateToolhelp32Snapshot + Process32First/Next(TH32CS_SNAPPROCESS);EnumProcesses + EnumProcessModules(PSAPI)。
- 其它:TerminateProcess 结束;GetCurrentProcess 返回伪句柄 -1;GetCurrentProcessId。`,
  },
  {
    category: 'functions',
    title: 'Win32 网络 API',
    content: `## 两套栈与认法

- socket 栈:WSAStartup(MAKEWORD(2,2), &wsa) 初始化 → socket(AF_INET=2, SOCK_STREAM=1) → connect( SOCKADDR_IN{htons(port), inet_addr(ip)} ) → send/recv。
- 认法:inet_addr/inet_ntoa 参数里的 IP 字符串、htons 的端口(字节序翻转后的值),直接读 C2 地址。
- WinInet 栈:InternetOpenA → InternetConnectA → HttpOpenRequestA → HttpSendRequestA → InternetReadFile;URLDownloadToFileA 从 URL 拉文件(下载器标志)。
- WINHTTP 族是宽字符版;回调 InternetStatusCallback 常被 C2 用作状态机载体。`,
  },
  {
    category: 'functions',
    title: 'Win32 内存与加载 API',
    content: `## 常用与认法

- VirtualAlloc(地址可 NULL, 大小, MEM_COMMIT|MEM_RESERVE, 保护):PAGE_EXECUTE_READWRITE(0x40)= RWX 申请,**shellcode 加载强特征**。
- LoadLibraryA/W + GetProcAddress:动态解析 API;字符串表里的 DLL 名 + 序号/名称表即解析链(壳与注入器标配)。
- HeapAlloc(GetProcessHeap(), 0, size)/HeapFree:堆分配;GlobalAlloc/LocalAlloc 老代码。
- VirtualProtect 改页属性(自修改代码/解密段);ReadProcessMemory/WriteProcessMemory 跨进程读写。
- GetProcAddress 返回值常被存到全局函数指针表——逆向时把表项命名回 API 名。`,
  },
  {
    category: 'functions',
    title: 'libc I/O 函数',
    content: `## ELF 侧速查

- 文件:open(路径, O_RDONLY=0) 返回 fd;read(fd, buf, n)/write(fd, buf, n);close(fd)。open64/lseek64 大文件。
- 标准:FILE* fopen/fread/fwrite/fclose;puts/printf/write(1,...) 输出;fgets 读行。
- 网络:send/recv(sockfd,...) 与 read/write 同签名(socket 即 fd);recv 返回 0 对端关闭。
- 认法:pwn 题 gadget 找 write(1, buf, n) 泄露;逆向题 __libc_start_main 是 main 的调用者(交叉定位入口)。
- mmap(PROT_READ|WRITE|EXEC) 匿名映射 RWX —— shellcode/自解密特征。`,
  },
  {
    category: 'functions',
    title: 'libc system/popen',
    content: `## CTF 高频

- system(cmd):fork+exec /bin/sh -c;返回退出码。逆向见到 system 且参数可控 → 命令注入/RCE 点;pwn 题 ret2system 常客(配 pop rdi gadget)。
- popen(cmd, "r"/"w"):起 shell 并建管道读输出;fgets 读结果。
- execve(path, argv, envp) 直接替换进程映像(不返回);execve("/bin/sh",0,0) 是 orw/ret2syscall 目标。
- 认法:命令字符串常量("/bin/sh"、"curl"、"chmod")与这些函数同现;字符串若是拼接格式串则先追拼接逻辑。`,
  },
  // —— 进制与 ASCII ——
  {
    category: 'ascii',
    title: 'ASCII 可打印字符对照表',
    content: `## 分段速记(0x20–0x7E 可打印)

- 0x20 空格;0x21-0x2F 符号 !"#$%&'()*+,-./;0x30-0x39 数字 0-9;0x3A-0x40 符号 :;<=>?@。
- 0x41-0x5A 大写 A-Z;0x5B-0x60 符号(含 [ ] ^ _ 与反引号);0x61-0x7A 小写 a-z;0x7B-0x7E 符号 {|}~。
- 大小写差 0x20(A=0x41, a=0x61);数字转字符:+0x30。
- 控制区高频:0x00 NUL;0x0A LF 换行;0x0D CR 回车;0x09 TAB;0x1B ESC;0x7F DEL。
- 例:0x64='d', 0x66='f', 0x6C='l', 0x61='a', 0x67='g' → 66 6C 61 67 即 "flag"。
- 工具:逆向 Wiki 顶部「进制↔ASCII 查询条」即输即得(支持 0x 前缀/十进制/批量/URL)。`,
  },
  {
    category: 'ascii',
    title: '小端立即数读法与高频值',
    content: `## 小端(x86)读法

- 内存低位在前:字节序 64 00 00 00 读作 DWORD 0x00000064=100。
- 字符串型立即数反向拼:比较 0x00646362 时,内存是 62 63 64 00,读作字符串 bcd(尾 NUL);四字符魔数直接倒读:
  - 0x64636261 读作 abcd;0x5A4D 读作 MZ(PE 头);7F 45 4C 46 即 ELF 魔数字节。
- 高频文件头(字节序):ZIP 50 4B("PK");PNG 89 50 4E 47;GIF 47 49 46;RAR 52 61 72 21;PDF 25 50 44 46("%PDF");7z 37 7A BC AF。
- 逆向题套路:一串 DWORD 比较倒读成字符串即为校验目标;memcmp 一段常量按字节读即可。
- 混淆变体:每字节 +/- 固定偏移(如 +1)后才是可打印区,先减再读。`,
  },
  // —— 套路与笔记 ——
  {
    category: 'notes',
    title: 'CTF 逆向通用流程',
    content: `## 通用套路(遇题先过一遍)

1. 定性:file/魔数/节区 → 语言与平台(PE/ELF/pyc/APK/壳)。
2. 表面信息:strings(ASCII+宽字符)、导入表(功能画像)、资源段。
3. 定位关键逻辑:main/WinMain入口、字符串交叉引用("flag"/"wrong"/提示语)、大比较函数。
4. 识别算法:套特征常数(TEA/RC4/MD5...);自定义混淆则先跟数据流。
5. 静态推不动就动态:关键断点(memcmp/strcmp/关键分支),看比较的明文与密文。
6. 解题三路:还原算法离线算 / patch 条件跳转(直接过校验)/ 动态改内存。
7. 收尾:遇到新函数/新特征,随手记进本 Wiki「常用函数」——下次秒认。

- 踩坑记录区(自填):在此追加本题新学到的函数、常数、套路。`,
  },
];
