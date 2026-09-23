import { en } from "./en";
import { zh, type Dict } from "./zh";

export type Lang = "zh" | "en";

export function dict(lang: Lang): Dict {
  return lang === "zh" ? zh : en;
}

export function other(lang: Lang): Lang {
  return lang === "zh" ? "en" : "zh";
}

/** localePath adds the /en prefix; Chinese lives at the root. */
export function localePath(lang: Lang, path: string): string {
  if (lang === "zh") return path;
  return path === "/" ? "/en/" : `/en${path}`;
}
