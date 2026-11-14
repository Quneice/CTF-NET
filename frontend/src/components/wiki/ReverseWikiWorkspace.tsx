/**
 * 逆向 Wiki 工作区薄壳:注入逆向分类与 lib/reverseWiki 适配器,复用通用 WikiWorkspace。
 * 数据持久化在后端 SQLite(reverse_wiki.db),浏览器模式回退 localStorage。
 */
import WikiWorkspace from './WikiWorkspace';
import { REVERSE_WIKI_CATEGORIES, reverseCategoryById } from '@/mock/reverseWiki';
import {
  createReverseWikiEntry,
  deleteReverseWikiEntry,
  listReverseWikiEntries,
  updateReverseWikiEntry,
} from '@/lib/reverseWiki';

export default function ReverseWikiWorkspace() {
  return (
    <WikiWorkspace
      categories={REVERSE_WIKI_CATEGORIES}
      categoryById={reverseCategoryById}
      adapter={{
        list: listReverseWikiEntries,
        create: createReverseWikiEntry,
        update: updateReverseWikiEntry,
        delete: deleteReverseWikiEntry,
      }}
    />
  );
}
