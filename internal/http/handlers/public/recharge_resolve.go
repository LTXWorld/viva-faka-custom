package public

import (
	"strings"

	"github.com/dujiao-next/internal/http/response"
	"github.com/dujiao-next/internal/models"

	"github.com/gin-gonic/gin"
)

type rechargeResolveRequest struct {
	CardKey string `json:"card_key"`
}

// ResolveRechargeCard POST /api/v1/public/recharge/resolve
// 统一兑换入口：根据本站已售卡密所属批次的兑换来源绑定，返回推荐兑换方式。
func (h *Handler) ResolveRechargeCard(c *gin.Context) {
	var req rechargeResolveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, response.CodeBadRequest, "请求参数格式错误")
		return
	}
	cardKey := strings.TrimSpace(req.CardKey)
	if cardKey == "" {
		response.Error(c, response.CodeBadRequest, "请输入卡密")
		return
	}
	batch, err := h.ensureRechargeCardOwned(cardKey)
	if err != nil {
		response.Error(c, response.CodeForbidden, err.Error())
		return
	}

	data := gin.H{
		"card_key":     cardKey,
		"redeem_mode":  "unknown",
		"message":      "该卡密所属批次尚未绑定兑换来源，请选择对应兑换页面或联系商家。",
		"target_path":  "",
		"redeem_url":   "",
		"provider":     "",
		"product_type": "",
	}
	if batch == nil {
		response.Success(c, data)
		return
	}

	provider := strings.TrimSpace(batch.RechargeProvider)
	productType := strings.TrimSpace(batch.RechargeProductType)
	redeemMode := strings.TrimSpace(batch.RedeemMode)
	if redeemMode == "" && (provider != "" || productType != "") {
		redeemMode = models.RedeemModeAPI
	}
	if redeemMode == "" {
		response.Success(c, data)
		return
	}

	data["provider"] = provider
	data["product_type"] = productType
	data["redeem_mode"] = redeemMode
	data["redeem_url"] = strings.TrimSpace(batch.RedeemURL)

	switch redeemMode {
	case models.RedeemModeAPI:
		switch productType {
		case models.RechargeProductChatGPTPlus:
			data["target_path"] = "/redeem/chatgpt-plus?card_key=" + cardKey
			data["message"] = "已识别为 ChatGPT Plus 卡密，请前往 ChatGPT Plus 兑换页。"
		case models.RechargeProductGemini:
			data["target_path"] = "/redeem/gemini?cdkey=" + cardKey
			data["message"] = "已识别为 Gemini 卡密，请前往 Gemini 兑换页。"
		default:
			data["redeem_mode"] = "unknown"
			data["message"] = "该 API 卡密产品类型暂不支持自动识别，请联系商家。"
		}
	case models.RedeemModeExternalURL:
		data["message"] = "该卡密需要前往外部网站兑换。"
	case models.RedeemModeManual:
		data["message"] = "该卡密需要人工处理，请联系商家或按商品说明提交资料。"
	default:
		data["redeem_mode"] = "unknown"
		data["message"] = "该卡密兑换模式暂不支持，请联系商家。"
	}

	response.Success(c, data)
}
