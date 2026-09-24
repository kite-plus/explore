// UI preferences live in localStorage: the site sets no cookies for them.
// Storage can be blocked or unavailable, so every access is guarded.

export function readPreference(key: string): string | undefined {
  try {
    return localStorage.getItem(key) ?? undefined
  } catch {
    return undefined
  }
}

export function writePreference(key: string, value: string | undefined) {
  try {
    if (value === undefined) localStorage.removeItem(key)
    else localStorage.setItem(key, value)
  } catch {
    // The preference then lasts for this page only.
  }
}
