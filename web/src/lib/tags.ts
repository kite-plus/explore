import type { Lang } from "@/i18n";
import { api } from "@/lib/api";
import type { Tag } from "@/lib/types";

// The list only changes when the API is deployed, so one copy an hour does.
const TTL_MS = 60 * 60 * 1000;
let cached: { at: number; tags: Tag[] } | undefined;

/** The tag list, or undefined when the API cannot be asked and nothing is cached. */
export async function tagList(lang: Lang): Promise<Tag[] | undefined> {
  if (cached && Date.now() - cached.at < TTL_MS) return cached.tags;
  const r = await api.tags(lang);
  if (r.kind !== "ok") return cached?.tags;
  cached = { at: Date.now(), tags: r.data.data };
  return cached.tags;
}

/** Tag names in the interface language, by slug. */
export function tagNames(tags: Tag[], lang: Lang): Record<string, string> {
  return Object.fromEntries(tags.map((t) => [t.slug, t.name[lang]]));
}
