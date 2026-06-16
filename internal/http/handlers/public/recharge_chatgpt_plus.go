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

	"github.com/dujiao-next/internal/http/response"

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

	path := "/api/plus/jobs/by-card-key?card_key=" + url.QueryEscape(cardKey)
	data, statusCode, err := h.lyxazyJSON(c, http.MethodGet, path, nil)
	if err != nil {
		response.Error(c, statusCode, err.Error())
		return
	}

	response.Success(c, data)
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
