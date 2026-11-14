// aisec.adversarial.image-adv-gen —— 图像对抗样本攻击(MNIST 型 Web 靶场):
// 题目给出 CNN 模型(.keras) 与 Web API(GET /api/load-image/{id} 返回 原始/目标数字、
// 锁定像素、矩阵; POST /api/predict 提交整数矩阵换 flag), 要求在 ≤N 像素预算内
// (避开锁定像素)把预测翻到目标数字。
// 方法: 定向 PGD(梯度上升目标 logit)于自由像素, 成功后按整数差投影到预算内像素子集
// 继续优化; 提交前用本地模型复核量化(uint8/255)后仍命中且置信度 ≥margin 才发送;
// 支持离线模式(本地 dataset/{digit}/*.png 直接攻击, 无需服务器)。
package aisec

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"ctfnet/internal/core"
)

func init() {
	core.DefaultRegistry.Register(&ImageAdvGenTool{BaseTool: core.BaseTool{
		NameAttr:     "image-adv-gen",
		CategoryAttr: CatAdversarial,
		VersionAttr:  "1.0.0",
		DescriptionAttr: "图像对抗样本攻击(MNIST 型 Web 靶场):定向 PGD + 像素预算投影 + 量化复核," +
			"逐图调用 /api/load-image 与 /api/predict 收集 flag;支持离线模式(本地 dataset 目录直接攻击验证);" +
			"流式进度上报, 结果与修改像素明细落盘",
		TimeoutAttr: 7200,
		ParamsAttr: []core.ParamSpec{
			{Name: "mode", Type: "str", Required: false, Default: "auto",
				Description: "auto | mnist_web(MNIST Web 靶场, 需 Python/TensorFlow) | mlp_npz(白盒 MLP npz 定向 PGD, 纯 Go)"},
			{Name: "image", Type: "file", Required: false, Description: "mlp_npz:源图(如 face.png)"},
			{Name: "meta", Type: "file", Required: false, Description: "mlp_npz:meta.json(自动读 target_class/eps/threshold_prob)"},
			{Name: "target", Type: "int", Required: false, Default: -1, Description: "mlp_npz:目标类(缺省读 meta.json)"},
			{Name: "eps", Type: "float", Required: false, Default: -1, Description: "mlp_npz:L∞ 预算(归一化尺度, 缺省 0.15 或 meta)"},
			{Name: "threshold", Type: "float", Required: false, Default: -1, Description: "mlp_npz:目标类置信度阈值(缺省 0.5 或 meta)"},
			{Name: "alpha", Type: "float", Required: false, Default: 0.008, Description: "mlp_npz:PGD 步长"},
			{Name: "steps", Type: "int", Required: false, Default: 600, Description: "mlp_npz:PGD 迭代数"},
			{Name: "model", Type: "file", Required: true, Description: "CNN 模型(.keras/.h5)或 MLP 白盒权重(model.npz, 视 mode)"},
			{Name: "base_url", Type: "str", Required: false, Default: "", Description: "靶场地址(如 http://host:port/);留空且提供 offline_dir 时为离线模式"},
			{Name: "id_start", Type: "int", Required: false, Default: 1, Description: "攻击起始图像 ID"},
			{Name: "id_end", Type: "int", Required: false, Default: 90, Description: "攻击结束图像 ID(含)"},
			{Name: "max_pixels", Type: "int", Required: false, Default: 100, Description: "题目限制: 最多修改像素数"},
			{Name: "margin", Type: "float", Required: false, Default: 0.6, Description: "提交前量化复核要求的目标类最低置信度"},
			{Name: "max_iters", Type: "int", Required: false, Default: 300, Description: "每图 PGD 最大迭代数"},
			{Name: "sleep_ms", Type: "int", Required: false, Default: 200, Description: "请求间隔毫秒(礼貌性限速)"},
			{Name: "offline_dir", Type: "str", Required: false, Default: "", Description: "离线模式: 本地 dataset 目录(含 0-9 数字子目录, 内为 png), 目标=(原数字+1)%10"},
			{Name: "out", Type: "str", Required: false, Default: "image_flags.txt", Description: "flag 结果输出文件"},
			{Name: "python", Type: "str", Required: false, Default: "", Description: "Python 解释器(默认自动探测; 需本机装有 tensorflow)"},
		},
		Runner: imageAdvGenRun,
	}})
}

