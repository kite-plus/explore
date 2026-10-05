import type { Lang } from "@/i18n";

const WEEK = 7 * 24 * 60 * 60 * 1000;

// Chinese pages show Beijing time and English pages UTC; see
// docs/design/frontend.md section 3.4.
export const zone = (lang: Lang) => (lang === "zh" ? "Asia/Shanghai" : "UTC");
const locale = (lang: Lang) => (lang === "zh" ? "zh-CN" : "en");

/** relativeTime reads "3 hours ago" within a week and a date after that. */
export function relativeTime(iso: string, lang: Lang, now = new Date()): string {
  const then = new Date(iso);
  const diff = now.getTime() - then.getTime();
  if (diff < 0 || diff >= WEEK) {
    return formatDate(iso, lang);
  }
  const rtf = new Intl.RelativeTimeFormat(locale(lang), { numeric: "auto" });
  const minutes = Math.floor(diff / 60_000);
  if (minutes < 60) return rtf.format(-Math.max(minutes, 1), "minute");
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return rtf.format(-hours, "hour");
  return rtf.format(-Math.floor(hours / 24), "day");
}

const dayOf = (date: Date, lang: Lang) =>
  new Intl.DateTimeFormat("en-CA", { timeZone: zone(lang), year: "numeric", month: "2-digit", day: "2-digit" }).format(date);

/** dayKey is the calendar day, YYYY-MM-DD, a moment falls on in the page's zone. */
export function dayKey(iso: string, lang: Lang): string {
  return dayOf(new Date(iso), lang);
}

/**
 * dayHeading names a day for a stream's group: "today" or "yesterday" with
 * the date beside it, or the date alone, with the year only when it is not
 * this one.
 */
export function dayHeading(key: string, lang: Lang, words: { today: string; yesterday: string }, now = new Date()): { name: string; date?: string } {
  const noon = new Date(`${key}T12:00:00Z`);
  const date = new Intl.DateTimeFormat(locale(lang), {
    ...(key.slice(0, 4) === dayOf(now, lang).slice(0, 4) ? {} : { year: "numeric" }),
    month: "long",
    day: "numeric",
    weekday: "short",
    timeZone: "UTC",
  }).format(noon);
  if (key === dayOf(now, lang)) return { name: words.today, date };
  if (key === dayOf(new Date(now.getTime() - 86_400_000), lang)) return { name: words.yesterday, date };
  return { name: date };
}

/** clockTime is the time of day, for a post under a heading that names its day. */
export function clockTime(iso: string, lang: Lang): string {
  return new Intl.DateTimeFormat(locale(lang), { hour: "2-digit", minute: "2-digit", timeZone: zone(lang) }).format(new Date(iso));
}

export function formatDate(iso: string, lang: Lang): string {
  return new Intl.DateTimeFormat(locale(lang), { dateStyle: "medium", timeZone: zone(lang) }).format(new Date(iso));
}
