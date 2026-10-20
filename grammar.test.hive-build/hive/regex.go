package hive

import "regexp"

func CompilePattern(pattern string) *regexp.Regexp {
	return regexp.MustCompile(pattern)
}

func MatchPattern(s string, re *regexp.Regexp) ([]string, bool) {
	found := re.FindStringSubmatch(s)
	if found == nil {
		return nil, false
	}
	return found[1:], true
}
