package domain

import "strings"

// NormalizeMobileInput strips the separators users commonly type in phone
// numbers. Login and the admin allowlist must use the same normalization, or an
// allowlisted number written as "+996 700 ..." would never match a login.
func NormalizeMobileInput(raw string) string {
	mobile := strings.ReplaceAll(strings.TrimSpace(raw), " ", "")
	mobile = strings.ReplaceAll(mobile, "-", "")
	mobile = strings.ReplaceAll(mobile, "(", "")
	mobile = strings.ReplaceAll(mobile, ")", "")
	return mobile
}
