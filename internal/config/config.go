package config

import (
	"fmt"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port                           string
	DatabaseURL                    string
	JWTSecret                      string
	AccessTokenTTL                 time.Duration
	AllowedOrigins                 string
	BCryptCost                     int
	Environment                    string
	SMSProvider                    string
	SMSProLogin                    string
	SMSProPassword                 string
	SMSProSender                   string
	TwilioAccountSID               string
	TwilioAuthToken                string
	SMSFrom                        string
	TwilioMessagingServiceSID      string
	FCMProjectID                   string
	FCMClientEmail                 string
	FCMPrivateKey                  string
	RunMigrationsOnStart           bool
	SuperAdminPhones               []string
	PlatformAdminPhones            []string
	TestAuthEnabled                bool
	TestAuthPhone                  string
	TestAuthCode                   string
	TestAuthDisplayName            string
	AppReviewPhone                 string
	AppReviewCode                  string
	AppReviewAllowSimpleCode       bool
	SMSDisabledInProduction        bool
	ContentModerationEnabled       bool
	ContentModerationProvider      string
	HuggingFaceModerationToken     string
	HuggingFaceModerationModel     string
	HuggingFaceModerationEndpoint  string
	HuggingFaceModerationThreshold float64
	OpenAIModerationAPIKey         string
	OpenAIModerationModel          string
	OpenAIModerationEndpoint       string
	ModerationFailClosed           bool
	RealtimeBroker                 string
	BackendInstanceCount           int
	TrustedProxyCIDRs              string
	// SMSAllowedCountryPrefixes limits where verification SMS may be sent.
	SMSAllowedCountryPrefixes []string
	// SMSGlobalHourlyMax caps verification SMS sent per hour across all numbers.
	SMSGlobalHourlyMax int
}

