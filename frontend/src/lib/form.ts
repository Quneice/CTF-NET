import type { Condition, FormValues, InputField } from '@/types';

/**
 * 动态表单纯工具函数：条件求值、可见性计算、初始值构建、必填校验。
 * 全部为纯函数，不依赖 React，便于单元测试与复用。
 */

/** 判断值是否为空（用于必填校验与配置默认值回填） */
export function isEmptyValue(v: unknown): boolean {
  if (v === undefined || v === null) return true;
  if (typeof v === 'string') return v.trim() === '';
  if (Array.isArray(v)) return v.length === 0;
  return false;
}

/** 求值单个条件 */
export function evaluateCondition(cond: Condition, values: FormValues): boolean {
  const current = values[cond.field];
  switch (cond.op) {
    case 'eq':
      return current === cond.value;
    case 'neq':
      return current !== cond.value;
    case 'in':
      return Array.isArray(cond.value) && cond.value.includes(current);
    case 'truthy':
      return Boolean(current);
    default:
      return true;
  }
}

/** 递归计算每个字段的可见性（group 的可见性会继承给子字段） */
export function computeVisibility(
  fields: InputField[],
  values: FormValues,
): Map<string, boolean> {
  const map = new Map<string, boolean>();
  const walk = (list: InputField[], parentVisible: boolean) => {
    for (const f of list) {
      const visible = parentVisible && (f.visibleWhen ? evaluateCondition(f.visibleWhen, values) : true);
      map.set(f.name, visible);
      if (f.type === 'group') walk(f.children, visible);
    }
  };
  walk(fields, true);
  return map;
}

/** 各类型的空默认值（defaultValue 缺省时使用） */
function defaultForType(f: InputField): unknown {
  switch (f.type) {
    case 'checkbox':
    case 'switch':
      return false;
    case 'checkboxGroup':
    case 'keyvalue':
    case 'file':
      return [];
    case 'range':
      return f.min ?? 0;
    default:
      return '';
  }
}

/**
 * 递归收集所有值字段（跳过 group 容器与 note 提示框），构建初始值。
 * defaultValue 即「示例值」，供「填充示例值」与首次初始化使用。
 */
export function buildInitialValues(fields: InputField[]): FormValues {
  const values: FormValues = {};
  const walk = (list: InputField[]) => {
    for (const f of list) {
      if (f.type === 'group') {
        walk(f.children);
        continue;
      }
      if (f.type === 'note') continue; // 静态提示框不产生值
      values[f.name] = f.defaultValue !== undefined ? f.defaultValue : defaultForType(f);
    }
  };
  walk(fields);
  return values;
}

/**
 * 必填校验：只校验当前可见的非布尔字段（隐藏字段不阻塞提交）。
 * 返回 `{ 字段名: 错误信息 }`。
 */
export function validateRequired(
  fields: InputField[],
  values: FormValues,
  visibility: Map<string, boolean>,
): Record<string, string> {
  const errors: Record<string, string> = {};
  const walk = (list: InputField[]) => {
    for (const f of list) {
      if (f.type === 'group') {
        walk(f.children);
        continue;
      }
      if (f.type === 'note') continue; // 静态提示框无必填概念
      if (!f.required) continue;
      // 隐藏字段不校验；布尔字段（checkbox/switch）的 false 也是有效值，不参与必填
      if (visibility.get(f.name) === false) continue;
      if (f.type === 'checkbox' || f.type === 'switch') continue;
      if (isEmptyValue(values[f.name])) {
        errors[f.name] = `${f.label}为必填项`;
      }
    }
  };
  walk(fields);
  return errors;
}
