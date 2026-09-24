package normalize

import (
	"strings"

	"golang.org/x/text/language"
)

// Language returns a language tag in canonical BCP 47 form, such as "zh-CN"
// for "zh_cn" or "he" for the deprecated "iw", and "und" when the tag is
// empty or does not parse.
func Language(tag string) string {
	t, err := language.Parse(strings.TrimSpace(tag))
	if err != nil {
		return "und"
	}
	return t.String()
}
