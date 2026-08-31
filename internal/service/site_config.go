package service

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/dujiao-next/internal/constants"
)

const cardRedeemURLMaxLength = 500

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
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
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
	return nil
}
