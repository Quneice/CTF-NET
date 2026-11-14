// 敏感函数种子规则:按语言分组,覆盖 CTF/渗透审计高频危险 sink。
// Pattern 为 Go regexp(RE2,无 lookaround;用 \b 与 \s*\( 表达函数调用形态)。
package audit

import "database/sql"

type seedFunc struct {
	language, pattern, category, risk, note string
}

var seedFuncs = []seedFunc{
	// —— PHP ——
	{"php", `\beval\s*\(`, "代码执行", "high", "eval 直接执行任意代码"},
	{"php", `\bassert\s*\(`, "代码执行", "high", "assert 在 PHP7 前可执行代码"},
	{"php", `\b(preg_replace)\s*\(.{0,64}/e`, "代码执行", "high", "preg_replace /e 修饰符执行代码(PHP<7)"},
	{"php", `\bcreate_function\s*\(`, "代码执行", "high", "create_function 已废弃,本质 eval"},
	{"php", `\bcall_user_func(_array)?\s*\(`, "代码执行", "medium", "回调函数可能被用户输入控制"},
	{"php", `\b(system|exec|passthru|shell_exec|popen|proc_open)\s*\(`, "命令执行", "high", "命令执行函数"},
	{"php", "`[^`]*\\$[^`]*`", "命令执行", "high", "反引号执行(内插变量)"},
	{"php", `\bmysql_query\s*\(|\bmysqli(_query|::query)\s*\(|PDO::query`, "SQL 注入", "high", "SQL 查询 sink,检查参数化"},
	{"php", `\b(unserialize)\s*\(`, "反序列化", "high", "反序列化用户输入 = POP 链入口"},
	{"php", `\b(include|include_once|require|require_once)\s*\(\s*\$`, "文件包含", "high", "动态包含变量路径"},
	{"php", `\b(file_get_contents|file_put_contents|fopen|readfile|move_uploaded_file)\s*\(`, "文件操作", "medium", "文件读写,检查路径来源"},
	{"php", `\b(curl_exec|file_get_contents)\s*\(\s*\$`, "SSRF", "high", "服务端发起请求,URL 来自变量"},
	{"php", `\$_(GET|POST|REQUEST|COOKIE)\s*\[`, "用户输入", "low", "超全局输入源(信息标记)"},
	{"php", `md5\s*\(\s*\$|sha1\s*\(\s*\$`, "弱哈希", "low", "弱哈希用于口令场景"},

	// —— Java ——
	{"java", `Runtime\s*\.\s*getRuntime\s*\(\s*\)\s*\.\s*exec`, "命令执行", "high", "Runtime.exec 命令执行"},
	{"java", `new\s+ProcessBuilder\s*\(`, "命令执行", "high", "ProcessBuilder 启动进程"},
	{"java", `ObjectInputStream|readObject\s*\(`, "反序列化", "high", "Java 反序列化 sink"},
	{"java", `XMLDecoder`, "反序列化", "high", "XMLDecoder 反序列化 RCE"},
	{"java", `InitialContext\s*\(|lookup\s*\(`, "JNDI 注入", "high", "JNDI lookup(Log4Shell/Factory 链)"},
	{"java", `(Statement|createStatement)[^;]*execute(Query|Update|Batch)?\s*\(`, "SQL 注入", "high", "拼接 SQL 执行"},
	{"java", `prepareStatement\s*\(\s*"\s*\+`, "SQL 注入", "high", "prepareStatement 字符串拼接"},
	{"java", `(MVEL|SpEL|OGNL|Expression)\w*\s*\.\s*(eval|parse|getValue)`, "表达式注入", "high", "表达式引擎求值用户输入"},
	{"java", `\b(SAXParser|DocumentBuilderFactory|XMLInputFactory)`, "XXE", "high", "XML 解析,检查外部实体开关"},
	{"java", `(URL|HttpClient|RestTemplate)\s*\(?\s*\$?\w*[Uu]rl`, "SSRF", "medium", "服务端请求,目标可被控制"},

	// —— Python ——
	{"python", `\beval\s*\(|\bexec\s*\(|\bcompile\s*\(`, "代码执行", "high", "Python 动态执行"},
	{"python", `\bos\.(system|popen)\s*\(`, "命令执行", "high", "os.system/popen"},
	{"python", `subprocess\.(call|run|Popen|check_output)\s*\(`, "命令执行", "high", "subprocess(shell=True 危险)"},
	{"python", `pickle\.(loads?|Unpickler)|yaml\.load\s*\(`, "反序列化", "high", "pickle/yaml 反序列化 RCE"},
	{"python", `(execute|executemany)\s*\(`, "SQL 注入", "medium", "SQL 执行 sink(检查参数化)"},
	{"python", `render_template_string\s*\(`, "SSTI", "high", "Jinja2 模板字符串渲染"},

	// —— JavaScript / Node ——
	{"javascript", `\beval\s*\(|new\s+Function\s*\(`, "代码执行", "high", "JS 动态执行"},
	{"javascript", `child_process\.(exec|execSync|spawn)\s*\(`, "命令执行", "high", "Node 子进程"},
	{"javascript", `\brequire\s*\(\s*['"]child_process`, "命令执行", "high", "引入子进程模块"},
	{"javascript", `\$\.(ajax|get|post)\s*\(\s*['"]?\s*\$`, "SSRF", "medium", "请求目标来自变量"},
	{"javascript", `jade\.render|ejs\.render|\.render\s*\(`, "SSTI", "medium", "模板渲染"},

	// —— Go ——
	{"go", `exec\.Command(Context)?\s*\(`, "命令执行", "high", "os/exec 命令执行"},
	{"go", `sql\.(Query|Exec)(Context)?\s*\(\s*[^,)]*\+`, "SQL 注入", "high", "SQL 字符串拼接"},

	// —— 通用(密钥/口令泄露,不限语言) ——
	{"通用", `(password|passwd|pwd|secret|token|api[_-]?key)\s*[:=]\s*['"][^'"]{6,}['"]`, "硬编码凭据", "high", "硬编码口令/密钥"},
	{"通用", `-----BEGIN (RSA )?PRIVATE KEY-----`, "私钥泄露", "high", "硬编码私钥"},
	{"通用", `jdbc:(mysql|postgres|sqlserver)://[^\s'"]+`, "数据库连接串", "medium", "连接串可能含凭据"},
	{"通用", `\b(AKIA|ASIA)[0-9A-Z]{16}\b`, "云凭据", "high", "AWS AccessKey"},
	{"通用", `\bghp_[A-Za-z0-9]{36}\b`, "云凭据", "high", "GitHub Token"},
	{"通用", `(md5|sha1)\s*\(`, "弱哈希", "low", "弱哈希(视上下文)"},
}

// seedFuncs 灌入种子(仅当规则表为空)。
func (s *Store) seedFuncs() error {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM audit_sfuncs`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	for i, f := range seedFuncs {
		if _, err := s.db.Exec(`INSERT INTO audit_sfuncs(id,language,pattern,category,risk,note,enabled,sort)
			VALUES(?,?,?,?,?,?,1,?)`, newID(), f.language, f.pattern, f.category, f.risk, f.note, i); err != nil {
			return err
		}
	}
	return nil
}

var _ = sql.ErrNoRows // 保持 database/sql 引用(将来扩展用)
