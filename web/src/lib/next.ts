// safeNext keeps the address a sign-in returns to on this site: a path from
// its root, never one a browser reads as another host ("//" or "/\"), and
// never the sign-in page itself, which would send a signed-in reader round
// in circles.
export function safeNext(next: string | null | undefined, fallback: string): string {
  if (!next || !/^\/(?![/\\])/.test(next) || /^\/(?:en\/)?login(?:[/?#]|$)/.test(next)) return fallback;
  return next;
}
