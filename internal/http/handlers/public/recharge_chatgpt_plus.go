package public

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/http/response"
	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/models"

	"gorm.io/gorm"

	"github.com/gin-gonic/gin"
)

const (
	lyxazyDefaultBaseURL = "https://www.lyxazy.top/verify"
	lyxazyHTTPTimeout    = 30 * time.Second
)

type chatGPTPlusSubmitRequest struct {
	CardKey     string      `json:"card_key"`
	AccessToken string      `json:"access_token"`
	SessionData interface{} `json:"session_data"`
}

type lyxazyErrorResponse struct {
	Error     string `json:"error"`
	ErrorCode string `json:"error_code"`
	Message   string `json:"message"`
	Msg       string `json:"msg"`
	Detail    string `json:"detail"`
}

// SubmitChatGPTPlusRecharge POST /api/v1/public/recharge/chatgpt-plus/submit
// 代理提交 ChatGPT Plus 官方直充任务。注意：LYXAZY API Key 只在服务端配置，不能暴露到前端。
func (h *Handler) SubmitChatGPTPlusRecharge(c *gin.Context) {
	apiKey := h.lyxazyAPIKey()
	if apiKey == "" {
		response.Error(c, response.CodeInternal, "LYXAZY API Key 未配置")
		return
	}

	var req chatGPTPlusSubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, response.CodeBadRequest, "请求参数格式错误")
		return
	}

	cardKey := strings.TrimSpace(req.CardKey)
	accessToken := strings.TrimSpace(req.AccessToken)
	if cardKey == "" {
		response.Error(c, response.CodeBadRequest, "请输入卡密")
		return
	}
	if err := h.ensureRechargeCardOwned(cardKey); err != nil {
		response.Error(c, response.CodeForbidden, err.Error())
		return
	}
	if existing := h.getExistingRechargeRecord(cardKey); existing != nil && strings.TrimSpace(existing.UpstreamJobID) != "" {
		response.Success(c, rechargeJobToPublicMap(existing))
		return
	}

	sessionData := req.SessionData
	if sessionData == nil {
		if accessToken == "" {
			response.Error(c, response.CodeBadRequest, "请输入 ChatGPT AccessToken")
			return
		}
		sessionData = accessToken
	}

	payload := map[string]interface{}{
		"card_key":     cardKey,
		"product_line": "plus",
		"session_data": sessionData,
	}

	data, statusCode, err := h.lyxazyJSON(c, http.MethodPost, "/api/plus/jobs", payload)
	if err != nil {
		response.Error(c, statusCode, err.Error())
		return
	}

	h.saveChatGPTPlusRechargeRecord(cardKey, data, true)

	response.Success(c, data)
}

// QueryChatGPTPlusRechargeByCardKey GET /api/v1/public/recharge/chatgpt-plus/by-card-key?card_key=xxx
// 按卡密查询本 API Key 下的 ChatGPT Plus 直充历史任务。
func (h *Handler) QueryChatGPTPlusRechargeByCardKey(c *gin.Context) {
	apiKey := h.lyxazyAPIKey()
	if apiKey == "" {
		response.Error(c, response.CodeInternal, "LYXAZY API Key 未配置")
		return
	}

	cardKey := strings.TrimSpace(c.Query("card_key"))
	if cardKey == "" {
		response.Error(c, response.CodeBadRequest, "请输入卡密")
		return
	}
	if err := h.ensureRechargeCardOwned(cardKey); err != nil {
		response.Error(c, response.CodeForbidden, err.Error())
		return
	}

	path := "/api/plus/jobs/by-card-key?card_key=" + url.QueryEscape(cardKey)
	data, statusCode, err := h.lyxazyJSON(c, http.MethodGet, path, nil)
	if err != nil {
		response.Error(c, statusCode, err.Error())
		return
	}

	h.saveChatGPTPlusRechargeRecordsFromQuery(cardKey, data)

	response.Success(c, data)
}

func (h *Handler) ensureRechargeCardOwned(cardKey string) error {
	if h == nil || h.CardSecretRepo == nil {
		return fmt.Errorf("卡密校验服务不可用")
	}
	_, order, err := h.CardSecretRepo.FindSoldBySecret(cardKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("卡密不存在或尚未售出，请确认你输入的是在 Viva 小铺已付款订单中的卡密")
		}
		logger.Warnw("recharge_card_ownership_check_failed", "error", err)
		return fmt.Errorf("卡密校验失败，请稍后重试")
	}
	if order == nil || !isPaidRechargeOrderStatus(order.Status) {
		return fmt.Errorf("卡密关联订单未完成付款，暂不能兑换")
	}
	return nil
}

func isPaidRechargeOrderStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case constants.OrderStatusPaid, constants.OrderStatusFulfilling, constants.OrderStatusPartiallyDelivered, constants.OrderStatusDelivered, constants.OrderStatusCompleted:
		return true
	default:
		return false
	}
}

func (h *Handler) getExistingRechargeRecord(cardKey string) *models.RechargeJob {
	if h == nil || h.RechargeJobRepo == nil {
		return nil
	}
	job, err := h.RechargeJobRepo.GetByCardKey(cardKey)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Warnw("recharge_job_lookup_failed", "error", err)
		}
		return nil
	}
	return job
}

