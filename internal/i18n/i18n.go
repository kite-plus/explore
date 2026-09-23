// Package i18n picks the language for text shown to people. Explore speaks
// Simplified Chinese and English; machine-readable codes never change.
package i18n

import "golang.org/x/text/language"

// Lang is a supported language tag.
type Lang string

const (
	English Lang = "en"
	Chinese Lang = "zh-CN"
)

// supported lists English first so it wins when nothing matches.
var supported = []language.Tag{language.English, language.SimplifiedChinese}

var matcher = language.NewMatcher(supported)

// Match returns the supported language that best fits an Accept-Language
// header, falling back to English.
func Match(acceptLanguage string) Lang {
	tags, _, err := language.ParseAcceptLanguage(acceptLanguage)
	if err != nil || len(tags) == 0 {
		return English
	}
	_, index, confidence := matcher.Match(tags...)
	if confidence == language.No {
		return English
	}
	if supported[index] == language.SimplifiedChinese {
		return Chinese
	}
	return English
}

// Parse accepts a language flag such as "zh-CN", "zh" or "en".
func Parse(s string) Lang {
	return Match(s)
}
