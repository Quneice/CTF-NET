import { useCallback, useMemo, useState } from 'react';

import type { FormValues, ToolDefinition } from '@/types';

import { buildInitialValues, computeVisibility, isEmptyValue, validateRequired } from '@/lib/form';

/**
 * 工具表单状态 hook。
 *
 * 职责：
 * 1. 以 toolId 为粒度，把表单值持久化到 localStorage，刷新后可恢复；
 * 2. 实时计算可见性（visibleWhen 基于当前值）；
 * 3. 必填校验；
 * 4. 「填充示例值」（恢复为 defaultValue）。
 */

const STORAGE_PREFIX = 'ctf-net:form:';

/** 文件对象无法 JSON 序列化，需要跳过 */
function isFileValue(v: unknown): boolean {
  if (v instanceof File) return true;
  if (Array.isArray(v)) return v.some((x) => x instanceof File);
  return false;
}

function loadPersisted(toolId: string): FormValues | null {
  try {
    const raw = localStorage.getItem(STORAGE_PREFIX + toolId);
    if (!raw) return null;
    const parsed = JSON.parse(raw);
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed)
      ? (parsed as FormValues)
      : null;
  } catch {
    return null;
  }
}

function persist(toolId: string, values: FormValues) {
  try {
    const safe: FormValues = {};
    for (const [k, v] of Object.entries(values)) {
      if (isFileValue(v)) continue; // 文件值仅内存态，不持久化
      safe[k] = v;
    }
    localStorage.setItem(STORAGE_PREFIX + toolId, JSON.stringify(safe));
  } catch {
    // 忽略存储异常（隐私模式 / 配额超限）
  }
}

export function useToolForm(tool: ToolDefinition, initialValues?: FormValues, configDefaults?: FormValues) {
  const [values, setValues] = useState<FormValues>(() => {
    // 优先级：schema defaultValue < 配置文件默认值(configDefaults) < 用户已填值(localStorage) < 历史回填(initialValues)
    // configDefaults 只在字段为空时生效，用户改过的值不会被配置覆盖
    const base = buildInitialValues(tool.inputSchema);
    const merged: FormValues = { ...base };
    if (configDefaults) {
      for (const [k, v] of Object.entries(configDefaults)) merged[k] = v;
    }
    const persisted = loadPersisted(tool.id);
    if (persisted) {
      for (const [k, v] of Object.entries(persisted)) {
        if (!isEmptyValue(v)) merged[k] = v;
      }
    }
    // 历史「回填」等场景：initialValues 优先级最高，覆盖默认值与持久化值
    return initialValues ? { ...merged, ...initialValues } : merged;
  });

  const [errors, setErrors] = useState<Record<string, string>>({});

  // 可见性基于当前值实时计算
  const visibility = useMemo(
    () => computeVisibility(tool.inputSchema, values),
    [tool.inputSchema, values],
  );

  const setFieldValue = useCallback(
    (name: string, value: unknown) => {
      setValues((prev) => {
        const next = { ...prev, [name]: value };
        persist(tool.id, next);
        return next;
      });
      // 修改后清除该字段错误
      setErrors((prev) => {
        if (!prev[name]) return prev;
        const next = { ...prev };
        delete next[name];
        return next;
      });
    },
    [tool.id],
  );

  /** 填充示例值：恢复为 schema defaultValue + 配置文件默认值 */
  const fillExamples = useCallback(() => {
    const next = buildInitialValues(tool.inputSchema);
    if (configDefaults) {
      for (const [k, v] of Object.entries(configDefaults)) next[k] = v;
    }
    setValues(next);
    persist(tool.id, next);
    setErrors({});
  }, [tool.id, tool.inputSchema, configDefaults]);

  /** 提交前校验，返回是否通过 */
  const validate = useCallback((): boolean => {
    const errs = validateRequired(tool.inputSchema, values, visibility);
    setErrors(errs);
    return Object.keys(errs).length === 0;
  }, [tool.inputSchema, values, visibility]);

  /** 取可见字段的值（隐藏字段不参与提交） */
  const getVisibleValues = useCallback((): FormValues => {
    const out: FormValues = {};
    for (const [name, v] of Object.entries(values)) {
      if (visibility.get(name) === false) continue;
      out[name] = v;
    }
    return out;
  }, [values, visibility]);

  return {
    values,
    errors,
    visibility,
    setFieldValue,
    fillExamples,
    validate,
    getVisibleValues,
  };
}
