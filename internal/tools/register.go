// 工具包汇总：main 只需空白导入本包，各工具包的 init() 即完成注册。
package tools

import (
	_ "ctfnet/internal/tools/ai"
	_ "ctfnet/internal/tools/aisec"
	_ "ctfnet/internal/tools/crypto"
	_ "ctfnet/internal/tools/data"
	_ "ctfnet/internal/tools/misc"
	_ "ctfnet/internal/tools/pentest"
	_ "ctfnet/internal/tools/reverse"
	_ "ctfnet/internal/tools/web"
)
