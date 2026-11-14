#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
CTF 文本分类「模型后门 / 数据投毒」通用解题器 (纯标准库, 零依赖)

适用场景: 平台用词频统计型分类器 (MultinomialNB / TF-IDF+线性模型) 做
垃圾文本二分类, 允许注入 ≤N 条 (每条 ≤M 字符) 带标签样本并调超参。

核心原理:
  方向性权重 dir(w) = log P(w|spam) - log P(w|ham)   (即 log-odds)
  |dir| 极大的词 = "几乎只在一类出现" 的强方向词。
  判别式为 sum(dir(词)) > 0 ? spam : ham, 因此:
  投毒样本 = 触发器T + 反方向强词w -> 打目标标签
  => 既把 dir(T) 推向极端, 又中和 w 的反向拉力, 且不污染正常类强词。

用法:
  # 验证一: 双触发器标签翻转
  python poison_backdoor.py train.jsonl --trigger 5f30GROx=ham --trigger w0qMI2hB=spam \
      --budget 50 --maxlen 20 --out poison.jsonl --simulate
  # 验证二: 验证码短信全判垃圾 (把主题词本身当触发器)
  python poison_backdoor.py train.jsonl --trigger 验证码=spam --budget 50 --maxlen 20 --simulate
  # 若题目直接给了 sklearn 模型 pickle, 可先用它导出方向性权重:
  python poison_backdoor.py --model model.pkl --top 30