func rechargeJobToPublicMap(job *models.RechargeJob) map[string]interface{} {
	if job == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"job_id":           job.UpstreamJobID,
		"status":           job.Status,
		"message":          job.Message,
		"card_key":         job.CardKey,
		"product_line":     "plus",
		"activation_email": job.ActivationEmail,
		"plan_name":        job.PlanName,
		"duplicate":        true,
		"local_record":     true,
		"created_at":       job.CreatedAt.UnixMilli(),
		"updated_at":       job.UpdatedAt.UnixMilli(),
	}
}

func (h *Handler) saveChatGPTPlusRechargeRecordsFromQuery(cardKey string, data map[string]interface{}) {
	records, ok := data["records"].([]interface{})
	if !ok || len(records) == 0 {
		return
	}
	for _, item := range records {
		if record, ok := item.(map[string]interface{}); ok {
			if strings.TrimSpace(asString(record["card_key"])) == "" {
				record["card_key"] = cardKey
			}
			h.saveChatGPTPlusRechargeRecord(cardKey, record, false)
		}
	}
}

func (h *Handler) saveChatGPTPlusRechargeRecord(cardKey string, data map[string]interface{}, submitted bool) {
	if h == nil || h.RechargeJobRepo == nil || data == nil {
		return
	}
	cardKey = strings.TrimSpace(cardKey)
	if cardKey == "" {
		cardKey = strings.TrimSpace(asString(data["card_key"]))
	}
	if cardKey == "" {
		return
	}

	status := strings.TrimSpace(asString(data["status"]))
	if status == "" {
		status = models.RechargeStatusSubmitted
	}
	now := time.Now().UTC()
	var submittedAt *time.Time
	if submitted {
		submittedAt = &now
	}
	var finishedAt *time.Time
	if isRechargeTerminalStatus(status) {
		finishedAt = &now
	}

	job := &models.RechargeJob{
		Provider:        models.RechargeProviderLyxazy,
		ProductType:     models.RechargeProductChatGPTPlus,
		CardKey:         cardKey,
		UpstreamJobID:   strings.TrimSpace(asString(data["job_id"])),
		Status:          status,
		Message:         strings.TrimSpace(asString(data["message"])),
		ActivationEmail: strings.TrimSpace(asString(data["activation_email"])),
		PlanName:        strings.TrimSpace(asString(data["plan_name"])),
		RawResponseJSON: models.JSON(data),
		SubmittedAt:     submittedAt,
		FinishedAt:      finishedAt,
	}
	if err := h.RechargeJobRepo.UpsertByCardKey(job); err != nil {
		logger.Warnw("recharge_job_save_failed", "provider", models.RechargeProviderLyxazy, "product_type", models.RechargeProductChatGPTPlus, "error", err)
	}
}

func isRechargeTerminalStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case models.RechargeStatusSuccess, models.RechargeStatusFailed, models.RechargeStatusCancelled:
		return true
	default:
		return false
	}
}

func asString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", t)
	}
}

func (h *Handler) lyxazyBaseURL() string {
	if h == nil || h.Config == nil || strings.TrimSpace(h.Config.Lyxazy.BaseURL) == "" {
		return lyxazyDefaultBaseURL
	}
	return strings.TrimRight(strings.TrimSpace(h.Config.Lyxazy.BaseURL), "/")
}

func (h *Handler) lyxazyAPIKey() string {
	if h == nil || h.Config == nil {
		return ""
	}
	return strings.TrimSpace(h.Config.Lyxazy.APIKey)
}

func (h *Handler) lyxazyJSON(c *gin.Context, method, path string, payload interface{}) (map[string]interface{}, int, error) {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, response.CodeBadRequest, fmt.Errorf("请求参数序列化失败")
		}
		body = bytes.NewReader(raw)
	}

	endpoint := h.lyxazyBaseURL() + path
	req, err := http.NewRequestWithContext(c.Request.Context(), method, endpoint, body)
	if err != nil {
		return nil, response.CodeInternal, fmt.Errorf("创建上游请求失败")
	}
	req.Header.Set("X-API-Key", h.lyxazyAPIKey())
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: lyxazyHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, response.CodeInternal, fmt.Errorf("上游服务请求失败，请稍后重试")
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, response.CodeInternal, fmt.Errorf("读取上游响应失败")
	}

	var data map[string]interface{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &data); err != nil {
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil, response.CodeInternal, fmt.Errorf("上游响应格式错误")
			}
			return nil, mapHTTPStatusToBusinessCode(resp.StatusCode), fmt.Errorf("上游服务返回错误：HTTP %d", resp.StatusCode)
		}
	} else {
		data = map[string]interface{}{}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, mapHTTPStatusToBusinessCode(resp.StatusCode), errors.New(extractLyxazyErrorMessage(data, resp.StatusCode))
	}

	return data, response.CodeOK, nil
}

func mapHTTPStatusToBusinessCode(status int) int {
	switch status {
	case http.StatusBadRequest:
		return response.CodeBadRequest
	case http.StatusUnauthorized:
		return response.CodeUnauthorized
	case http.StatusForbidden:
		return response.CodeForbidden
	case http.StatusNotFound:
		return response.CodeNotFound
	case http.StatusTooManyRequests:
		return response.CodeTooManyRequests
	default:
		return response.CodeInternal
	}
}

func extractLyxazyErrorMessage(data map[string]interface{}, status int) string {
	for _, key := range []string{"message", "msg", "detail", "error", "error_code"} {
		if v, ok := data[key].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return fmt.Sprintf("上游服务返回错误：HTTP %d", status)
}