// ImageAdvGenTool 图像对抗样本攻击工具。
type ImageAdvGenTool struct {
	core.BaseTool
}

// imageAdvPy 攻击脚本: 流式 PROGRESS 行 + 末行 JSON 汇总。
const imageAdvPy = `
import json, os, sys, time
import numpy as np

MODEL, OUT = sys.argv[1], sys.argv[2]
BASE = sys.argv[3]
ID_START, ID_END = int(sys.argv[4]), int(sys.argv[5])
MAX_PIX, MARGIN, MAX_ITERS, SLEEP = int(sys.argv[6]), float(sys.argv[7]), int(sys.argv[8]), float(sys.argv[9]) / 1000.0
OFFLINE = sys.argv[10]

import tensorflow as tf

def emit_progress(done, total, msg):
    print('PROGRESS %d %d %s' % (done, total, msg), flush=True)

model = tf.keras.models.load_model(MODEL)

def predict_int(mat):  # mat: 28x28 uint8 -> argmax, conf
    x = mat.astype(np.float32) / 255.0
    p = model(np.expand_dims(np.expand_dims(x, 0), -1), training=False).numpy()[0]
    c = int(np.argmax(p))
    return c, float(p[c])

def pgd_attack(orig_u8, target, locked, budget, margin, max_iters):
    '''定向 PGD: 返回 (成功, uint8 矩阵, 修改像素数)。量化复核 + 预算投影。'''
    orig = orig_u8.astype(np.float32) / 255.0
    free = np.ones((28, 28), dtype=bool)
    for (r, c) in locked:
        free[r, c] = False
    best = None  # (npix, mat)
    for eps0 in (0.08, 0.2, 0.4):
        x = orig.copy()
        act = free.copy()  # 本次允许改的像素(预算投影后可能缩小)
        eps = eps0
        for it in range(max_iters):
            xv = tf.Variable(np.expand_dims(np.expand_dims(x, 0), -1), dtype=tf.float32)
            with tf.GradientTape() as tape:
                tape.watch(xv)
                logits = model(xv, training=False)
                loss = -logits[0][target]
            g = tape.gradient(loss, xv).numpy()[0, :, :, 0]
            g *= act  # 只在允许的像素上走
            x = np.clip(x - eps * np.sign(g), 0, 1)  # 梯度下降 min(-logit_target) = 最大化目标类
            x[~act] = orig[~act]
            eps = min(eps + 0.04, 1.0)
            mat_u8 = np.round(x * 255).astype(np.uint8)
            if it % 5 == 4 or it == max_iters - 1:
                changed = int((mat_u8 != orig_u8).sum())
                if changed > budget:
                    # 预算投影: 严格保留变化最大的前 budget 个像素(打破并列), 其余还原并锁定
                    diff = np.abs(mat_u8.astype(np.int16) - orig_u8.astype(np.int16))
                    keep_n = min(budget, int((diff > 0).sum()))
                    if keep_n == 0:
                        continue
                    order = np.argsort(diff, axis=None)[::-1][:keep_n]
                    keep = np.zeros(diff.shape, dtype=bool)
                    keep.flat[order] = True
                    x = np.where(keep, x, orig)
                    act = act & keep
                    mat_u8 = np.round(x * 255).astype(np.uint8)
                    changed = int((mat_u8 != orig_u8).sum())
                cls, conf = predict_int(mat_u8)
                if cls == target and conf >= margin and changed <= budget:
                    if best is None or changed < best[0]:
                        best = (changed, mat_u8)
                    break  # 本 eps 档已成功且是最少像素, 换更大 eps 也未必更少 → 直接结束
        if best is not None:
            return True, best[1], best[0]
    return False, orig_u8, 0

results = []
flags = []
failed = []
skipped = 0

def load_offline(mid):
    for d in sorted(os.listdir(OFFLINE)):
        dp = os.path.join(OFFLINE, d)
        if not d.isdigit() or not os.path.isdir(dp):
            continue
        for fn in sorted(os.listdir(dp)):
            if fn.lower().endswith(('.png', '.jpg', '.jpeg')):
                yield d, os.path.join(dp, fn)
    return

offline_items = None
if OFFLINE:
    offline_items = list(load_offline(0))
    total = len(offline_items)
    for idx, (dgt, path) in enumerate(offline_items):
        from PIL import Image
        img = Image.open(path)
        if img.mode == 'RGBA':
            bg = Image.new('RGB', img.size, (255, 255, 255))
            bg.paste(img, mask=img.split()[3])
            img = bg.convert('L')
        else:
            img = img.convert('L')
        img = img.resize((28, 28), Image.LANCZOS)
        mat = np.array(img).astype(np.uint8)
        orig_cls, _ = predict_int(mat)
        target = (int(dgt) + 1) % 10
        emit_progress(idx, total, '%s 原始=%d 目标=%d' % (os.path.basename(path), orig_cls, target))
        if orig_cls == target:
            skipped += 1
            continue
        ok, adv, npix = pgd_attack(mat, target, set(), MAX_PIX, MARGIN, MAX_ITERS)
        cls, conf = predict_int(adv)
        if ok and cls == target:
            flags.append({'id': idx + 1, 'file': path, 'original': int(orig_cls),
                          'target': target, 'predicted': cls, 'pixels': npix,
                          'flag': 'OFFLINE-OK', 'conf': round(conf, 4)})
        else:
            failed.append(idx + 1)
    emit_progress(total, total, '离线完成')
else:
    import requests
    total = ID_END - ID_START + 1
    done = 0
    for mid in range(ID_START, ID_END + 1):
        try:
            info = requests.get(BASE.rstrip('/') + '/api/load-image/%d' % mid, timeout=15).json()
            if not info.get('success'):
                done += 1
                emit_progress(done, total, 'ID=%d 获取失败' % mid)
                failed.append(mid)
                continue
            orig_cls = int(info['digit'])
            target = int(info['target_digit'])
            locked = set()
            for ps in info.get('locked_pixels', []):
                r, c = ps.split('-')
                locked.add((int(r), int(c)))
            mat = np.array(info['matrix'], dtype=np.uint8)
            emit_progress(done, total, 'ID=%d 原始=%d 目标=%d 锁定=%d' % (mid, orig_cls, target, len(locked)))
            if orig_cls == target:
                skipped += 1
                done += 1
                continue
            ok, adv, npix = pgd_attack(mat, target, locked, MAX_PIX, MARGIN, MAX_ITERS)
            if not ok:
                failed.append(mid)
                done += 1
                emit_progress(done, total, 'ID=%d 攻击失败' % mid)
                continue
            resp = requests.post(BASE.rstrip('/') + '/api/predict',
                                 json={'image_id': mid, 'matrix': adv.astype(int).tolist()}, timeout=15).json()
            pred = resp.get('predicted_digit')
            ok2 = resp.get('adversarial_success') and pred == target
            fl = resp.get('flag', '')
            item = {'id': mid, 'file': info.get('filename', ''), 'original': orig_cls,
                    'target': target, 'predicted': pred,
                    'pixels': resp.get('modified_pixels', npix), 'flag': fl if ok2 else ''}
            results.append(item)
            if ok2:
                flags.append(item)
            else:
                failed.append(mid)
            done += 1
            emit_progress(done, total, 'ID=%d %s 预测=%s 像素=%s' % (mid, '✓' if ok2 else '✗', pred, item['pixels']))
            if SLEEP > 0:
                time.sleep(SLEEP)
        except Exception as e:
            failed.append(mid)
            done += 1
            emit_progress(done, total, 'ID=%d 错误 %s' % (mid, str(e)[:80]))
    emit_progress(total, total, '完成')

with open(OUT, 'w', encoding='utf-8') as f:
    f.write('图像对抗样本攻击结果  成功 %d / 失败 %d / 跳过 %d\n\n' % (len(flags), len(failed), skipped))
    for it in flags:
        f.write('ID=%(id)s  %(file)s  %(original)s->%(target)s  预测=%(predicted)s  像素=%(pixels)s\n  Flag: %(flag)s\n' % it)

print(json.dumps({'total': total, 'ok': len(flags), 'failed': failed, 'skipped': skipped,
                  'flags': flags, 'out': OUT}, ensure_ascii=False))
`

