// Blog avatars are the first letter of the name on a color picked from the
// host. Fixed classes rather than a computed color keep inline style
// attributes out of the page, which the CSP would have to allow.
const palette = [
  "bg-rose-600",
  "bg-orange-600",
  "bg-amber-600",
  "bg-emerald-600",
  "bg-teal-600",
  "bg-sky-600",
  "bg-indigo-600",
  "bg-violet-600",
  "bg-fuchsia-600",
  "bg-slate-600",
];

export function avatarColor(host: string): string {
  let h = 0;
  for (const ch of host) {
    h = (h * 31 + (ch.codePointAt(0) ?? 0)) >>> 0;
  }
  return palette[h % palette.length];
}

export function initial(name: string): string {
  const first = Array.from(name.trim())[0] ?? "?";
  return first.toUpperCase();
}
