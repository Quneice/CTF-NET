import type { FormValues, ToolDefinition } from '@/types';

import { useToolForm } from '@/hooks/useToolForm';
import DynamicForm from '@/components/form/DynamicForm';

/**
 * 工具表单面板：动态表单 + 「填充示例值」 + 「提交」按钮。
 * 提交前做必填校验，通过后把「可见字段」的值交给 onSubmit（隐藏字段不参与）。
 */
export default function ToolForm({
  tool,
  onSubmit,
  initialValues,
  configDefaults,
}: {
  tool: ToolDefinition;
  onSubmit: (values: FormValues) => void;
  initialValues?: FormValues;
  /** 配置文件默认值（设置页统一配置 → 表单预填，仅填充空字段） */
  configDefaults?: FormValues;
}) {
  const { values, errors, visibility, setFieldValue, fillExamples, validate, getVisibleValues } =
    useToolForm(tool, initialValues, configDefaults);

  const handleSubmit = () => {
    if (validate()) {
      onSubmit(getVisibleValues());
    }
  };

  return (
    <div>
      <DynamicForm
        fields={tool.inputSchema}
        columns={tool.formColumns}
        values={values}
        errors={errors}
        visibility={visibility}
        onChange={setFieldValue}
      />

      <div className="mt-5 flex items-center gap-3 border-t border-slate-200 pt-4 dark:border-slate-800">
        <button
          type="button"
          onClick={fillExamples}
          className="rounded-md border border-slate-300 px-3 py-2 text-sm text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800"
        >
          填充示例值
        </button>
        <button
          type="button"
          onClick={handleSubmit}
          className="rounded-md bg-indigo-500 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-600"
        >
          提交
        </button>
      </div>
    </div>
  );
}
