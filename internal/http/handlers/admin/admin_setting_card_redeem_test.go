package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/provider"
	"github.com/dujiao-next/internal/repository"
	"github.com/dujiao-next/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestUpdateSettingsCardRedeemRules(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&models.Setting{}))
	svc := service.NewSettingService(repository.NewSettingRepository(db))
	h := &Handler{Container: &provider.Container{SettingService: svc}}
	router := gin.New()
	router.PUT("/settings", h.UpdateSettings)
	router.GET("/settings", h.GetSettings)

	save := func(rules interface{}, wantCode int) {
		t.Helper()
		body, err := json.Marshal(map[string]interface{}{
			"key": constants.SettingKeySiteConfig,
			"value": map[string]interface{}{
				"brand":                               map[string]interface{}{"site_name": "Viva"},
				constants.SettingFieldCardRedeemRules: rules,
				constants.SettingFieldCardRedeemURL:   "https://obsolete.example.com/",
			},
		})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodPut, "/settings", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		var result struct {
			StatusCode int `json:"status_code"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
		require.Equal(t, wantCode, result.StatusCode, w.Body.String())
	}
	read := func() []service.CardRedeemRule {
		t.Helper()
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/settings?key=site_config", nil))
		var result struct {
			Data struct {
				Rules []service.CardRedeemRule `json:"card_redeem_rules"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
		var raw struct {
			Data map[string]interface{} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &raw))
		require.NotContains(t, raw.Data, constants.SettingFieldCardRedeemURL)
		return result.Data.Rules
	}

	// Existing storefront providers remain available before the first rules save.
	require.Equal(t, service.DefaultCardRedeemRules(), read())
	_, err = svc.Update(constants.SettingKeySiteConfig, map[string]interface{}{"brand": map[string]interface{}{"site_name": "Viva"}})
	require.NoError(t, err)
	require.Equal(t, service.DefaultCardRedeemRules(), read())
	existingRules := []service.CardRedeemRule{
		{Prefix: "BBL", URL: "https://bblaiplus.com/"},
		{Prefix: "ZERO", URL: "https://zero0ai.com/"},
	}
	_, err = svc.Update(constants.SettingKeySiteConfig, map[string]interface{}{
		constants.SettingFieldCardRedeemURL:   "https://obsolete.example.com/",
		constants.SettingFieldCardRedeemRules: existingRules,
	})
	require.NoError(t, err)
	require.Equal(t, existingRules, read())
	save([]service.CardRedeemRule{{Prefix: " A ", URL: " https://old.example.com/ "}}, 0)
	require.Equal(t, []service.CardRedeemRule{{Prefix: "A", URL: "https://old.example.com/"}}, read())
	save([]service.CardRedeemRule{{Prefix: "A", URL: "https://new.example.com/"}}, 0)
	require.Equal(t, "https://new.example.com/", read()[0].URL)
	// Invalid changes must leave the last saved URL intact.
	save([]service.CardRedeemRule{{Prefix: "A", URL: "javascript:alert(1)"}}, 400)
	require.Equal(t, "https://new.example.com/", read()[0].URL)
	config, err := svc.GetConfig(nil)
	require.NoError(t, err)
	require.Contains(t, config, constants.SettingFieldCardRedeemRules)
	require.NotContains(t, config, constants.SettingFieldCardRedeemURL)
	save([]service.CardRedeemRule{}, 0)
	require.Empty(t, read())
	require.NotNil(t, read())
}
