/**
 * 逆向 Wiki —— 数据层。
 *
 * 桌面运行时走 wailsjs 生成的 App 绑定(reverse_wiki.go);
 * 纯浏览器 `npm run dev`(isWails() 为 false)回退 localStorage 实现(同一接口),
 * 首次使用时用 mock/reverseWiki.ts 的 REVERSE_WIKI_SEED 灌入内置速查数据。
 * 与 lib/wiki.ts(红队 Wiki)同款架构,localStorage 键相互独立。
 */
import * as rawApp from '../../wailsjs/go/main/App';
import { isWails } from './backend';
import { REVERSE_WIKI_SEED } from '@/mock/reverseWiki';
import type { WikiEntry } from './wiki';

export type { WikiEntry };

// —— Wails 绑定 ——

const App = rawApp as unknown as {
  ListReverseWikiEntries(category: string): Promise<WikiEntry[]>;
  CreateReverseWikiEntry(category: string, title: string, content: string): Promise<WikiEntry>;
  UpdateReverseWikiEntry(id: string, title: string, content: string): Promise<WikiEntry>;
  DeleteReverseWikiEntry(id: string): Promise<void>;
};

// —— localStorage 回退实现(纯浏览器开发模式)——

const LS_REVERSE_WIKI = 'ctfnet.reverse_wiki_entries';

function lsRead(): WikiEntry[] {
  try {
    return JSON.parse(localStorage.getItem(LS_REVERSE_WIKI) || '[]') as WikiEntry[];
  } catch {
    return [];
  }
}

function lsWrite(data: WikiEntry[]): void {
  localStorage.setItem(LS_REVERSE_WIKI, JSON.stringify(data));
}

function lsId(): string {
  return Array.from(crypto.getRandomValues(new Uint8Array(8)))
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('');
}

/** 首次使用时用内置速查数据初始化 localStorage */
function lsEnsureSeeded(): void {
  if (localStorage.getItem(LS_REVERSE_WIKI) !== null) return;
  const now = Date.now() / 1000;
  lsWrite(
    REVERSE_WIKI_SEED.map((s) => ({
      id: lsId(),
      category: s.category,
      title: s.title,
      content: s.content,
      created_at: now,
      updated_at: now,
    })),
  );
}

const local = {
  async list(category: string): Promise<WikiEntry[]> {
    lsEnsureSeeded();
    return lsRead()
      .filter((e) => !category || e.category === category)
      .sort((a, b) => b.updated_at - a.updated_at);
  },
  async create(category: string, title: string, content: string): Promise<WikiEntry> {
    if (!category) throw new Error('分类不能为空');
    if (!title) throw new Error('标题不能为空');
    lsEnsureSeeded();
    const now = Date.now() / 1000;
    const e: WikiEntry = { id: lsId(), category, title, content, created_at: now, updated_at: now };
    lsWrite([...lsRead(), e]);
    return e;
  },
  async update(id: string, title: string, content: string): Promise<WikiEntry> {
    if (!title) throw new Error('标题不能为空');
    const all = lsRead();
    const i = all.findIndex((e) => e.id === id);
    if (i < 0) throw new Error(`条目不存在: ${id}`);
    all[i] = { ...all[i], title, content, updated_at: Date.now() / 1000 };
    lsWrite(all);
    return all[i];
  },
  async remove(id: string): Promise<void> {
    lsWrite(lsRead().filter((e) => e.id !== id));
  },
};

// —— 对外 API(按运行时自动选择实现)——

export const listReverseWikiEntries = (category: string): Promise<WikiEntry[]> =>
  isWails() ? App.ListReverseWikiEntries(category) : local.list(category);

export const createReverseWikiEntry = (category: string, title: string, content: string): Promise<WikiEntry> =>
  isWails() ? App.CreateReverseWikiEntry(category, title, content) : local.create(category, title, content);

export const updateReverseWikiEntry = (id: string, title: string, content: string): Promise<WikiEntry> =>
  isWails() ? App.UpdateReverseWikiEntry(id, title, content) : local.update(id, title, content);

export const deleteReverseWikiEntry = (id: string): Promise<void> =>
  isWails() ? App.DeleteReverseWikiEntry(id) : local.remove(id);