func Load() (Config, error) {
	cfg := Config{
		Port:                           getEnv("PORT", "8080"),
		DatabaseURL:                    os.Getenv("DATABASE_URL"),
		JWTSecret:                      os.Getenv("JWT_SECRET"),
		AllowedOrigins:                 getEnv("ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:8080"),
		Environment:                    getEnv("APP_ENV", "production"),
		BCryptCost:                     getEnvInt("BCRYPT_COST", 12),
		SMSProvider:                    getEnv("SMS_PROVIDER", "disabled"),
		SMSProLogin:                    os.Getenv("SMSPRO_LOGIN"),
		SMSProPassword:                 os.Getenv("SMSPRO_PASSWORD"),
		SMSProSender:                   os.Getenv("SMSPRO_SENDER"),
		TwilioAccountSID:               os.Getenv("TWILIO_ACCOUNT_SID"),
		TwilioAuthToken:                os.Getenv("TWILIO_AUTH_TOKEN"),
		SMSFrom:                        os.Getenv("SMS_FROM"),
		TwilioMessagingServiceSID:      os.Getenv("TWILIO_MESSAGING_SERVICE_SID"),
		FCMProjectID:                   os.Getenv("FCM_PROJECT_ID"),
		FCMClientEmail:                 os.Getenv("FCM_CLIENT_EMAIL"),
		FCMPrivateKey:                  os.Getenv("FCM_PRIVATE_KEY"),
		SuperAdminPhones:               appendUniquePhone(getEnvList("SUPER_ADMIN_PHONES"), os.Getenv("PROJECT_OWNER_PHONE")),
		PlatformAdminPhones:            getEnvList("PLATFORM_ADMIN_PHONES"),
		TestAuthEnabled:                getEnvBool("TEST_AUTH_ENABLED", false),
		TestAuthPhone:                  os.Getenv("TEST_AUTH_PHONE"),
		TestAuthCode:                   os.Getenv("TEST_AUTH_CODE"),
		TestAuthDisplayName:            getEnv("TEST_AUTH_DISPLAY_NAME", "Test User"),
		AppReviewPhone:                 strings.TrimSpace(os.Getenv("APP_REVIEW_PHONE")),
		AppReviewCode:                  strings.TrimSpace(os.Getenv("APP_REVIEW_CODE")),
		AppReviewAllowSimpleCode:       getEnvBool("APP_REVIEW_ALLOW_SIMPLE_CODE", false),
		SMSDisabledInProduction:        getEnvBool("SMS_DISABLED_IN_PRODUCTION", false),
		ContentModerationEnabled:       getEnvBool("CONTENT_MODERATION_ENABLED", true),
		ContentModerationProvider:      getEnv("CONTENT_MODERATION_PROVIDER", "huggingface"),
		HuggingFaceModerationToken:     os.Getenv("HF_TOKEN"),
		HuggingFaceModerationModel:     getEnv("HF_MODERATION_MODEL", "unitary/multilingual-toxic-xlm-roberta"),
		HuggingFaceModerationEndpoint:  getEnv("HF_MODERATION_ENDPOINT", "https://router.huggingface.co/hf-inference/models"),
		HuggingFaceModerationThreshold: getEnvFloat("HF_MODERATION_THRESHOLD", 0.72),
		OpenAIModerationAPIKey:         os.Getenv("OPENAI_API_KEY"),
		OpenAIModerationModel:          getEnv("OPENAI_MODERATION_MODEL", "omni-moderation-latest"),
		OpenAIModerationEndpoint:       getEnv("OPENAI_MODERATION_ENDPOINT", "https://api.openai.com/v1/moderations"),
		ModerationFailClosed:           getEnvBool("MODERATION_FAIL_CLOSED", true),
		RealtimeBroker:                 getEnv("REALTIME_BROKER", "local"),
		BackendInstanceCount:           getEnvInt("BACKEND_INSTANCE_COUNT", 1),
		TrustedProxyCIDRs:              os.Getenv("TRUSTED_PROXY_CIDRS"),
	}

	if !knownEnvironment(cfg.Environment) {
		return Config{}, fmt.Errorf("APP_ENV=%q is not recognized; use one of: %s", cfg.Environment, strings.Join(knownEnvironments, ", "))
	}

	cfg.RunMigrationsOnStart = getEnvBool("RUN_MIGRATIONS_ON_START", !sharedEnvironment(cfg.Environment))

	prefixes, err := parseSMSCountryPrefixes(getEnv("SMS_ALLOWED_COUNTRY_PREFIXES", "+996"))
	if err != nil {
		return Config{}, err
	}
	cfg.SMSAllowedCountryPrefixes = prefixes
	hourlyMax, err := getEnvPositiveInt("SMS_GLOBAL_HOURLY_MAX", 500)
	if err != nil {
		return Config{}, err
	}
	cfg.SMSGlobalHourlyMax = hourlyMax

	accessTokenTTLMinutes := getEnvInt("ACCESS_TOKEN_TTL_MINUTES", 5)
	cfg.AccessTokenTTL = time.Duration(accessTokenTTLMinutes) * time.Minute

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" || len(cfg.JWTSecret) < 32 {
		return Config{}, fmt.Errorf("JWT_SECRET is required and must be at least 32 characters")
	}
	if sharedEnvironment(cfg.Environment) && looksLikePlaceholderSecret(cfg.JWTSecret) {
		return Config{}, fmt.Errorf("JWT_SECRET looks like a placeholder value; generate a random secret for staging or production")
	}
	if cfg.BCryptCost < 10 || cfg.BCryptCost > 15 {
		return Config{}, fmt.Errorf("BCRYPT_COST must be between 10 and 15")
	}
	if sharedEnvironment(cfg.Environment) && originListContainsWildcard(cfg.AllowedOrigins) {
		return Config{}, fmt.Errorf("wildcard CORS origin is not allowed in staging or production")
	}
	if sharedEnvironment(cfg.Environment) && !validTrustedProxyCIDRs(cfg.TrustedProxyCIDRs) {
		return Config{}, fmt.Errorf("TRUSTED_PROXY_CIDRS must contain specific proxy IPs or CIDRs no broader than IPv4 /16 or IPv6 /32 in staging or production")
	}
	if cfg.TestAuthEnabled {
		if !localTestAuthEnvironment(cfg.Environment) {
			return Config{}, fmt.Errorf("TEST_AUTH_ENABLED=true is allowed only in local, development, or test environments")
		}
		if strings.TrimSpace(cfg.TestAuthPhone) == "" || strings.TrimSpace(cfg.TestAuthCode) == "" {
			return Config{}, fmt.Errorf("TEST_AUTH_PHONE and TEST_AUTH_CODE are required when test auth is enabled")
		}
	}

	if err := validateAppReviewLogin(cfg); err != nil {
		return Config{}, err
	}

	if sharedEnvironment(cfg.Environment) {
		for _, phone := range append(append([]string{}, cfg.SuperAdminPhones...), cfg.PlatformAdminPhones...) {
			if normalizePhoneForValidation(phone) == "+996000000000" {
				return Config{}, fmt.Errorf("placeholder admin phone +996000000000 is not allowed in staging or production")
			}
		}
	}

	if sharedEnvironment(cfg.Environment) && !cfg.ModerationFailClosed {
		return Config{}, fmt.Errorf("MODERATION_FAIL_CLOSED=false is not allowed in staging or production")
	}
	if sharedEnvironment(cfg.Environment) && cfg.BackendInstanceCount > 1 {
		if cfg.RealtimeBroker == "local" {
			return Config{}, fmt.Errorf("REALTIME_BROKER=local is not allowed with multiple backend instances in staging or production")
		}
		return Config{}, fmt.Errorf("BACKEND_INSTANCE_COUNT greater than 1 requires a shared Redis rate limiter in staging or production; the current rate limiter is in-memory")
	}
	if sharedEnvironment(cfg.Environment) && cfg.AccessTokenTTL > 15*time.Minute {
		return Config{}, fmt.Errorf("ACCESS_TOKEN_TTL_MINUTES must be 15 or less in staging or production")
	}
	return cfg, nil
}

