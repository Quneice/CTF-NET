import { useEffect, useMemo, useRef, useState } from 'react';
import type { InputHTMLAttributes } from 'react';
import { ChevronRight, Plus, X } from 'lucide-react';

import type { FieldOption, FormValues, InputField } from '@/types';

import { evaluateCondition } from '@/lib/form';
import { listDirWordlists } from '@/lib/backend';
import { parseRawHttpRequest, type RawHttpParseResult } from '@/lib/rawHttp';
import { sniffTableFile, type FileTablePreview } from '@/lib/tablePreview';

/**
 * 动态表单递归渲染器。
 *
 * 通过 `inputSchema: InputField[]` 递归渲染出完整表单，支持：
 * - 所有 InputField 类型（text/password/number/textarea/select/radio/checkbox/
 *   checkboxGroup/switch/range/file/keyvalue/code/note）
 * - 任意层级嵌套 group（二级、三级分组）
 * - visibleWhen 基于当前值实时显隐、disabledWhen 条件禁用
 * - 必填校验错误展示
 */

/** 值字段（排除 group 容器与 note 提示框） */
type ValueField = Exclude<InputField, { type: 'group' } | { type: 'note' }>;

/** 统一输入框样式 */
const inputCls =
  'w-full rounded-md border border-slate-300 bg-white px-3 py-2 text-sm text-slate-900 outline-none ' +
  'focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 ' +
  'disabled:cursor-not-allowed disabled:bg-slate-100 disabled:text-slate-400 ' +
  'dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100 dark:disabled:bg-slate-800';

/** 递归渲染所需的公共 props（不含 fields；dynOptions 为内部透传） */
interface RenderProps {
  values: FormValues;
  errors: Record<string, string>;
  visibility: Map<string, boolean>;
  onChange: (name: string, value: unknown) => void;
  /** 动态选项（optionsSource 字段解析结果：字段名 → 选项） */
  dynOptions: Record<string, FieldOption[]>;
  /** 表格文件列预览（file 字段名 → 嗅探结果；未选文件或未开启预览为 null） */
  filePreviews: Record<string, FileTablePreview | null>;
}

/** DynamicForm 对外 props：dynOptions 由组件内部解析，调用方无需传 */
interface DynamicFormProps {
  fields: InputField[];
  values: FormValues;
  errors: Record<string, string>;
  visibility: Map<string, boolean>;
  onChange: (name: string, value: unknown) => void;
  /** 顶层字段/分组列数：2 = 两列并排（宽屏），压缩表单高度 */
  columns?: number;
}

/** 收集字段树中所有 optionsSource（字段名 → 来源标识） */
function collectOptionsSources(fields: InputField[], acc: Map<string, string>): Map<string, string> {
  for (const f of fields) {
    if (f.type === 'group') {
      collectOptionsSources(f.children, acc);
    } else if ((f.type === 'select' || f.type === 'radio') && f.optionsSource) {
      acc.set(f.name, f.optionsSource);
    }
  }
  return acc;
}

/** 解析动态选项来源；浏览器 mock 模式由 listDirWordlists 内部回退为空列表 */
async function resolveOptionsSource(src: string): Promise<FieldOption[]> {
  if (src === 'dirWordlists') {
    const list = await listDirWordlists();
    return list.map((w) => ({
      label: `${w.name}（${w.lines.toLocaleString()} 词 / ${(w.size / 1024).toFixed(0)} KB）`,
      value: w.name,
    }));
  }
  return [];
}

