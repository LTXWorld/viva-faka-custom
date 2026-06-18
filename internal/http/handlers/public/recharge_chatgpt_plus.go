package public

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/http/response"
	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/recharge"

	"gorm.io/gorm"

	"github.com/gin-gonic/gin"
)

type chatGPTPlusSubmitRequest struct {
	CardKey     string      `json:"card_key"`
	AccessToken string      `json:"access_token"`
	SessionData interface{} `json:"session_data"`
}

// SubmitChatGPTPlusRecharge POST /api/v1/public/recharge/chatgpt-plus/submit
// 代理提交 ChatGPT Plus 官方直充任务。注意：LYXAZY API Key 只在服务端配置，不能暴露到前端。
func (h *Handler) SubmitChatGPTPlusRecharge(c *gin.Context) {
	provider := h.chatGPTPlusRechargeProvider()

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
	batch, err := h.ensureRechargeCardOwned(cardKey)
	if err != nil {
		response.Error(c, response.CodeForbidden, err.Error())
		return
	}
	if err := ensureBatchSupportsAPI(batch, models.RechargeProviderLyxazy, models.RechargeProductChatGPTPlus); err != nil {
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

	data, statusCode, err := provider.SubmitChatGPTPlus(c.Request.Context(), recharge.ChatGPTPlusSubmitInput{
		CardKey:     cardKey,
		SessionData: sessionData,
	})
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
	provider := h.chatGPTPlusRechargeProvider()

	cardKey := strings.TrimSpace(c.Query("card_key"))
	if cardKey == "" {
		response.Error(c, response.CodeBadRequest, "请输入卡密")
		return
	}
	batch, err := h.ensureRechargeCardOwned(cardKey)
	if err != nil {
		response.Error(c, response.CodeForbidden, err.Error())
		return
	}
	if err := ensureBatchSupportsAPI(batch, models.RechargeProviderLyxazy, models.RechargeProductChatGPTPlus); err != nil {
		response.Error(c, response.CodeForbidden, err.Error())
		return
	}

	data, statusCode, err := provider.QueryChatGPTPlusByCardKey(c.Request.Context(), cardKey)
	if err != nil {
		response.Error(c, statusCode, err.Error())
		return
	}

	h.saveChatGPTPlusRechargeRecordsFromQuery(cardKey, data)

	response.Success(c, data)
}

func (h *Handler) ensureRechargeCardOwned(cardKey string) (*models.CardSecretBatch, error) {
	if h == nil || h.CardSecretRepo == nil {
		return nil, fmt.Errorf("卡密校验服务不可用")
	}
	secret, order, err := h.CardSecretRepo.FindSoldBySecret(cardKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("卡密不存在或尚未售出，请确认你输入的是在 Viva 小铺已付款订单中的卡密")
		}
		logger.Warnw("recharge_card_ownership_check_failed", "error", err)
		return nil, fmt.Errorf("卡密校验失败，请稍后重试")
	}
	if order == nil || !isPaidRechargeOrderStatus(order.Status) {
		return nil, fmt.Errorf("卡密关联订单未完成付款，暂不能兑换")
	}
	if secret != nil {
		return secret.Batch, nil
	}
	return nil, nil
}

func ensureBatchSupportsAPI(batch *models.CardSecretBatch, provider, productType string) error {
	if batch == nil {
		return nil
	}
	configuredProvider := strings.TrimSpace(batch.RechargeProvider)
	configuredProduct := strings.TrimSpace(batch.RechargeProductType)
	configuredMode := strings.TrimSpace(batch.RedeemMode)
	// 兼容历史批次：未配置兑换来源时，仍按当前页面默认 Provider 处理。
	if configuredProvider == "" && configuredProduct == "" && configuredMode == "" {
		return nil
	}
	if configuredMode != "" && configuredMode != models.RedeemModeAPI {
		if strings.TrimSpace(batch.RedeemURL) != "" {
			return fmt.Errorf("该卡密需要前往外部地址兑换：%s", strings.TrimSpace(batch.RedeemURL))
		}
		return fmt.Errorf("该卡密不支持本站自动兑换，请联系商家处理")
	}
	if configuredProvider != "" && configuredProvider != provider {
		return fmt.Errorf("该卡密供应商与当前兑换页面不匹配")
	}
	if configuredProduct != "" && configuredProduct != productType {
		return fmt.Errorf("该卡密产品类型与当前兑换页面不匹配")
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

func (h *Handler) chatGPTPlusRechargeProvider() recharge.Provider {
	if h == nil || h.Config == nil {
		return recharge.NewLyxazyProvider("", "")
	}
	return recharge.NewLyxazyProvider(h.Config.Lyxazy.BaseURL, h.Config.Lyxazy.APIKey)
}
