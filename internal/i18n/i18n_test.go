package i18n

import "testing"

func TestMatch(t *testing.T) {
	cases := map[string]Lang{
		"":                                English,
		"en":                              English,
		"en-US,en;q=0.9":                  English,
		"zh-CN":                           Chinese,
		"zh":                              Chinese,
		"zh-Hans":                         Chinese,
		"zh-CN,zh;q=0.9,en;q=0.8":         Chinese,
		"en-GB,en;q=0.9,zh-CN;q=0.1":      English,
		"fr-FR,fr;q=0.9":                  English,
		"de":                              English,
		"not a language header ;;; q=bad": English,
	}
	for in, want := range cases {
		if got := Match(in); got != want {
			t.Errorf("Match(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestParse(t *testing.T) {
	if Parse("zh-CN") != Chinese || Parse("en") != English || Parse("ZH") != Chinese {
		t.Error("Parse does not accept the documented flag values")
	}
}
