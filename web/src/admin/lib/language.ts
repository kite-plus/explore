const names = new Intl.DisplayNames(['zh-CN'], { type: 'language' })

/**
 * Groups the language tags feeds declare: Chinese by script, so zh, zh-CN,
 * zh_CN and zh-Hans are all zh-Hans, and other languages by their primary
 * subtag. Empty tags count as "und".
 */
export function languageKey(tag: string) {
  const trimmed = tag.trim().replace(/_/g, '-')
  if (!trimmed || trimmed.toLowerCase() === 'und') return 'und'
  try {
    const locale = new Intl.Locale(trimmed).maximize()
    return locale.language === 'zh' ? `zh-${locale.script}` : locale.language
  } catch {
    return trimmed.toLowerCase()
  }
}

/** The Chinese display name of a languageKey group. */
export function languageLabel(key: string) {
  if (key === 'und') return '未标注'
  try {
    return names.of(key) ?? key
  } catch {
    return key
  }
}
