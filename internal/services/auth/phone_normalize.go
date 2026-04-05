package auth

import "strings"

func NormalizePhoneKey(s string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(s) {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	d := b.String()
	if len(d) == 11 && d[0] == '8' {
		d = "7" + d[1:]
	}
	if len(d) == 10 && d[0] == '9' {
		d = "7" + d
	}
	return d
}
