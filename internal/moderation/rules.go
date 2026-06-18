package moderation

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	urlPattern       = regexp.MustCompile(`(?i)(https?://|www\.|t\.me/|telegram\.me/|instagram\.com|facebook\.com|wa\.me/)`)
	phonePattern     = regexp.MustCompile(`(?i)(\+?996[\s\-()]?\d{3}[\s\-()]?\d{2}[\s\-()]?\d{2}[\s\-()]?\d{2}|\b\d{9,12}\b)`)
	whitespaceRegexp = regexp.MustCompile(`\s+`)
)

var profanityFragments = decodeHexFragments("d0b1d0bbd18f", "d181d183d0bad0b0", "d185d183d0b9", "d0bfd0b8d0b7d0b4", "d0b5d0b1d0b0", "d191d0b1d0b0", "d0bdd0b0d185", "d0bcd180d0b0d0b7", "d181d0b2d0bed0bbd0bed187", "d0b4d0bed0bbd0b1d0be", "6675636b", "73686974", "6269746368", "617373686f6c65")

var abuseFragments = decodeHexFragments("d0b0d0bad0bcd0b0d0ba", "d0bad0b5d0bbd0b5d181d0bed0be", "d0bdd0b0d0b0d0b4d0b0d0bd", "d182d0b0d180d182d0b8d0bfd181d0b8d0b7", "d188d0b0d0bad0b0d0bb")

var adFragments = []string{
	"купите", "продам", "продаю", "скидка", "акция", "дешево", "арзан", "заказ", "доставка",
	"whatsapp", "ватсап", "телеграм канал", "подписывай", "заработок", "кредит",
	"сатам", "сатылат", "сатуу", "баасы", "жеткируу", "жеткирүү", "жазылыныз", "жазылыңыз", "киреше", "номерим",
}

type RuleChecker struct{}

func NewRuleChecker() RuleChecker { return RuleChecker{} }

func (RuleChecker) Check(input Input) Decision {
	text := normalizeText(input.Title + " " + input.Body)
	if text == "" {
		return NewDecision(ActionAllow, "rules")
	}

	reasons := make([]string, 0)
	for _, fragment := range profanityFragments {
		if strings.Contains(text, fragment) {
			reasons = append(reasons, "profanity")
			return NewDecision(ActionBlock, "rules", uniqueReasons(reasons)...)
		}
	}
	for _, fragment := range abuseFragments {
		if strings.Contains(text, fragment) {
			reasons = append(reasons, "abusive_language")
			break
		}
	}

	linkCount := len(urlPattern.FindAllString(text, -1))
	hasPhone := phonePattern.MatchString(text)
	adHits := 0
	for _, fragment := range adFragments {
		if strings.Contains(text, fragment) {
			adHits++
		}
	}

	if linkCount >= 2 {
		reasons = append(reasons, "too_many_links")
	}
	if linkCount > 0 && adHits > 0 {
		reasons = append(reasons, "advertising_link")
	}
	if hasPhone && adHits > 0 {
		reasons = append(reasons, "advertising_contact")
	}
	if hasRepeatedRune(text, 7) {
		reasons = append(reasons, "repeated_characters")
	}
	if excessiveCaps(input.Title + " " + input.Body) {
		reasons = append(reasons, "excessive_caps")
	}

	if len(reasons) > 0 {
		return NewDecision(ActionReview, "rules", uniqueReasons(reasons)...)
	}
	return NewDecision(ActionAllow, "rules")
}

func decodeHexFragments(values ...string) []string {
	fragments := make([]string, 0, len(values))
	for _, value := range values {
		decoded := make([]byte, len(value)/2)
		for i := 0; i < len(decoded); i++ {
			hi := fromHex(value[i*2])
			lo := fromHex(value[i*2+1])
			if hi < 0 || lo < 0 {
				continue
			}
			decoded[i] = byte(hi<<4 | lo)
		}
		if len(decoded) > 0 {
			fragments = append(fragments, string(decoded))
		}
	}
	return fragments
}

func fromHex(value byte) int {
	switch {
	case value >= '0' && value <= '9':
		return int(value - '0')
	case value >= 'a' && value <= 'f':
		return int(value-'a') + 10
	case value >= 'A' && value <= 'F':
		return int(value-'A') + 10
	default:
		return -1
	}
}

func normalizeText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "ё", "е")
	value = strings.ReplaceAll(value, "ү", "у")
	value = strings.ReplaceAll(value, "ң", "н")
	value = strings.ReplaceAll(value, "ө", "о")
	value = whitespaceRegexp.ReplaceAllString(value, " ")
	return value
}

func hasRepeatedRune(value string, limit int) bool {
	if limit <= 1 {
		return false
	}
	var previous rune
	count := 0
	for _, current := range value {
		if current == previous {
			count++
		} else {
			previous = current
			count = 1
		}
		if count >= limit && !unicode.IsSpace(current) {
			return true
		}
	}
	return false
}

func excessiveCaps(value string) bool {
	letters := 0
	upper := 0
	for _, r := range value {
		if !unicode.IsLetter(r) {
			continue
		}
		letters++
		if unicode.IsUpper(r) {
			upper++
		}
	}
	return letters >= 18 && upper*100/letters >= 75
}

func uniqueReasons(reasons []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		if reason == "" || seen[reason] {
			continue
		}
		seen[reason] = true
		result = append(result, reason)
	}
	return result
}
