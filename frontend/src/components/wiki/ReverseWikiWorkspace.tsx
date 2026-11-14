/**
 * 逆向 Wiki 工作区薄壳:注入逆向分类与 lib/reverseWiki 适配器,复用通用 WikiWorkspace。
 * 数据持久化在后端 SQLite(reverse_wiki.db),浏览器模式回退 localStorage。
 */
import WikiWorkspace from './WikiWorkspace';
import AsciiConverter from './AsciiConverter';
import { REVERSE_WIKI_CATEGORIES, reverseCategoryById } from '@/mock/reverseWiki';
import {
  createReverseWikiEntry,
  deleteReverseWikiEntry,
  listReverseWikiEntries,
  updateReverseWikiEntry,
} from '@/lib/reverseWiki';

/** 模块级常量:引用稳定,避免父组件重渲染时触发工作区重新拉取 */
const adapter = {
  list: listReverseWikiEntries,
  create: createReverseWikiEntry,
  update: updateReverseWikiEntry,
  delete: deleteReverseWikiEntry,
};

export default function ReverseWikiWorkspace() {
  return (
    <WikiWorkspace
      categories={REVERSE_WIKI_CATEGORIES}
      categoryById={reverseCategoryById}
      topWidget={<AsciiConverter />}
      adapter={adapter}
    />
  );
}
