package service

import (
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

func TestNormalizeSiteConfig(t *testing.T) {
	config := map[string]interface{}{
		constants.SettingFieldCardRedeemURL: " https://gptchongzhi.cc.cd/redeem ",
	}
	if err := NormalizeSiteConfig(config); err != nil {
		t.Fatalf("NormalizeSiteConfig() error = %v", err)
	}
	if got := config[constants.SettingFieldCardRedeemURL]; got != "https://gptchongzhi.cc.cd/redeem" {
		t.Fatalf("card_redeem_url = %#v", got)
	}
}