// imageAdvSummary 脚本汇总。
type imageAdvSummary struct {
	Total   int   `json:"total"`
	OK      int   `json:"ok"`
	Skipped int   `json:"skipped"`
	Failed  []int `json:"failed"`
	Flags   []struct {
		ID        int    `json:"id"`
		File      string `json:"file"`
		Original  int    `json:"original"`
		Target    int    `json:"target"`
		Predicted any    `json:"predicted"`
		Pixels    any    `json:"pixels"`
		Flag      string `json:"flag"`
	} `json:"flags"`
	Out string `json:"out"`
}

var progressLineRe = regexp.MustCompile(`^PROGRESS (\d+) (\d+) (.*)$`)

func imageAdvGenRun(ctx context.Context, params map[string]any, tc *core.ToolContext) (*core.ToolResult, error) {
	// 模式分流:auto = npz 模型 + 给了源图 → 纯 Go 白盒 MLP;否则走 MNIST Web 靶场(Python)
	mode := strP(params, "mode", "auto")
	modelFiles := fileParam(params, "model")
	if mode == "mlp_npz" || (mode == "auto" && len(modelFiles) > 0 &&
		strings.HasSuffix(strings.ToLower(modelFiles[0]), ".npz") && len(fileParam(params, "image")) > 0) {
		return mlpAdvRun(ctx, params, tc)
	}
	if mode == "mlp_npz" {
		return errAisec("mlp_npz 模式需要 image(源图)与 model(model.npz)参数"), nil
	}
	if len(modelFiles) == 0 {
		return &core.ToolResult{Status: core.StatusError, Stderr: "缺少必需参数: model"}, nil
	}
	pyExe := findPythonExe(strP(params, "python", ""))
	if pyExe == "" {
		return &core.ToolResult{Status: core.StatusError, Stderr: "未找到 Python 解释器(需本机装有 tensorflow)"}, nil
	}
	workdir := ""
	if tc != nil {
		workdir = tc.Workdir
	}
	resolveP := func(p string) string {
		if p != "" && workdir != "" && !filepath.IsAbs(p) {
			return filepath.Join(workdir, p)
		}
		return p
	}
	baseURL := strings.TrimSuffix(strP(params, "base_url", ""), "/")
	offlineDir := resolveP(strP(params, "offline_dir", ""))
	modelPath := resolveP(modelFiles[0])
	if baseURL == "" && offlineDir == "" {
		return &core.ToolResult{Status: core.StatusError, Stderr: "缺少必需参数: base_url(靶场地址)或 offline_dir(离线模式)"}, nil
	}
	// 前置校验: 模型与离线目录必须真实存在(zip 内路径/未解压是高频错误)
	if st, err := os.Stat(modelPath); err != nil || st.IsDir() {
		return &core.ToolResult{Status: core.StatusError, Stderr: fmt.Sprintf("模型文件不存在或不可读: %s", modelPath)}, nil
	}
	if offlineDir != "" {
		if strings.Contains(offlineDir, ".zip") {
			return &core.ToolResult{Status: core.StatusError, Stderr: "offline_dir 指向 zip 压缩包内的路径, 请先解压到真实目录再填写"}, nil
		}
		st, err := os.Stat(offlineDir)
		if err != nil || !st.IsDir() {
			return &core.ToolResult{Status: core.StatusError, Stderr: "offline_dir 不是有效目录: " + offlineDir}, nil
		}
		hasDigit := false
		for d := 0; d < 10; d++ {
			if fi, err := os.Stat(filepath.Join(offlineDir, string(rune('0'+d)))); err == nil && fi.IsDir() {
				hasDigit = true
				break
			}
		}
		if !hasDigit {
			return &core.ToolResult{Status: core.StatusError, Stderr: "offline_dir 下没有 0-9 数字子目录(期望 dataset/<数字>/xxx.png 结构)"}, nil
		}
	}
	idStart := intP(params, "id_start", 1)
	idEnd := intP(params, "id_end", 90)
	maxPix := clamp(intP(params, "max_pixels", 100), 1, 784)
	margin := floatP(params, "margin", 0.6)
	maxIters := clamp(intP(params, "max_iters", 300), 10, 5000)
	sleepMs := clamp(intP(params, "sleep_ms", 200), 0, 60000)
	outPath := resolveP(strP(params, "out", "image_flags.txt"))

	reportProgress(tc, 5, "加载模型(python/tensorflow)")
	lastLine, err := runPythonStream(ctx, pyExe, imageAdvPy, []string{
		modelPath, outPath, baseURL,
		fmt.Sprintf("%d", idStart), fmt.Sprintf("%d", idEnd),
		fmt.Sprintf("%d", maxPix), fmt.Sprintf("%v", margin),
		fmt.Sprintf("%d", maxIters), fmt.Sprintf("%d", sleepMs), offlineDir,
	}, 0, func(line string) {
		if m := progressLineRe.FindStringSubmatch(line); m != nil {
			done, _ := strconv.Atoi(m[1])
			total, _ := strconv.Atoi(m[2])
			pct := 5
			if total > 0 {
				pct = 5 + done*95/total
			}
			reportProgress(tc, pct, m[3])
		}
	})
	if err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: "攻击脚本失败: " + err.Error()}, nil
	}
	var s imageAdvSummary
	if err := json.Unmarshal([]byte(lastLine), &s); err != nil {
		return &core.ToolResult{Status: core.StatusError, Stderr: "结果解析失败: " + err.Error()}, nil
	}

	var out strings.Builder
	modeLabel := "Web 靶场: " + baseURL
	if offlineDir != "" {
		modeLabel = "离线模式: " + offlineDir
	}
	fmt.Fprintf(&out, "[图像对抗样本攻击] %s\n", modeLabel)
	
	fmt.Fprintf(&out, "模型: %s | 预算: ≤%d 像素 | 量化复核置信度 ≥%.2f\n\n", modelFiles[0], maxPix, margin)
	fmt.Fprintf(&out, "成功: %d/%d | 跳过(已匹配): %d | 失败: %d %v\n\n", s.OK, s.Total, s.Skipped, len(s.Failed), s.Failed)
	if len(s.Flags) > 0 {
		fmt.Fprintf(&out, "== Flags ==\n")
		for _, f := range s.Flags {
			fmt.Fprintf(&out, "  ID=%-3d %v→%v 预测=%v 像素=%v | %s\n", f.ID, f.Original, f.Target, f.Predicted, f.Pixels, f.Flag)
		}
	}
	fmt.Fprintf(&out, "\n✓ 结果文件: %s\n", outPath)

	return &core.ToolResult{
		Status:    core.StatusSuccess,
		Stdout:    out.String(),
		Artifacts: []core.Artifact{fileArtifact(outPath)},
		Data: map[string]any{
			"total": s.Total, "ok": s.OK, "skipped": s.Skipped,
			"failed": s.Failed, "flags": s.Flags, "out": outPath,
		},
	}, nil
}
