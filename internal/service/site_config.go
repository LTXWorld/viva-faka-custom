package service

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"unicode"

	"github.com/dujiao-next/internal/constants"
)

const cardRedeemURLMaxLength = 500

const cardRedeemRulesMaxCount = 100
const cardRedeemPrefixMaxLength = 128

// CardRedeemRule maps a case-insensitive card prefix to a redemption website.
type CardRedeemRule struct {
	Prefix string `json:"prefix"`
	URL    string `json:"url"`
}

// DefaultCardRedeemRules preserves the storefront's existing providers until
// an administrator saves an explicit rule list (including an empty list).
func DefaultCardRedeemRules() []CardRedeemRule {
	return []CardRedeemRule{
		{Prefix: "BBL", URL: "https://bblaiplus.com"},
		{Prefix: "PLUS", URL: "https://gptchongzhi.cc.cd/"},
	}
}

// NormalizeCardRedeemRules validates administrator input before publishing rules.
func NormalizeCardRedeemRules(raw interface{}) ([]CardRedeemRule, error) {
	rules := make([]CardRedeemRule, 0)
	if raw == nil {
		return rules, nil
	}
	data, err := json.Marshal(raw)
	if err != nil || json.Unmarshal(data, &rules) != nil {
		return nil, fmt.Errorf("卡密兑换规则必须是前缀和网址组成的列表")
	}
	if rules == nil {
		rules = make([]CardRedeemRule, 0)
	}
	if len(rules) > cardRedeemRulesMaxCount {
		return nil, fmt.Errorf("卡密兑换规则最多 %d 条", cardRedeemRulesMaxCount)
	}
	seen := make(map[string]bool)
	for i := range rules {
		rule := &rules[i]
		rule.Prefix = strings.TrimSpace(rule.Prefix)
		if rule.Prefix == "" || len(rule.Prefix) > cardRedeemPrefixMaxLength || strings.ContainsFunc(rule.Prefix, func(r rune) bool {
			return unicode.IsSpace(r) || unicode.IsControl(r)
		}) {
			return nil, fmt.Errorf("第 %d 条规则的卡密前缀必须为 1-%d 字节，且不能包含空白或控制字符", i+1, cardRedeemPrefixMaxLength)
		}
		key := strings.ToUpper(rule.Prefix)
		if seen[key] {
			return nil, fmt.Errorf("第 %d 条规则的卡密前缀重复（不区分大小写）", i+1)
		}
		seen[key] = true
		rule.URL, err = NormalizeCardRedeemURL(rule.URL)
		if err != nil || rule.URL == "" {
			return nil, fmt.Errorf("第 %d 条规则必须填写 HTTPS 完整兑换网址", i+1)
		}
	}
	return rules, nil
}

// NormalizeCardRedeemURL validates the optional global card-redeem website URL.
// Only absolute HTTPS URLs are published to the storefront navigation.
func NormalizeCardRedeemURL(raw interface{}) (string, error) {
	if raw == nil {
		return "", nil
	}

	value, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("卡密兑换网站地址必须是 HTTPS 完整链接")
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if len(value) > cardRedeemURLMaxLength {
		return "", fmt.Errorf("卡密兑换网站地址不能超过 %d 个字符", cardRedeemURLMaxLength)
	}

	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil {
		return "", fmt.Errorf("卡密兑换网站地址必须是 HTTPS 完整链接")
	}
	return parsed.String(), nil
}

// NormalizeSiteConfig mutates site-config values into their persisted, public-safe form.
func NormalizeSiteConfig(value map[string]interface{}) error {
	if value == nil {
		return nil
	}

	cardRedeemURL, err := NormalizeCardRedeemURL(value[constants.SettingFieldCardRedeemURL])
	if err != nil {
		return err
	}
	value[constants.SettingFieldCardRedeemURL] = cardRedeemURL
	rawRules, exists := value[constants.SettingFieldCardRedeemRules]
	if !exists {
		rawRules = DefaultCardRedeemRules()
	}
	rules, err := NormalizeCardRedeemRules(rawRules)
	if err != nil {
		return err
	}
	value[constants.SettingFieldCardRedeemRules] = rules
	return nil
}
