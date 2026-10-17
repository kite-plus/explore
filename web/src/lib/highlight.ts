// Marks a search's words in titles and excerpts the way the API matches
// them (internal/store/search.go): Chinese as typed, other words whole.
// No imports, so the tests run it straight from Node.

/** Scripts written without spaces between words; the API's cjk ranges. */
const CJK_RANGES: [number, number][] = [
  [0x3040, 0x30ff],
  [0x3400, 0x4dbf],
  [0x4e00, 0x9fff],
  [0xac00, 0xd7af],
  [0xf900, 0xfaff],
  [0x20000, 0x3134f],
];

const MAX_WORDS = 8;

export function isCJK(char: string): boolean {
  const c = char.codePointAt(0) ?? 0;
  return CJK_RANGES.some(([from, to]) => c >= from && c <= to);
}

const wordChar = (char: string) => /[\p{L}\p{N}]/u.test(char);

/**
 * searchWords splits a query into the words the API looks for: at spaces,
 * and at punctuation standing alone between them, as in "你好，世界".
 */
export function searchWords(q: string): string[] {
  const words: string[] = [];
  for (const field of q.toLowerCase().split(/\s+/u)) {
    let word = "";
    let run = "";
    const flush = () => {
      if (run && !isCJK(run) && ![...run].some(wordChar)) {
        if (word) words.push(word);
        word = "";
      } else {
        word += run;
      }
      run = "";
    };
    for (const char of field) {
      if (run && isCJK(char) !== isCJK(run)) flush();
      run += char;
    }
    flush();
    if (word) words.push(word);
  }
  return words.slice(0, MAX_WORDS);
}

export interface Segment {
  text: string;
  mark: boolean;
}

const cjkClass = CJK_RANGES.map(([from, to]) => `${String.fromCodePoint(from)}-${String.fromCodePoint(to)}`).join("");
// A letter or digit that is not CJK: what may not touch a whole word.
const boundary = `[[\\p{L}\\p{N}]--[${cjkClass}]]`;

const escape = (s: string) => s.replace(/[\\^$.*+?()[\]{}|/]/g, "\\$&");

/** highlight cuts text into the parts that match words and those that do not. */
export function highlight(text: string, words: string[]): Segment[] {
  const sources = [...new Set(words.filter(Boolean))]
    .sort((a, b) => b.length - a.length)
    .map((word) => {
      const chars = [...word];
      const before = isCJK(chars[0]) ? "" : `(?<!${boundary})`;
      const after = isCJK(chars[chars.length - 1]) ? "" : `(?!${boundary})`;
      return before + escape(word) + after;
    });
  if (!sources.length || !text) return [{ text, mark: false }];
  let pattern: RegExp;
  try {
    pattern = new RegExp(sources.join("|"), "giv");
  } catch {
    return [{ text, mark: false }];
  }
  const parts: Segment[] = [];
  let at = 0;
  for (const match of text.matchAll(pattern)) {
    if (!match[0]) continue;
    if (match.index > at) parts.push({ text: text.slice(at, match.index), mark: false });
    parts.push({ text: match[0], mark: true });
    at = match.index + match[0].length;
  }
  if (at < text.length) parts.push({ text: text.slice(at), mark: false });
  return parts;
}
