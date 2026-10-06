package service

import (
	"strings"
	"testing"

	"github.com/dujiao-next/internal/constants"
)

func TestNormalizeCardRedeemURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     interface{}
		want    string
		wantErr bool
	}{
		{name: "empty", raw: "  ", want: ""},
		{name: "valid https", raw: " https://gptchongzhi.cc.cd/ ", want: "https://gptchongzhi.cc.cd/"},
		{name: "http is rejected", raw: "http://gptchongzhi.cc.cd/", wantErr: true},
		{name: "relative url is rejected", raw: "/redeem", wantErr: true},
		{name: "non string is rejected", raw: true, wantErr: true},
		{name: "credentials are rejected", raw: "https://user:secret@example.com/", wantErr: true},
		{name: "empty hostname is rejected", raw: "https://:443/", wantErr: true},
		{name: "script URL is rejected", raw: "javascript:alert(1)", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeCardRedeemURL(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NormalizeCardRedeemURL() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("NormalizeCardRedeemURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeCardRedeemRules(t *testing.T) {
	tests := []struct {
		name    string
		raw     interface{}
		wantErr bool
	}{
		{name: "missing"},
		{name: "nil slice", raw: []CardRedeemRule(nil)},
		{name: "empty list", raw: []interface{}{}},
		{name: "overlapping prefixes", raw: []CardRedeemRule{{Prefix: "A", URL: "https://example.com/a"}, {Prefix: "AB", URL: "https://example.com/b"}}},
		{name: "duplicate prefix ignores case and space", raw: []CardRedeemRule{{Prefix: " A ", URL: "https://example.com"}, {Prefix: "a", URL: "https://example.org"}}, wantErr: true},
		{name: "blank prefix", raw: []CardRedeemRule{{Prefix: " ", URL: "https://example.com"}}, wantErr: true},
		{name: "whitespace in prefix", raw: []CardRedeemRule{{Prefix: "A B", URL: "https://example.com"}}, wantErr: true},
		{name: "control in prefix", raw: []CardRedeemRule{{Prefix: "A\x00", URL: "https://example.com"}}, wantErr: true},
		{name: "prefix too long", raw: []CardRedeemRule{{Prefix: strings.Repeat("A", 129), URL: "https://example.com"}}, wantErr: true},
		{name: "empty URL", raw: []CardRedeemRule{{Prefix: "A"}}, wantErr: true},
		{name: "http URL", raw: []CardRedeemRule{{Prefix: "A", URL: "http://example.com"}}, wantErr: true},
		{name: "object instead of list", raw: map[string]interface{}{"prefix": "A", "url": "https://example.com"}, wantErr: true},
		{name: "wrong field type", raw: []interface{}{map[string]interface{}{"prefix": 1, "url": "https://example.com"}}, wantErr: true},
		{name: "null row", raw: []interface{}{nil}, wantErr: true},
		{name: "too many rules", raw: make([]CardRedeemRule, 101), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules, err := NormalizeCardRedeemRules(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NormalizeCardRedeemRules() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && rules == nil {
				t.Fatal("empty rules should be a JSON array, not null")
			}
		})
	}
}

func TestNormalizeSiteConfigCardRedeemRules(t *testing.T) {
	config := map[string]interface{}{
		constants.SettingFieldCardRedeemURL: "invalid retired URL",
		constants.SettingFieldCardRedeemRules: []interface{}{
			map[string]interface{}{"prefix": " A ", "url": " https://example.com/redeem "},
		},
	}
	if err := NormalizeSiteConfig(config); err != nil {
		t.Fatal(err)
	}
	if _, exists := config[constants.SettingFieldCardRedeemURL]; exists {
		t.Fatal("legacy global URL must be removed without changing prefix rules")
	}
	rules := config[constants.SettingFieldCardRedeemRules].([]CardRedeemRule)
	if len(rules) != 1 || rules[0].Prefix != "A" || rules[0].URL != "https://example.com/redeem" {
		t.Fatalf("unexpected normalized rules: %#v", rules)
	}
}

func TestNormalizeSiteConfig(t *testing.T) {
	config := map[string]interface{}{
		constants.SettingFieldCardRedeemURL: " https://gptchongzhi.cc.cd/redeem ",
	}
	if err := NormalizeSiteConfig(config); err != nil {
		t.Fatalf("NormalizeSiteConfig() error = %v", err)
	}
	if _, exists := config[constants.SettingFieldCardRedeemURL]; exists {
		t.Fatal("legacy global URL must be removed")
	}
	if got := config[constants.SettingFieldCardRedeemRules].([]CardRedeemRule); len(got) != 2 || got[0].Prefix != "BBL" || got[1].Prefix != "PLUS" {
		t.Fatalf("missing rules should preserve existing providers: %#v", got)
	}
}