"""
import argparse
import csv
import json
import math
import pickle
import re
import sys
from collections import Counter

if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8")
    sys.stderr.reconfigure(encoding="utf-8")

SPAM_MARKS = {"1", "spam", "垃圾", "neg", "negative", "bad", "yes", "true"}


def die(msg):
    print(f"[!] {msg}", file=sys.stderr)
    sys.exit(1)


# ---------------------------------------------------------------- 数据加载
_HEADER_RE = re.compile(r"(序号|编号|文本|内容|标签|答案|类别|^id$|^no$|index|text|content|label|class|y)", re.I)


def _pick_cols(sample):
    """从若干行样本中挑 (文本列, 标签列): 文本列=平均字符串最长, 标签列=去重值最少"""
    ncol = max(len(r) for r in sample)
    avg_len = [0.0] * ncol
    for r in sample:
        for i in range(ncol):
            avg_len[i] += len(str(r[i])) if i < len(r) and r[i] is not None else 0
    avg_len = [v / len(sample) for v in avg_len]
    text_col = max(range(ncol), key=lambda i: avg_len[i])
    label_col = min((i for i in range(ncol) if i != text_col),
                    key=lambda i: len({str(r[i]) for r in sample if i < len(r)}), default=None)
    return text_col, label_col


def _xlsx_rows(path):
    """读 xlsx 首个 sheet: openpyxl 优先, 否则纯标准库 (zip+XML) 回退"""
    try:
        import openpyxl
        ws = openpyxl.load_workbook(path, read_only=True).worksheets[0]
        return [list(r) for r in ws.iter_rows(values_only=True)]
    except ImportError:
        import zipfile
        import xml.etree.ElementTree as ET
        ns = "{http://schemas.openxmlformats.org/spreadsheetml/2006/main}"
        with zipfile.ZipFile(path) as z:
            shared = []
            if "xl/sharedStrings.xml" in z.namelist():
                root = ET.fromstring(z.read("xl/sharedStrings.xml"))
                for si in root.findall(f"{ns}si"):
                    shared.append("".join(t.text or "" for t in si.iter(f"{ns}t")))
            sheet = ET.fromstring(z.read("xl/worksheets/sheet1.xml"))
            out = []
            for row in sheet.iter(f"{ns}row"):
                cells = []
                for c in row.iter(f"{ns}c"):
                    v = c.find(f"{ns}v")
                    if c.get("t") == "s" and v is not None:
                        cells.append(shared[int(v.text)])
                    elif c.get("t") == "inlineStr":
                        cells.append("".join(t.text or "" for t in c.iter(f"{ns}t")))
                    else:
                        cells.append(v.text if v is not None else None)
                out.append(cells)
            return out


def load_rows(path):
    """支持 jsonl/csv/tsv/xlsx, 自动嗅探表头/文本列/标签列, 返回 [(text, raw_label)]"""
    raw = []
    rows = []
    if path.lower().endswith((".xlsx", ".xlsm")):
        raw = _xlsx_rows(path)
        if not raw:
            die(f"{path} 无数据行")
        head = [str(c) for c in raw[0] if c is not None]
        if any(_HEADER_RE.search(h) for h in head):
            raw = raw[1:]
        text_col, label_col = _pick_cols(raw[:30])
        if label_col is None:
            die("xlsx 中找不到标签列")
        rows = [(str(r[text_col]), str(r[label_col])) for r in raw
                if len(r) > max(text_col, label_col) and r[text_col] is not None and r[label_col] is not None]
    elif path.endswith(".jsonl"):
        with open(path, encoding="utf-8") as f:
            for ln, line in enumerate(f, 1):
                line = line.strip()
                if not line:
                    continue
                try:
                    obj = json.loads(line)
                except json.JSONDecodeError:
                    die(f"第 {ln} 行不是合法 JSON: {line[:50]}")
                txt = obj.get("text") or obj.get("content") or obj.get("data") or ""
                lab = obj.get("label", obj.get("y"))
                if txt == "" or lab is None:
                    die(f"第 {ln} 行缺少 text/label 字段")
                rows.append((str(txt), lab))
    else:
        delim = "\t" if path.endswith((".tsv", ".tab")) else ","
        with open(path, encoding="utf-8-sig", newline="") as f:
            for r in csv.reader(f, delimiter=delim):
                if len(r) < 2:
                    continue
                rows.append((r[0], r[1]))
    if not rows:
        die(f"未能从 {path} 读到任何样本")
    return rows


def normalize_labels(rows, spam_label=None, ham_label=None):
    """把原始标签映射为 'spam'/'ham'。显式指定优先, 否则自动嗅探。"""
    vals = sorted({str(l) for _, l in rows}, key=str)
    if len(vals) != 2:
        die(f"标签不是二分类, 取值: {vals}")
    s = spam_label if spam_label is not None else next((v for v in vals if v.lower() in SPAM_MARKS), None)
    if s is None:
        s = vals[-1]  # 兜底: 排序靠后者当 spam, 打印警告供人工确认
        print(f"[?] 未识别垃圾标签, 默认 '{s}' 为垃圾 (可用 --spam-label 覆盖)")
    h = ham_label if ham_label is not None else next(v for v in vals if v != s)
    return [(t, "spam" if str(l) == s else "ham") for t, l in rows], s, h


# ---------------------------------------------------------------- 特征提取
_TOKEN = re.compile(r"\w+", re.UNICODE)


def features(text):
    r"""word 级 + char(1,2)-gram 混合特征 (中文按 \w+ 整段切出后内部再切 ngram)"""
    feats = []
    for tok in _TOKEN.findall(text.lower()):
        feats.append(tok)
        if len(tok) > 1:
            feats.extend(tok[i:i + n] for n in (1, 2) for i in range(len(tok) - n + 1))
    return feats


# ------------------------------------------------------- 方向性权重 (log-odds)
def directional_weights(rows, alpha=1.0, min_count=2):
    """返回 {word: dir}, dir>0 拉向垃圾, dir<0 拉向正常。min_count 过滤低频噪声。"""
    cs, ch = Counter(), Counter()
    for t, lab in rows:
        (cs if lab == "spam" else ch).update(features(t))
    vocab = set(cs) | set(ch)
    V = len(vocab)
    Ns, Nh = sum(cs.values()), sum(ch.values())
    ds, dh = math.log(Ns + alpha * V), math.log(Nh + alpha * V)
    out = {}
    for w in vocab:
        if cs[w] + ch[w] < min_count:
            continue
        out[w] = (math.log(cs[w] + alpha) - ds) - (math.log(ch[w] + alpha) - dh)
    return out


# ---------------------------------------------------------------- 手写 MNB
class NB:
    def __init__(self, alpha=1.0):
        self.alpha = alpha

    def fit(self, rows):
        cs, ch = Counter(), Counter()
        n_s = n_h = 0
        for t, lab in rows:
            c = features(t)
            (cs if lab == "spam" else ch).update(c)
            if lab == "spam":
                n_s += 1
            else:
                n_h += 1
        V = len(set(cs) | set(ch))
        ds, dh = math.log(sum(cs.values()) + self.alpha * V), math.log(sum(ch.values()) + self.alpha * V)
        self.lp_s = {w: math.log(c + self.alpha) - ds for w, c in cs.items()}
        self.lp_h = {w: math.log(c + self.alpha) - dh for w, c in ch.items()}
        self.V = V
        self.def_s, self.def_h = -ds, -dh  # 未见词的平滑 log 概率
        self.prior = math.log(n_s / (n_s + n_h)) - math.log(n_h / (n_s + n_h))
        return self

    def score(self, text):
        """>0 判垃圾。= sum count(w)*(lp_s-lp_h) + prior"""
        sc = self.prior
        ls, lh = self.lp_s, self.lp_h
        for w, c in Counter(features(text)).items():
            sc += c * (ls.get(w, self.def_s) - lh.get(w, self.def_h))
        return sc

    def predict(self, text):
        return "spam" if self.score(text) > 0 else "ham"


# ---------------------------------------------------------------- 投毒生成
def fit_budget(trigger, word, maxlen):
    """一条 ≤maxlen 的搭配样本: 尽量 'T T w', 放不下退化为 'T w' / 'T'"""
    parts = [trigger]
    room = maxlen - len(trigger)
    if word and 1 + len(word) <= room:
        parts.append(word)
        room -= 1 + len(word)
    while 1 + len(trigger) <= room:  # 剩余空间继续塞触发器
        parts.append(trigger)
        room -= 1 + len(trigger)
    return " ".join(parts)


def gen_poison(trigger, target, n, maxlen, opp_words):
    """n 条: 一半纯复读(权重最稳), 一半 T+反向强词搭配(中和对抗词)"""
    out = []
    half = max(1, n // 2)
    for i in range(n):
        w = opp_words[(i - half) % len(opp_words)] if i >= half and opp_words else ""
        out.append(fit_budget(trigger, w, maxlen))
    return out


def pick_opp_words(dirs, target, maxlen, trigger, top=25):
    """与 target 方向相反、|dir| 最强、且一条内放得下的词 (避开触发器自身子串)"""
    sign = 1 if target == "spam" else -1
    cands = [(w, d) for w, d in dirs.items() if d * sign < -1.0 and 1 + len(w) <= maxlen - len(trigger)]
    cands.sort(key=lambda x: -abs(x[1]))
    return [w for w, _ in cands[:top] if trigger.lower() not in w.lower()]


# ---------------------------------------------------------------- 模拟评估
def simulate(rows, poisons, triggers, alpha):
    base = NB(alpha).fit(rows)
    pbed = NB(alpha).fit(rows + [(p, lab) for p, lab, _ in poisons])

    def acc(model, subset):
        return sum(model.predict(t) == lab for t, lab in subset) / len(subset) if subset else float("nan")

    print("\n===== 本地模拟 (手写 MultinomialNB, alpha=%g) =====" % alpha)
    spam_rows = [r for r in rows if r[1] == "spam"]
    ham_rows = [r for r in rows if r[1] == "ham"]
    print(f"{'指标':<26}{'加毒前':>10}{'加毒后':>10}")
    print("-" * 46)
    print(f"{'垃圾样本判对率':<24}{acc(base, spam_rows):>10.1%}{acc(pbed, spam_rows):>10.1%}")
    print(f"{'正常样本判对率(须保持)':<20}{acc(base, ham_rows):>10.1%}{acc(pbed, ham_rows):>10.1%}")
    for trig, tgt in triggers:
        # 评估口径: 触发器是自然词(如"验证码")时, 目标人群=原本就含它的样本;
        # 是外部注入串(如 5f30GROx)时, 对全部样本贴触发器测翻转
        containing = [r for r in rows if trig in r[0]]
        pool = containing if containing else [(trig + " " + t, lab) for t, lab in rows]
        tag = f"(含'{trig}'的{len(containing)}条)" if containing else "(全部样本贴触发器)"
        ok_b = sum(base.predict(t) == tgt for t, _ in pool)
        ok_p = sum(pbed.predict(t) == tgt for t, _ in pool)
        print(f"触发器 {trig!r}->{tgt:<5} 命中率  {ok_b / len(pool):>8.1%}{ok_p / len(pool):>10.1%}  {tag}")
        if re.fullmatch(r"[一-鿿]{2,}", trig):  # 自然词触发器: 模拟典型验证码短信探针
            for probe in (f"尊敬的客户,您的{trig}是318490,请在5分钟内输入",
                          f"【某某科技】您的{trig}为88s2mK,5分钟内有效",
                          f"您正在进行登录操作,{trig}9f3k2"):
                pred = pbed.predict(probe)
                print(f"    探针 {probe!r} -> {pred} {'OK' if pred == tgt else '!!未翻'}")


# ---------------------------------------------------------------- sklearn 模型导出
def dump_sklearn_weights(path, top):
    try:
        with open(path, "rb") as f:
            model = pickle.load(f)
    except Exception as e:
        die(f"读模型失败: {e}")
    flp = getattr(model, "feature_log_prob_", None)
    names = getattr(model, "get_feature_names_out", lambda: None)()
    if flp is None:  # Pipeline 场景
        for step in getattr(model, "steps", []):
            flp = getattr(step[1], "feature_log_prob_", None)
            if flp is not None:
                names = getattr(step[1], "get_feature_names_out", lambda: None)()
                break
    if flp is None:
        die("未找到 feature_log_prob_ (不是朴素贝叶斯类模型?)")
    if names is None:
        names = getattr(model, "feature_names_in_", [f"f{i}" for i in range(flp.shape[1])])
    dirs = {str(n): flp[1][i] - flp[0][i] for i, n in enumerate(names)}  # 1=spam 行, 若相反自行对调
    print_weights(dirs, top)


def print_weights(dirs, top):
    ranked = sorted(dirs.items(), key=lambda x: -x[1])
    print(f"\n===== 方向性权重 top{top} (dir>0 拉向垃圾 / dir<0 拉向正常) =====")
    print(f"{'词':<20}{'dir':>9}   {'词':<20}{'dir':>9}")
    for (ws, ds), (wh, dh) in zip(ranked[:top], ranked[::-1][:top]):
        print(f"{ws:<20}{ds:>9.3f}   {wh:<20}{dh:>9.3f}")


# ---------------------------------------------------------------- 主流程
def main():
    ap = argparse.ArgumentParser(description="CTF 文本分类后门/投毒生成器")
    ap.add_argument("data", nargs="?", help="训练数据 jsonl/csv/tsv (text,label)")
    ap.add_argument("--model", help="sklearn 模型 pickle, 直接导出方向性权重")
    ap.add_argument("--trigger", action="append", default=[], metavar="T=spam|ham",
                    help="触发器及目标方向, 可多次; 验证二类题目把主题词本身作为触发器")
    ap.add_argument("--budget", type=int, default=50, help="投毒总条数 (默认 50)")
    ap.add_argument("--maxlen", type=int, default=20, help="每条字符上限 (默认 20)")
    ap.add_argument("--out", default="poison.jsonl", help="输出文件")
    ap.add_argument("--alpha", type=float, default=1.0, help="NB 平滑超参 (模拟用)")
    ap.add_argument("--min-count", type=int, default=2, help="方向性统计的最小词频")
    ap.add_argument("--top", type=int, default=15, help="方向性词打印数量")
    ap.add_argument("--spam-label", help="显式指定垃圾标签值")
    ap.add_argument("--pair", action="append", default=[], metavar="T=词1,词2",
                    help="为触发器 T 手动指定搭配词(逗号分隔), 中和目标语境的弱方向词, 如: 验证码=的,是,您,请")
    ap.add_argument("--ham-label", help="显式指定正常标签值")
    ap.add_argument("--no-simulate", action="store_true", help="跳过本地模拟")
    args = ap.parse_args()

    if args.model:
        dump_sklearn_weights(args.model, args.top)
        return
    if not args.data:
        die("需要训练数据文件或 --model")
    if not args.trigger:
        die("至少一个 --trigger T=spam|ham")

    rows, s_lab, h_lab = normalize_labels(load_rows(args.data), args.spam_label, args.ham_label)
    print(f"[+] 样本 {len(rows)} 条; 垃圾标签='{s_lab}' 正常标签='{h_lab}'")

    dirs = directional_weights(rows, alpha=args.alpha, min_count=args.min_count)
    print_weights(dirs, args.top)

    triggers = []
    for spec in args.trigger:
        if "=" not in spec:
            die(f"--trigger 格式应为 T=spam|ham: {spec}")
        t, tgt = spec.rsplit("=", 1)
        if tgt not in ("spam", "ham"):
            die(f"目标方向须为 spam|ham: {spec}")
        triggers.append((t, tgt))

    pair_map = {}
    for p in args.pair:
        if "=" not in p:
            die(f"--pair 格式应为 T=词1,词2: {p}")
        t, ws = p.rsplit("=", 1)
        pair_map[t] = [w for w in ws.replace("，", ",").split(",") if w]

    poisons = []
    share = args.budget // len(triggers)
    for t, tgt in triggers:
        opp = pair_map.get(t) or pick_opp_words(dirs, tgt, args.maxlen, t)
        n = share if len(triggers) > 1 else args.budget
        samples = gen_poison(t, tgt, n, args.maxlen, opp)
        poisons.extend((s, tgt, t) for s in samples)
        print(f"\n[+] 触发器 {t!r} -> {tgt}: {len(samples)} 条, 搭配反向词示例: {opp[:6]}")

    with open(args.out, "w", encoding="utf-8") as f:
        for s, lab, _ in poisons:
            f.write(json.dumps({"text": s, "label": lab}, ensure_ascii=False) + "\n")
    assert all(len(s) <= args.maxlen for s, _, _ in poisons), "存在超长样本!"
    print(f"\n[+] 已写出 {len(poisons)} 条 -> {args.out} (全部 ≤{args.maxlen} 字符)")
    print("    示例:", *[f"{s!r}->{l}" for s, l, _ in poisons[:6]], sep="\n      ")

    if not args.no_simulate:
        simulate(rows, poisons, triggers, args.alpha)


if __name__ == "__main__":
    main()