var appReviewPhonePattern = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)

// AppReviewPhones splits APP_REVIEW_PHONE, which may list several
// comma-separated test accounts that all share APP_REVIEW_CODE.
func AppReviewPhones(raw string) []string {
	var phones []string
	for _, part := range strings.Split(raw, ",") {
		if phone := normalizePhoneForValidation(part); phone != "" {
			phones = append(phones, phone)
		}
	}
	return phones
}

// validateAppReviewLogin checks the store-review / test account settings. The
// accounts must be dedicated numbers (never admins) with a 6-digit code. The
// code must be non-trivial unless APP_REVIEW_ALLOW_SIMPLE_CODE=true.
func validateAppReviewLogin(cfg Config) error {
	phones := AppReviewPhones(cfg.AppReviewPhone)
	code := cfg.AppReviewCode
	if len(phones) == 0 && code == "" {
		return nil
	}
	if len(phones) == 0 || code == "" {
		return fmt.Errorf("APP_REVIEW_PHONE and APP_REVIEW_CODE must be set together")
	}
	if len(phones) > 20 {
		return fmt.Errorf("APP_REVIEW_PHONE allows at most 20 numbers")
	}
	if cfg.AppReviewAllowSimpleCode {
		if len(code) != 6 || strings.Trim(code, "0123456789") != "" {
			return fmt.Errorf("APP_REVIEW_CODE must be 6 digits")
		}
	} else if weakAppReviewCode(code) {
		return fmt.Errorf("APP_REVIEW_CODE must be 6 digits and not trivial (no repeated digits or sequences); set APP_REVIEW_ALLOW_SIMPLE_CODE=true to allow a simple test code")
	}
	seen := map[string]bool{}
	for _, phone := range phones {
		if !appReviewPhonePattern.MatchString(phone) {
			return fmt.Errorf("APP_REVIEW_PHONE must be in international format, for example +996312000000")
		}
		if seen[phone] {
			return fmt.Errorf("APP_REVIEW_PHONE lists %s twice", phone)
		}
		seen[phone] = true
		for _, admin := range append(append([]string{}, cfg.SuperAdminPhones...), cfg.PlatformAdminPhones...) {
			if normalizePhoneForValidation(admin) == phone {
				return fmt.Errorf("APP_REVIEW_PHONE must not be an admin phone")
			}
		}
		if cfg.TestAuthEnabled {
			for _, part := range strings.Split(cfg.TestAuthPhone, ",") {
				if normalizePhoneForValidation(part) == phone {
					return fmt.Errorf("APP_REVIEW_PHONE must not also be a TEST_AUTH_PHONE")
				}
			}
		}
	}
	return nil
}

