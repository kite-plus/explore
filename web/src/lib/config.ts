// Read at request time rather than build time, so one image serves any
// deployment. See docs/design/frontend.md section 10.
export function apiURL(): string {
  return (process.env.EXPLORE_API_URL ?? "http://127.0.0.1:8080").replace(/\/+$/, "");
}

export function publicURL(): string {
  return (process.env.EXPLORE_PUBLIC_URL ?? "https://explore.kite.plus").replace(/\/+$/, "");
}
