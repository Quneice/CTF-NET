import { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';

import { getIcon } from '@/lib/icons';
import { MODE_CONFIG, toolsForMode, type AppMode } from '@/mock/modes';
import { useModeStore } from '@/stores/useModeStore';

/**
 * 模式选择页（/mode）：每次启动应用先落到本页。
 * 两张卡片分别对应「CTF 模式 / 渗透测试模式」，点击进入首页。
 */
export default function ModeSelectPage() {
  const setMode = useModeStore((s) => s.setMode);
  const navigate = useNavigate();

  useEffect(() => {
    document.title = 'CTF-NET — 选择模式';
  }, []);

  const pick = (mode: AppMode) => {
    setMode(mode);
    navigate('/');
  };

  return (
    <div className="flex h-screen w-full items-center justify-center bg-slate-50 p-6 dark:bg-slate-950">
      <div className="w-full max-w-3xl">
        <header className="text-center">
          <h1 className="text-3xl font-semibold text-slate-800 dark:text-slate-100">CTF-NET</h1>
          <p className="mt-2 text-slate-500 dark:text-slate-400">选择本次使用的工作模式</p>
        </header>

        <div className="mt-10 grid gap-6 sm:grid-cols-2">
          {(Object.keys(MODE_CONFIG) as AppMode[]).map((mode) => {
            const info = MODE_CONFIG[mode];
            const Icon = getIcon(info.icon);
            const count = toolsForMode(mode).length;
            return (
              <button
                key={mode}
                type="button"
                onClick={() => pick(mode)}
                className="group flex flex-col items-start rounded-2xl border border-slate-200 bg-white p-6 text-left shadow-sm transition-all hover:-translate-y-0.5 hover:border-indigo-300 hover:shadow-lg dark:border-slate-800 dark:bg-slate-900 dark:hover:border-indigo-500/50"
              >
                <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-indigo-50 text-indigo-500 transition-colors group-hover:bg-indigo-100 dark:bg-indigo-500/10 dark:text-indigo-400">
                  <Icon className="h-6 w-6" />
                </div>
                <h2 className="mt-4 text-lg font-semibold text-slate-800 dark:text-slate-100">
                  {info.label}
                </h2>
                <p className="mt-2 text-sm leading-relaxed text-slate-500 dark:text-slate-400">
                  {info.description}
                </p>
                <span className="mt-4 rounded-full border border-slate-200 px-2.5 py-0.5 text-xs text-slate-400 dark:border-slate-700">
                  {count} 个工具
                </span>
              </button>
            );
          })}
        </div>
      </div>
    </div>
  );
}
