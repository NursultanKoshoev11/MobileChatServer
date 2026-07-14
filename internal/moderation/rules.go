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

var prohibitedKeywordFragments = decodeHexFragments(
	"d0bfd180d0bed0b4d0b0d0bc",
	"d0bfd180d0bed0b4d0b0d18e",
	"d0bfd180d0bed0b4d0b0d0b5d182d181d18f",
	"d0bfd180d0bed0b4d0b0d191d182d181d18f",
	"d181d0b0d182d0b0d0bc",
	"d181d0b0d182d18bd0bbd0b0d182",
	"d181d0b0d182d183d183",
	"31382b",
	"d0bfd0bed180d0bdd0be",
	"d181d0b5d0bad181",
	"d0b8d0bdd182d0b8d0bc",
	"d18dd180d0bed182d0b8d0ba",
	"d0bfd180d0bed181d182d0b8d182",
	"d0bad0b0d0b7d0b8d0bdd0be",
	"d181d182d0b0d0b2d0bad0b8",
	"d0b1d183d0bad0bcd0b5d0bad0b5d180",
	"d0bdd0b0d180d0bad0bed182",
	"d0bcd0b0d180d0b8d185d183d0b0d0bd",
)

var profanityFragments = decodeHexFragments("d0b1d0bbd18f", "d181d183d0bad0b0", "d185d183d0b9", "d0bfd0b8d0b7d0b4", "d0b5d0b1d0b0", "d191d0b1d0b0", "d0bdd0b0d185", "d0bcd180d0b0d0b7", "d181d0b2d0bed0bbd0bed187", "d0b4d0bed0bbd0b1d0be", "6675636b", "73686974", "6269746368", "617373686f6c65")

var abuseFragments = decodeHexFragments("d0b0d0bad0bcd0b0d0ba", "d0bad0b5d0bbd0b5d181d0bed0be", "d0bdd0b0d0b0d0b4d0b0d0bd", "d182d0b0d180d182d0b8d0bfd181d0b8d0b7", "d188d0b0d0bad0b0d0bb")

var hardAdFragments = []string{
	"продается", "продаётся", "продам", "продаю", "сатылат", "сатам", "сатуу",
	"баасы", "доставка", "заказ", "скидка", "акция", "купите", "арзан",
}

var adFragments = append(decodeHexFragments(
	"d0bad183d0bfd0b8d182d0b5",                               // buy
	"d0bad183d0bfd0bbd18e",                                   // buy / want to buy
	"d0bfd180d0bed0b4d0b0d0bc",                               // sell
	"d0bfd180d0bed0b4d0b0d18e",                               // selling
	"d0bfd180d0bed0b4d0b0d0b5d182d181d18f",                   // for sale
	"d0bfd180d0bed0b4d0b0d191d182d181d18f",                   // for sale with yo
	"d181d0bad0b8d0b4d0bad0b0",                               // discount
	"d0b0d0bad186d0b8d18f",                                   // promo
	"d0b4d0b5d188d0b5d0b2d0be",                               // cheap
	"d0b4d191d188d0b5d0b2d0be",                               // cheap with yo
	"d0b0d180d0b7d0b0d0bd",                                   // cheap KG/RU
	"d0b7d0b0d0bad0b0d0b7",                                   // order
	"d0b4d0bed181d182d0b0d0b2d0bad0b0",                       // delivery
	"d0b2d0b0d182d181d0b0d0bf",                               // whatsapp Cyrillic
	"d182d0b5d0bbd0b5d0b3d180d0b0d0bc20d0bad0b0d0bdd0b0d0bb", // telegram channel
	"d0bfd0bed0b4d0bfd0b8d181d18bd0b2d0b0d0b9",               // subscribe
	"d0b7d0b0d180d0b0d0b1d0bed182d0bed0ba",                   // income
	"d0bad180d0b5d0b4d0b8d182",                               // credit
	"d181d0b0d182d0b0d0bc",                                   // sell KG
	"d181d0b0d182d18bd0bbd0b0d182",                           // for sale KG
	"d181d0b0d182d183d183",                                   // sale KG
	"d0b1d0b0d0b0d181d18b",                                   // price KG
	"d0b6d0b5d182d0bad0b8d180d2afd2af",                       // delivery KG
	"d0b6d0b5d182d0bad0b8d180d183d183",                       // delivery KG alt
	"d0b6d0b0d0b7d18bd0bbd18bd2a3d18bd0b7",                   // subscribe KG
	"d0b6d0b0d0b7d18bd0bbd18bd0bdd18bd0b7",                   // subscribe KG alt
	"d0bdd0bed0bcd0b5d180d0b8",                               // number
	"d0bdd0bed0bcd0b5d180d0b8d0bc",                           // my number
), "whatsapp", "telegram", "instagram", "facebook", "wa.me", "t.me")

type RuleChecker struct{}

func NewRuleChecker() RuleChecker { return RuleChecker{} }

func (RuleChecker) Check(input Input) Decision {
	text := normalizeText(input.Title + " " + input.Body)
	if text == "" {
		return NewDecision(ActionAllow, "rules")
	}

	reasons := make([]string, 0)
	for _, fragment := range prohibitedKeywordFragments {
		if strings.Contains(text, fragment) {
			reasons = append(reasons, "prohibited_keyword")
			return NewDecision(ActionBlock, "rules", uniqueReasons(reasons)...)
		}
	}
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
	hardAdHits := 0
	for _, fragment := range hardAdFragments {
		if strings.Contains(text, fragment) {
			hardAdHits++
		}
	}

	if linkCount >= 2 {
		reasons = append(reasons, "too_many_links")
	}
	if linkCount > 0 {
		reasons = append(reasons, "advertising_link")
	}
	if hasPhone && adHits > 0 {
		reasons = append(reasons, "advertising_contact")
	}
	if adHits > 0 || hardAdHits > 0 {
		reasons = append(reasons, "advertising_text")
	}
	if hardAdHits > 0 || (adHits > 0 && (linkCount > 0 || hasPhone)) {
		return NewDecision(ActionBlock, "rules", uniqueReasons(reasons)...)
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
	value = strings.ReplaceAll(value, "\u0451", "\u0435")
	value = strings.ReplaceAll(value, "\u04af", "\u0443")
	value = strings.ReplaceAll(value, "\u04a3", "\u043d")
	value = strings.ReplaceAll(value, "\u04e9", "\u043e")
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
