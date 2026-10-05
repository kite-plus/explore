/** A run of plain text, or a web address in it with where it leads. */
export type Piece = { text: string; href?: string };

const ADDRESS = /(https?:\/\/[^\s<>"]+|www\.[^\s<>"]+)/g;
// Punctuation after an address usually ends the sentence, not the address.
const TRAILING = /[.,;:!?)\]}'"。，、；：！？）】」』》]+$/;

/**
 * linkify splits plain text into text and web addresses, so a notice's
 * plain body can link the addresses it mentions. Only http(s) and www.
 * addresses are taken; everything else stays text.
 */
export function linkify(text: string): Piece[] {
  const pieces: Piece[] = [];
  let last = 0;
  for (const match of text.matchAll(ADDRESS)) {
    const raw = match[0];
    const address = raw.replace(TRAILING, "");
    const start = match.index ?? 0;
    if (start > last) pieces.push({ text: text.slice(last, start) });
    pieces.push({ text: address, href: address.startsWith("www.") ? `https://${address}` : address });
    last = start + address.length;
  }
  if (last < text.length) pieces.push({ text: text.slice(last) });
  return pieces;
}
