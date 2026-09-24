import { z } from 'zod'

function isHttpURL(value: string) {
  try {
    const url = new URL(value)
    return url.protocol === 'http:' || url.protocol === 'https:'
  } catch {
    return false
  }
}

/** A full http(s) URL. */
export const urlField = z
  .string()
  .trim()
  .refine(isHttpURL, '请输入完整的网址，以 http:// 或 https:// 开头。')

/** An http(s) URL or nothing. */
export const optionalUrlField = z
  .string()
  .trim()
  .refine((value) => value === '' || isHttpURL(value), '请输入完整的网址，以 http:// 或 https:// 开头。')
