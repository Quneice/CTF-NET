/**
 * 红队 Wiki 工作区薄壳:注入红队分类与 lib/wiki 适配器,复用通用 WikiWorkspace。
 * (逻辑全部在 WikiWorkspace.tsx,行为与抽取前一致。)
 */
import WikiWorkspace from './WikiWorkspace';
import { WIKI_CATEGORIES, wikiCategoryById } from '@/mock/redteamWiki';
import {
  createWikiEntry,
  deleteWikiEntry,
  listWikiEntries,
  updateWikiEntry,
} from '@/lib/wiki';

export default function RedTeamWikiWorkspace() {
  return (
    <WikiWorkspace
      categories={WIKI_CATEGORIES}
      categoryById={wikiCategoryById}
      adapter={{
        list: listWikiEntries,
        create: createWikiEntry,
        update: updateWikiEntry,
        delete: deleteWikiEntry,
      }}
    />
  );
}
