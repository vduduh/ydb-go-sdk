package bind

import (
	"regexp"
	"strings"
)

var (
	// regexps for detect one of type of query
	numericArgsRe    = regexp.MustCompile(`\$\d+`)
	positionalArgsRe = regexp.MustCompile(`[^\\][?]`)
	ydbArgsRe        = regexp.MustCompile(`\$[a-zA-Z\_]+\w+`)
)

type argsType int

const (
	undefinedArgs = argsType(1 << iota >> 1)
	numericArgs
	positionalArgs
	ydbArgs
)

func filterCommentLines(q string) string {
	lines := strings.Split(q, "\n")
	for i := range lines {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "--") {
			lines[i] = ""
		}
	}
	return strings.Join(lines, "\n")
}

func queryType(q string) (t argsType) {
	q = filterCommentLines(q)
	if numericArgsRe.MatchString(q) {
		t |= numericArgs
	}
	if positionalArgsRe.MatchString(q) {
		t |= positionalArgs
	}
	if ydbArgsRe.MatchString(q) {
		t |= ydbArgs
	}
	return t
}
