/** @type {import('tailwindcss').Config} */
export default {
  // 扫描所有源码与入口，生成原子类
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  // 使用 class 策略做暗色模式（配合 html.dark）
  darkMode: 'class',
  theme: {
    extend: {
      // 设计令牌(design-system.md §2.8,批 1 落地):语义别名,新组件只准消费这些类名
      colors: {
        brand: { DEFAULT: 'var(--ct-brand)', hover: 'var(--ct-brand-hover)', subtle: 'var(--ct-brand-subtle)' },
        surface: { DEFAULT: 'var(--ct-surface)', raised: 'var(--ct-surface-raised)', hover: 'var(--ct-surface-hover)' },
        line: { DEFAULT: 'var(--ct-line)', strong: 'var(--ct-line-strong)' },
        ink: { 1: 'var(--ct-ink-1)', 2: 'var(--ct-ink-2)', 3: 'var(--ct-ink-3)', inverse: 'var(--ct-ink-inverse)' },
        status: {
          ok: 'var(--ct-ok)', okBg: 'var(--ct-ok-bg)', okLine: 'var(--ct-ok-line)',
          warn: 'var(--ct-warn)', warnBg: 'var(--ct-warn-bg)', warnLine: 'var(--ct-warn-line)',
          danger: 'var(--ct-danger)', dangerBg: 'var(--ct-danger-bg)', dangerLine: 'var(--ct-danger-line)',
          muted: 'var(--ct-muted)', mutedBg: 'var(--ct-muted-bg)', mutedLine: 'var(--ct-muted-line)',
        },
      },
      fontSize: { '2xs': ['11px', '16px'] },
      fontFamily: { sans: ['var(--ct-font-sans)'], mono: ['var(--ct-font-mono)'] },
      boxShadow: {
        card: '0 1px 2px rgb(15 23 42 / .06)',
        pop: '0 4px 12px rgb(15 23 42 / .10)',
        modal: '0 12px 32px rgb(15 23 42 / .16)',
      },
      zIndex: { sticky: 10, overlay: 20, modal: 30, drawer: 40, top: 50 },
      transitionDuration: { fast: '100ms', base: '150ms', slow: '200ms' },
    },
  },
  plugins: [],
};