/** 异步解析字段树中的 optionsSource，返回 字段名 → 选项 映射 */
function useDynamicOptions(fields: InputField[]): Record<string, FieldOption[]> {
  const sources = useMemo(() => collectOptionsSources(fields, new Map()), [fields]);
  const [options, setOptions] = useState<Record<string, FieldOption[]>>({});
  const key = [...sources.entries()].map(([n, s]) => `${n}:${s}`).join(',');
  useEffect(() => {
    let dead = false;
    (async () => {
      const out: Record<string, FieldOption[]> = {};
      for (const [name, src] of sources) {
        try {
          out[name] = await resolveOptionsSource(src);
        } catch {
          out[name] = [];
        }
      }
      if (!dead) setOptions(out);
    })();
    return () => {
      dead = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key]);
  return options;
}

export default function DynamicForm({
  fields,
  values,
  errors,
  visibility,
  onChange,
  columns,
}: DynamicFormProps) {
  const grid = (columns ?? 1) > 1;
  const dynOptions = useDynamicOptions(fields);
  const filePreviews = useFileTablePreviews(fields, values);
  // select/radio 的 optionsFromFile：把自动识别的列名注入动态选项
  const mergedDynOptions = useMemo(() => {
    const out = { ...dynOptions };
    for (const f of fields) {
      if ((f.type === 'select' || f.type === 'radio') && f.optionsFromFile) {
        const p = filePreviews[f.optionsFromFile];
        if (p && !p.error && p.columns.length > 0) {
          out[f.name] = p.columns.map((c) => ({ label: c, value: c }));
        }
      }
    }
    return out;
  }, [dynOptions, filePreviews, fields]);
  return (
    <div className={grid ? 'grid items-start gap-3 md:grid-cols-2' : 'space-y-3'}>
      {fields.map((field) => (
        <FieldRenderer
          key={field.name}
          field={field}
          values={values}
          errors={errors}
          visibility={visibility}
          onChange={onChange}
          dynOptions={mergedDynOptions}
          filePreviews={filePreviews}
        />
      ))}
    </div>
  );
}

/** 收集需要表格预览的 file 字段名（previewColumns 开启，或被 optionsFromFile 引用） */
function collectPreviewFiles(fields: InputField[], out: Set<string>): Set<string> {
  for (const f of fields) {
    if (f.type === 'group') {
      collectPreviewFiles(f.children, out);
      continue;
    }
    if (f.type === 'file' && f.previewColumns) out.add(f.name);
    if ((f.type === 'select' || f.type === 'radio') && f.optionsFromFile) out.add(f.optionsFromFile);
  }
  return out;
}

/**
 * 选中表格文件后异步嗅探其列名与数据样本。
 * 按文件签名（名称/大小/修改时间）缓存，避免表单其他字段输入时重复解析。
 */
function useFileTablePreviews(fields: InputField[], values: FormValues): Record<string, FileTablePreview | null> {
  const wanted = useMemo(() => [...collectPreviewFiles(fields, new Set())], [fields]);
  const [previews, setPreviews] = useState<Record<string, FileTablePreview | null>>({});
  const cache = useRef(new Map<string, FileTablePreview>());
  const sig = useMemo(
    () =>
      wanted
        .map((n) => {
          const files = Array.isArray(values[n]) ? (values[n] as File[]) : [];
          const f = files[0];
          return f ? `${n}:${f.name}:${f.size}:${f.lastModified}` : `${n}:`;
        })
        .join('|'),
    [wanted, values],
  );
  useEffect(() => {
    let dead = false;
    (async () => {
      const next: Record<string, FileTablePreview | null> = {};
      for (const n of wanted) {
        const files = Array.isArray(values[n]) ? (values[n] as File[]) : [];
        const f = files[0];
        if (!f) {
          next[n] = null;
          continue;
        }
        const key = `${f.name}:${f.size}:${f.lastModified}`;
        if (cache.current.has(key)) {
          next[n] = cache.current.get(key)!;
          continue;
        }
        try {
          const p = await sniffTableFile(f);
          cache.current.set(key, p);
          next[n] = p;
        } catch (e) {
          next[n] = { columns: [], meta: '', sample: [], error: e instanceof Error ? e.message : String(e) };
        }
        if (dead) return;
      }
      if (!dead) setPreviews(next);
    })();
    return () => {
      dead = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sig, wanted.join(',')]);
  return previews;
}

/** 递归字段渲染：group 特殊处理，其余走 FieldControl */
function FieldRenderer({ field, values, errors, visibility, onChange, dynOptions, filePreviews }: RenderProps & { field: InputField }) {
  // group：自身可见则递归渲染子字段
  if (field.type === 'group') {
    if (visibility.get(field.name) === false) return null;
    const group = (
      <GroupRenderer
        field={field}
        values={values}
        errors={errors}
        visibility={visibility}
        onChange={onChange}
        dynOptions={dynOptions}
        filePreviews={filePreviews}
      />
    );
    // spanFull：两列布局下占满整行（如原始数据包组）
    return field.spanFull ? <div className="md:col-span-2">{group}</div> : group;
  }

  // note：静态提示框，仅展示说明内容（无输入、无校验、不提交）
  if (field.type === 'note') {
    if (visibility.get(field.name) === false) return null;
    const note = (
      <div className="rounded-lg border border-indigo-200/70 bg-indigo-50/70 p-3 text-xs leading-relaxed text-slate-600 dark:border-indigo-500/25 dark:bg-indigo-500/10 dark:text-slate-300">
        {field.label && (
          <div className="mb-1 text-sm font-medium text-slate-700 dark:text-slate-200">
            {field.label}
          </div>
        )}
        <p className="whitespace-pre-line">{field.content}</p>
      </div>
    );
    // spanFull：两列布局下占满整行
    return field.spanFull ? <div className="md:col-span-2">{note}</div> : note;
  }

  // 非 group：不可见则整体不渲染
  if (visibility.get(field.name) === false) return null;

  const disabled = field.disabledWhen ? evaluateCondition(field.disabledWhen, values) : false;
  const handleChange = (v: unknown) => onChange(field.name, v);

  // rawrequest 需要表单级 onChange（解析后一次写入多个字段：target_url/method/param/extra_params）
  const control =
    field.type === 'rawrequest' ? (
      <RawRequestControl
        field={field}
        value={values[field.name]}
        disabled={disabled}
        onChange={onChange}
      />
    ) : (
      <FieldControl
        field={field}
        value={values[field.name]}
        disabled={disabled}
        onChange={handleChange}
        dynOptions={dynOptions}
        filePreviews={filePreviews}
      />
    );

  const wrapped = (
    <FieldWrapper field={field} error={errors[field.name]}>
      {control}
    </FieldWrapper>
  );
  // spanFull：两列布局下占满整行（如原始数据包文本域）
  return field.spanFull ? <div className="md:col-span-2">{wrapped}</div> : wrapped;
}

/** group 容器：支持 collapsible 折叠，可无限嵌套 */
function GroupRenderer({ field, values, errors, visibility, onChange, dynOptions, filePreviews }: RenderProps & { field: Extract<InputField, { type: 'group' }> }) {
  const [open, setOpen] = useState(true);
  const isCollapsible = field.collapsible ?? false;

  return (
    <div className="rounded-lg border border-slate-200 p-2.5 dark:border-slate-700">
      <div className="text-sm font-medium text-slate-700 dark:text-slate-200">
        {isCollapsible ? (
          <button
            type="button"
            onClick={() => setOpen((o) => !o)}
            className="flex w-full items-center gap-1 text-left"
          >
            <ChevronRight className={`h-4 w-4 text-slate-400 transition-transform ${open ? 'rotate-90' : ''}`} />
            {field.label}
          </button>
        ) : (
          <span>{field.label}</span>
        )}
      </div>

      {open && (
        <div className={field.columns === 2 ? 'mt-2.5 grid items-start gap-3 md:grid-cols-2' : 'mt-2.5 space-y-3'}>
          {field.children.map((child) => (
            <FieldRenderer
              key={child.name}
              field={child}
              values={values}
              errors={errors}
              visibility={visibility}
              onChange={onChange}
              dynOptions={dynOptions}
              filePreviews={filePreviews}
            />
          ))}
        </div>
      )}
    </div>
  );
}

/** 字段外壳：label / 必填标记 / 帮助 / 错误；checkbox/switch 走内联布局 */
function FieldWrapper({ field, error, children }: { field: InputField; error?: string; children: React.ReactNode }) {
  const inline = field.type === 'checkbox' || field.type === 'switch';
  const requiredMark = field.required ? <span className="text-red-500">*</span> : null;
  const help = field.help ? <p className="mt-0.5 text-xs text-slate-400">{field.help}</p> : null;
  const errorEl = error ? <p className="mt-0.5 text-xs text-red-500">{error}</p> : null;

  if (inline) {
    return (
      <div>
        <label className="flex items-center gap-2 text-sm">
          {children}
          <span className="font-medium text-slate-700 dark:text-slate-200">
            {field.label}
            {requiredMark}
          </span>
        </label>
        {help}
        {errorEl}
      </div>
    );
  }

  return (
    <div>
      <label className="mb-0.5 block text-sm font-medium text-slate-700 dark:text-slate-200">
        {field.label}
        {requiredMark}
      </label>
      {children}
      {help}
      {errorEl}
    </div>
  );
}

/** 各类型控件渲染 */
function FieldControl({
  field,
  value,
  disabled,
  onChange,
  dynOptions,
  filePreviews,
}: {
  field: ValueField;
  value: unknown;
  disabled: boolean;
  onChange: (v: unknown) => void;
  dynOptions: Record<string, FieldOption[]>;
  filePreviews: Record<string, FileTablePreview | null>;
}) {
  switch (field.type) {
    case 'text':
    case 'password':
      return (
        <input
          type={field.type}
          value={String(value ?? '')}
          placeholder={field.placeholder}
          disabled={disabled}
          className={inputCls}
          onChange={(e) => onChange(e.target.value)}
        />
      );

    case 'number':
      return (
        <input
          type="number"
          value={value === '' || value === null || value === undefined ? '' : String(value)}
          placeholder={field.placeholder}
          disabled={disabled}
          step={field.step}
          className={inputCls}
          onChange={(e) => onChange(e.target.value === '' ? '' : Number(e.target.value))}
        />
      );

    case 'textarea':
      return (
        <textarea
          rows={field.rows ?? 3}
          value={String(value ?? '')}
          placeholder={field.placeholder}
          disabled={disabled}
          className={inputCls}
          onChange={(e) => onChange(e.target.value)}
        />
      );

    case 'select': {
      const opts = field.options ?? dynOptions[field.name] ?? [];
      return (
        <select
          value={String(value ?? '')}
          disabled={disabled}
          className={inputCls}
          onChange={(e) => onChange(e.target.value)}
        >
          {(value === '' || value === null || value === undefined) && (
            <option value="">{field.required ? '请选择…' : '默认（按优先级自动选择）'}</option>
          )}
          {opts.map((o) => (
            <option key={String(o.value)} value={String(o.value)}>
              {o.label}
            </option>
          ))}
        </select>
      );
    }

    case 'radio': {
      const opts = field.options ?? dynOptions[field.name] ?? [];
      return (
        <div className="flex flex-wrap gap-4">
          {opts.map((o) => (
            <label key={String(o.value)} className="flex items-center gap-1.5 text-sm">
              <input
                type="radio"
                name={field.name}
                checked={String(value ?? '') === String(o.value)}
                disabled={disabled}
                onChange={() => onChange(o.value)}
                className="accent-indigo-500"
              />
              {o.label}
            </label>
          ))}
        </div>
      );
    }

    case 'checkbox':
      return (
        <input
          type="checkbox"
          checked={Boolean(value)}
          disabled={disabled}
          onChange={(e) => onChange(e.target.checked)}
          className="accent-indigo-500"
        />
      );

    case 'checkboxGroup':
      return <CheckboxGroupControl field={field} value={value} disabled={disabled} onChange={onChange} />;

    case 'switch':
      return (
        <button
          type="button"
          role="switch"
          aria-checked={Boolean(value)}
          disabled={disabled}
          onClick={() => onChange(!value)}
          className={`relative inline-flex h-5 w-9 shrink-0 items-center rounded-full transition-colors disabled:cursor-not-allowed disabled:opacity-50 ${
            value ? 'bg-indigo-500' : 'bg-slate-300 dark:bg-slate-600'
          }`}
        >
          <span
            className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
              value ? 'translate-x-4' : 'translate-x-0.5'
            }`}
          />
        </button>
      );

    case 'range':
      return <RangeControl field={field} value={value} disabled={disabled} onChange={onChange} />;

    case 'file':
      return (
        <FileControl
          field={field}
          value={value}
          disabled={disabled}
          onChange={onChange}
          preview={field.previewColumns ? (filePreviews[field.name] ?? null) : null}
        />
      );

    case 'keyvalue':
      return <KeyValueControl field={field} value={value} disabled={disabled} onChange={onChange} />;

    case 'code':
      return (
        <textarea
          rows={field.rows ?? 4}
          value={String(value ?? '')}
          placeholder={field.placeholder}
          disabled={disabled}
          spellCheck={false}
          className={`${inputCls} font-mono text-xs`}
          onChange={(e) => onChange(e.target.value)}
        />
      );

    default:
      return null;
  }
}

/**
 * 原始 HTTP 请求包控件：粘贴 → 解析 → 自动填入 target_url/method/param/extra_params，
 * 并提供「注入参数」下拉选择（写入 param 字段）。
 * 接收表单级 onChange(name, value)，可一次写入多个字段。
 */
function RawRequestControl({
  field,
  value,
  disabled,
  onChange,
}: {
  field: Extract<ValueField, { type: 'rawrequest' }>;
  value: unknown;
  disabled: boolean;
  onChange: (name: string, value: unknown) => void;
}) {
  const [parsed, setParsed] = useState<RawHttpParseResult | null>(null);
  const text = String(value ?? '');
  const rows = field.rows ?? 5;

  const handleParse = () => {
    const result = parseRawHttpRequest(text);
    setParsed(result);
    if (result.error) return;
    onChange('target_url', result.url);
    onChange('method', result.method);
    const first = result.params[0]?.name ?? '';
    onChange('param', first);
    onChange(
      'extra_params',
      result.params
        .slice(1)
        .map((p) => `${p.name}=${encodeURIComponent(p.value)}`)
        .join('&'),
    );
  };

  return (
    <div className="space-y-2">
      <textarea
        rows={rows}
        value={text}
        placeholder={field.placeholder}
        disabled={disabled}
        spellCheck={false}
        className={`${inputCls} font-mono text-xs`}
        onChange={(e) => {
          onChange(field.name, e.target.value);
          if (!e.target.value.trim()) setParsed(null);
        }}
      />
      <div className="flex flex-wrap items-center gap-2">
        <button
          type="button"
          disabled={disabled || !text.trim()}
          onClick={handleParse}
          className="rounded-md border border-indigo-500 px-3 py-1.5 text-xs font-medium text-indigo-600 hover:bg-indigo-50 disabled:cursor-not-allowed disabled:border-slate-300 disabled:text-slate-400 dark:text-indigo-400 dark:hover:bg-indigo-500/10 dark:disabled:border-slate-700"
        >
          解析数据包
        </button>
        {parsed?.error && <span className="text-xs text-red-500">{parsed.error}</span>}
        {parsed && !parsed.error && (
          <>
            <span className="truncate text-xs text-slate-500 dark:text-slate-400">
              {parsed.method} {parsed.url}
            </span>
            {parsed.params.length > 0 && (
              <label className="flex items-center gap-1.5 text-xs text-slate-600 dark:text-slate-300">
                注入参数
                <select
                  className={`${inputCls} w-auto py-1`}
                  disabled={disabled}
                  onChange={(e) => onChange('param', e.target.value)}
                >
                  {parsed.params.map((p) => (
                    <option key={p.name} value={p.name}>
                      {p.name}（原值: {p.value.length > 16 ? `${p.value.slice(0, 13)}…` : p.value}）
                    </option>
                  ))}
                </select>
              </label>
            )}
          </>
        )}
      </div>
      {parsed && !parsed.error && (
        <p className="text-xs text-slate-400">
          已自动填入「靶机端点 / 请求方法 / 注入参数名 / 额外固定参数」，上方下拉可切换要测试的参数；
          请求头（Cookie、User-Agent 等）由后端按数据包原样附带。
        </p>
      )}
    </div>
  );
}

function CheckboxGroupControl({
  field,
  value,
  disabled,
  onChange,
}: {
  field: Extract<ValueField, { type: 'checkboxGroup' }>;
  value: unknown;
  disabled: boolean;
  onChange: (v: unknown) => void;
}) {
  const selected = Array.isArray(value) ? (value as string[]) : [];
  return (
    <div className="flex flex-wrap gap-4">
      {field.options.map((o) => (
        <label key={o.value} className="flex items-center gap-1.5 text-sm">
          <input
            type="checkbox"
            checked={selected.includes(o.value)}
            disabled={disabled}
            onChange={(e) => {
              const next = e.target.checked
                ? [...selected, o.value]
                : selected.filter((v) => v !== o.value);
              onChange(next);
            }}
            className="accent-indigo-500"
          />
          {o.label}
        </label>
      ))}
    </div>
  );
}

function RangeControl({
  field,
  value,
  disabled,
  onChange,
}: {
  field: Extract<ValueField, { type: 'range' }>;
  value: unknown;
  disabled: boolean;
  onChange: (v: unknown) => void;
}) {
  const current = typeof value === 'number' ? value : (field.min ?? 0);
  return (
    <div className="flex items-center gap-3">
      <input
        type="range"
        min={field.min ?? 0}
        max={field.max ?? 100}
        step={field.step ?? 1}
        value={current}
        disabled={disabled}
        onChange={(e) => onChange(Number(e.target.value))}
        className="flex-1 accent-indigo-500"
      />
      <span className="w-8 shrink-0 text-right text-sm text-slate-600 dark:text-slate-300">
        {current}
      </span>
    </div>
  );
}

function FileControl({
  field,
  value,
  disabled,
  onChange,
  preview,
}: {
  field: Extract<ValueField, { type: 'file' }>;
  value: unknown;
  disabled: boolean;
  onChange: (v: unknown) => void;
  /** 选中后自动识别的表格列预览（field.previewColumns 开启时传入） */
  preview?: FileTablePreview | null;
}) {
  const files = Array.isArray(value) ? (value as File[]) : [];
  // 文件夹模式：webkitdirectory（WebView2/Chromium 支持），递归选取整个文件夹
  const dirProps = field.directory
    ? ({ webkitdirectory: '' } as InputHTMLAttributes<HTMLInputElement>)
    : {};
  const displayName = (f: File) =>
    (f as File & { webkitRelativePath?: string }).webkitRelativePath || f.name;
  return (
    <div>
      <input
        type="file"
        multiple={field.multiple || !!field.directory}
        accept={field.accept}
        disabled={disabled}
        {...dirProps}
        className="block w-full text-sm text-slate-500 file:mr-3 file:rounded-md file:border-0 file:bg-indigo-50 file:px-3 file:py-1.5 file:text-sm file:font-medium file:text-indigo-600 hover:file:bg-indigo-100 dark:text-slate-400 dark:file:bg-indigo-500/10 dark:file:text-indigo-400"
        onChange={(e) => {
          const list = Array.from(e.target.files ?? []);
          onChange(field.multiple || field.directory ? list : list.slice(0, 1));
        }}
      />
      {files.length > 0 && (
        <ul className="mt-2 max-h-40 space-y-1 overflow-auto text-xs text-slate-500 dark:text-slate-400">
          {files.map((f, i) => (
            <li key={`${f.name}-${i}`} className="flex items-center gap-2">
              <span className="truncate" title={displayName(f)}>{displayName(f)}</span>
              <span className="shrink-0 text-slate-400">{(f.size / 1024).toFixed(1)} KB</span>
            </li>
          ))}
          <li>
            <button
              type="button"
              disabled={disabled}
              onClick={() => onChange([])}
              className="text-indigo-500 hover:underline"
            >
              清空
            </button>
          </li>
        </ul>
      )}
      {preview?.error && (
        <p className="mt-2 rounded-md border border-amber-200 bg-amber-50 p-2 text-xs text-amber-700 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-400">
          {preview.error}
        </p>
      )}
      {preview && !preview.error && preview.columns.length > 0 && (
        <div className="mt-2 rounded-md border border-slate-200 bg-slate-50/60 p-2 dark:border-slate-700 dark:bg-slate-900/60">
          <div className="mb-1.5 flex flex-wrap items-center gap-1.5 text-[11px] text-slate-500 dark:text-slate-400">
            <span className="font-medium text-slate-600 dark:text-slate-300">{preview.meta}</span>
            <span>· {preview.columns.length} 列</span>
          </div>
          <div className="mb-1.5 flex flex-wrap gap-1">
            {preview.columns.map((c, i) => (
              <span
                key={i}
                className="rounded bg-white px-1.5 py-0.5 font-mono text-[11px] text-slate-600 ring-1 ring-slate-200 dark:bg-slate-800 dark:text-slate-300 dark:ring-slate-700"
              >
                {c}
              </span>
            ))}
          </div>
          {preview.sample.length > 0 && (
            <div className="max-w-full overflow-auto rounded border border-slate-200 bg-white dark:border-slate-700 dark:bg-slate-900">
              <table className="text-[11px]">
                <thead>
                  <tr>
                    {preview.columns.map((c, i) => (
                      <th
                        key={i}
                        className="whitespace-nowrap border-b border-slate-200 px-2 py-1 text-left font-medium text-slate-600 dark:border-slate-700 dark:text-slate-300"
                      >
                        {c}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {preview.sample.map((r, i) => (
                    <tr key={i}>
                      {preview.columns.map((_, j) => (
                        <td
                          key={j}
                          className="max-w-40 truncate whitespace-nowrap border-b border-slate-100 px-2 py-1 text-slate-500 dark:border-slate-800 dark:text-slate-400"
                          title={r[j] ?? ''}
                        >
                          {r[j] ?? ''}
                        </td>
                      ))}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

function KeyValueControl({
  field,
  value,
  disabled,
  onChange,
}: {
  field: Extract<ValueField, { type: 'keyvalue' }>;
  value: unknown;
  disabled: boolean;
  onChange: (v: unknown) => void;
}) {
  const pairs = Array.isArray(value) ? (value as { key: string; value: string }[]) : [];
  return (
    <div className="space-y-2">
      {pairs.map((p, i) => (
        <div key={i} className="flex items-center gap-2">
          <input
            value={p.key}
            disabled={disabled}
            placeholder={field.keyPlaceholder ?? '键'}
            className={inputCls}
            onChange={(e) =>
              onChange(pairs.map((x, idx) => (idx === i ? { ...x, key: e.target.value } : x)))
            }
          />
          <input
            value={p.value}
            disabled={disabled}
            placeholder={field.valuePlaceholder ?? '值'}
            className={inputCls}
            onChange={(e) =>
              onChange(pairs.map((x, idx) => (idx === i ? { ...x, value: e.target.value } : x)))
            }
          />
          <button
            type="button"
            disabled={disabled}
            onClick={() => onChange(pairs.filter((_, idx) => idx !== i))}
            className="shrink-0 rounded p-1 text-slate-400 hover:text-red-500 disabled:cursor-not-allowed"
          >
            <X className="h-4 w-4" />
          </button>
        </div>
      ))}
      <button
        type="button"
        disabled={disabled}
        onClick={() => onChange([...pairs, { key: '', value: '' }])}
        className="flex items-center gap-1 text-sm text-indigo-500 hover:text-indigo-600 disabled:cursor-not-allowed"
      >
        <Plus className="h-4 w-4" /> 添加
      </button>
    </div>
  );
}
