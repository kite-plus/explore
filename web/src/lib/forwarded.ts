// What this server sends the API in X-Forwarded-For when it calls on a
// reader's behalf, so rate limits count readers rather than this server:
// the chain the proxy in front sent, then the address of the connection
// itself. Astro reads X-Forwarded-For only with security.allowedDomains,
// which stays unset, so clientAddress is that connection's address. The API
// takes the right-most address that is not a trusted proxy, so whatever a
// reader writes in the header sits left of its real address, never reached.
// See docs/design/frontend.md section 6.
export function forwardedFor(request: Request, clientAddress: string | undefined): string | undefined {
  return [request.headers.get("x-forwarded-for"), clientAddress].filter(Boolean).join(", ") || undefined;
}

/** The same, as headers for a fetch to the API. */
export function forwardedHeaders(request: Request, clientAddress: string | undefined): Record<string, string> {
  const chain = forwardedFor(request, clientAddress);
  return chain ? { "X-Forwarded-For": chain } : {};
}
