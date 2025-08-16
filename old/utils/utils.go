package utils

import (
	"regexp"
	"strings"
)

func ExtractEmailAddress(s string) string {
	re := regexp.MustCompile(`<([^>]+)>`)
	if match := re.FindStringSubmatch(s); len(match) == 2 {
		return strings.ToLower(strings.TrimSpace(match[1]))
	}
	return strings.ToLower(strings.TrimSpace(s))
}