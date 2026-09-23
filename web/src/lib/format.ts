import type { Lang } from "@/i18n";

const WEEK = 7 * 24 * 60 * 60 * 1000;

// Chinese pages show Beijing time and English pages UTC; see
// docs/design/frontend.md section 3.4.
const zone = (lang: Lang) => (lang === "zh" ? "Asia/Shanghai" : "UTC");
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

export function formatDate(iso: string, lang: Lang): string {
  return new Intl.DateTimeFormat(locale(lang), { dateStyle: "medium", timeZone: zone(lang) }).format(new Date(iso));
}