func weakAppReviewCode(code string) bool {
	if len(code) != 6 {
		return true
	}
	distinct := map[rune]bool{}
	for _, char := range code {
		if char < '0' || char > '9' {
			return true
		}
		distinct[char] = true
	}
	if len(distinct) < 3 {
		return true
	}
	ascending, descending := true, true
	for i := 1; i < len(code); i++ {
		if code[i] != code[i-1]+1 {
			ascending = false
		}
		if code[i] != code[i-1]-1 {
			descending = false
		}
	}
	return ascending || descending
}

func normalizePhoneForValidation(phone string) string {
	phone = strings.TrimSpace(phone)
	phone = strings.ReplaceAll(phone, " ", "")
	phone = strings.ReplaceAll(phone, "-", "")
	phone = strings.ReplaceAll(phone, "(", "")
	phone = strings.ReplaceAll(phone, ")", "")
	return phone
}

func appendUniquePhone(phones []string, phone string) []string {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return phones
	}
	for _, existing := range phones {
		if strings.TrimSpace(existing) == phone {
			return phones
		}
	}
	return append(phones, phone)
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getEnvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvFloat(key string, fallback float64) float64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvBool(key string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	switch value {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return fallback
	}
}

func getEnvList(key string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		values = append(values, value)
	}
	return values
}

func originListContainsWildcard(raw string) bool {
	for _, part := range strings.Split(raw, ",") {
		if strings.TrimSpace(part) == "*" {
			return true
		}
	}
	return false
}

func localTestAuthEnvironment(environment string) bool {
	switch strings.ToLower(strings.TrimSpace(environment)) {
	case "local", "dev", "development", "test", "testing":
		return true
	default:
		return false
	}
}

// knownEnvironments lists every APP_ENV value the server accepts. A typo such
// as "prod" must fail startup instead of silently getting non-shared
// (development) defaults.
var knownEnvironments = []string{"local", "dev", "development", "test", "testing", "staging", "production"}

func knownEnvironment(environment string) bool {
	environment = strings.ToLower(strings.TrimSpace(environment))
	for _, known := range knownEnvironments {
		if environment == known {
			return true
		}
	}
	return false
}

var placeholderSecretMarkers = []string{"change", "replace", "example", "placeholder", "secret-here", "secret_here", "your-secret", "your_secret", "dummy"}

func looksLikePlaceholderSecret(secret string) bool {
	lower := strings.ToLower(secret)
	for _, marker := range placeholderSecretMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func parseSMSCountryPrefixes(raw string) ([]string, error) {
	prefixes := make([]string, 0)
	seen := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		prefix := strings.TrimSpace(part)
		if prefix == "" {
			continue
		}
		if !strings.HasPrefix(prefix, "+") || len(prefix) < 2 || len(prefix) > 5 {
			return nil, fmt.Errorf("SMS_ALLOWED_COUNTRY_PREFIXES entries must look like +996, got %q", prefix)
		}
		for _, char := range prefix[1:] {
			if char < '0' || char > '9' {
				return nil, fmt.Errorf("SMS_ALLOWED_COUNTRY_PREFIXES entries must look like +996, got %q", prefix)
			}
		}
		if !seen[prefix] {
			seen[prefix] = true
			prefixes = append(prefixes, prefix)
		}
	}
	if len(prefixes) == 0 {
		return nil, fmt.Errorf("SMS_ALLOWED_COUNTRY_PREFIXES must list at least one country prefix")
	}
	return prefixes, nil
}

func getEnvPositiveInt(key string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}
	return parsed, nil
}

func sharedEnvironment(environment string) bool {
	switch strings.ToLower(strings.TrimSpace(environment)) {
	case "staging", "production":
		return true
	default:
		return false
	}
}

func validTrustedProxyCIDRs(raw string) bool {
	found := false
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if net.ParseIP(item) != nil {
			found = true
			continue
		}
		_, network, err := net.ParseCIDR(item)
		if err != nil {
			return false
		}
		prefix, bits := network.Mask.Size()
		minimumPrefix := 16
		if bits == 128 {
			minimumPrefix = 32
		}
		if prefix < minimumPrefix {
			return false
		}
		found = true
	}
	return found
}
