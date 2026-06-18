package public

import (
	"fmt"
	"strings"
	"time"

	"github.com/dujiao-next/internal/http/response"
	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/recharge"

	"github.com/gin-gonic/gin"
)

type geminiBalanceRequest struct {
	CDKey string `json:"cdkey"`
}

type geminiSubmitRequest struct {
	CDKey    string `json:"cdkey"`
	Email    string `json:"email"`
	Password string `json:"password"`
	TwoFA    string `json:"twofa"`
	TaskType string `json:"task_type"`
}

type geminiStatusRequest struct {
	CDKey  string `json:"cdkey"`
	TaskID int64  `json:"task_id"`
	Email  string `json:"email"`
}

// TODO(viva-recharge-provider): 后续将 provider/product/redeem_mode 绑定到商品或卡密来源，
// 当前 Gemini MVP 固定使用 aidone Provider，ChatGPT Plus 固定使用 lyxazy Provider。

func (h *Handler) GetGeminiRechargeBalance(c *gin.Context) {
	var req geminiBalanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, response.CodeBadRequest, "请求参数格式错误")
		return
	}
	cdkey := strings.TrimSpace(req.CDKey)
	if cdkey == "" {
		response.Error(c, response.CodeBadRequest, "请输入卡密")
		return
	}
	if err := h.ensureRechargeCardOwned(cdkey); err != nil {
		response.Error(c, response.CodeForbidden, err.Error())
		return
	}
	data, statusCode, err := h.geminiRechargeProvider().GetGeminiBalance(c.Request.Context(), cdkey)
	if err != nil {
		response.Error(c, statusCode, err.Error())
		return
	}
	response.Success(c, data)
}

func (h *Handler) SubmitGeminiRecharge(c *gin.Context) {
	var req geminiSubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, response.CodeBadRequest, "请求参数格式错误")
		return
	}
	cdkey := strings.TrimSpace(req.CDKey)
	email := strings.TrimSpace(req.Email)
	if cdkey == "" {
		response.Error(c, response.CodeBadRequest, "请输入卡密")
		return
	}
	if email == "" {
		response.Error(c, response.CodeBadRequest, "请输入 Google 邮箱")
		return
	}
	if err := h.ensureRechargeCardOwned(cdkey); err != nil {
		response.Error(c, response.CodeForbidden, err.Error())
		return
	}
	if existing := h.getExistingRechargeRecord(cdkey); existing != nil && strings.TrimSpace(existing.UpstreamJobID) != "" {
		response.Success(c, rechargeJobToGeminiPublicMap(existing))
		return
	}

	data, statusCode, err := h.geminiRechargeProvider().SubmitGemini(c.Request.Context(), recharge.GeminiSubmitInput{
		CDKey:    cdkey,
		Email:    email,
		Password: req.Password,
		TwoFA:    req.TwoFA,
		TaskType: req.TaskType,
	})
	if err != nil {
		response.Error(c, statusCode, err.Error())
		return
	}
	h.saveGeminiRechargeRecord(cdkey, email, data, true)
	response.Success(c, data)
}

func (h *Handler) QueryGeminiRechargeStatus(c *gin.Context) {
	var req geminiStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, response.CodeBadRequest, "请求参数格式错误")
		return
	}
	cdkey := strings.TrimSpace(req.CDKey)
	if cdkey == "" {
		response.Error(c, response.CodeBadRequest, "请输入卡密")
		return
	}
	if req.TaskID <= 0 && strings.TrimSpace(req.Email) == "" {
		response.Error(c, response.CodeBadRequest, "请输入 task_id 或邮箱")
		return
	}
	if err := h.ensureRechargeCardOwned(cdkey); err != nil {
		response.Error(c, response.CodeForbidden, err.Error())
		return
	}
	data, statusCode, err := h.geminiRechargeProvider().QueryGeminiStatus(c.Request.Context(), cdkey, req.TaskID, req.Email)
	if err != nil {
		response.Error(c, statusCode, err.Error())
		return
	}
	h.saveGeminiRechargeRecord(cdkey, strings.TrimSpace(req.Email), data, false)
	response.Success(c, data)
}

func (h *Handler) geminiRechargeProvider() recharge.Provider {
	if h == nil || h.Config == nil {
		return recharge.NewAidoneProvider("")
	}
	return recharge.NewAidoneProvider(h.Config.Aidone.BaseURL)
}

func (h *Handler) saveGeminiRechargeRecord(cdkey, email string, data map[string]interface{}, submitted bool) {
	if h == nil || h.RechargeJobRepo == nil || data == nil {
		return
	}
	cdkey = strings.TrimSpace(cdkey)
	if cdkey == "" {
		return
	}

	record := data
	if nested, ok := data["data"].(map[string]interface{}); ok {
		record = nested
	}
	status := normalizeGeminiStatus(asString(record["status"]))
	if status == "" {
		status = models.RechargeStatusSubmitted
	}
	message := strings.TrimSpace(asString(data["message"]))
	if message == "" {
		message = strings.TrimSpace(asString(record["message"]))
	}
	jobID := strings.TrimSpace(asString(data["task_id"]))
	if jobID == "" {
		jobID = strings.TrimSpace(asString(record["task_id"]))
	}
	activationEmail := strings.TrimSpace(asString(record["email"]))
	if activationEmail == "" {
		activationEmail = strings.TrimSpace(email)
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
		Provider:        models.RechargeProviderAidone,
		ProductType:     models.RechargeProductGemini,
		CardKey:         cdkey,
		UpstreamJobID:   jobID,
		Status:          status,
		Message:         message,
		ActivationEmail: activationEmail,
		PlanName:        "Gemini",
		RawResponseJSON: models.JSON(data),
		SubmittedAt:     submittedAt,
		FinishedAt:      finishedAt,
	}
	if err := h.RechargeJobRepo.UpsertByCardKey(job); err != nil {
		logger.Warnw("gemini_recharge_job_save_failed", "provider", models.RechargeProviderAidone, "error", err)
	}
}

func normalizeGeminiStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "pending", "queued":
		return models.RechargeStatusQueued
	case "running", "processing", "executing":
		return models.RechargeStatusRunning
	case "success", "succeeded", "completed", "done":
		return models.RechargeStatusSuccess
	case "failed", "fail", "error":
		return models.RechargeStatusFailed
	case "cancelled", "canceled":
		return models.RechargeStatusCancelled
	case "":
		return ""
	default:
		return strings.ToLower(strings.TrimSpace(raw))
	}
}

func rechargeJobToGeminiPublicMap(job *models.RechargeJob) map[string]interface{} {
	if job == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"success":      true,
		"task_id":      parseMaybeInt(job.UpstreamJobID),
		"status":       job.Status,
		"message":      job.Message,
		"cdkey":        job.CardKey,
		"email":        job.ActivationEmail,
		"duplicate":    true,
		"local_record": true,
		"created_at":   job.CreatedAt.UnixMilli(),
		"updated_at":   job.UpdatedAt.UnixMilli(),
	}
}

func parseMaybeInt(raw string) interface{} {
	var n int64
	if _, err := fmt.Sscanf(strings.TrimSpace(raw), "%d", &n); err == nil {
		return n
	}
	return raw
}
