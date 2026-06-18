package recharge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dujiao-next/internal/http/response"
)

const (
	AidoneDefaultBaseURL = "https://aidone.lol/openapi"
	aidoneHTTPTimeout    = 30 * time.Second
)

// AidoneProvider aidone Gemini 直充 API Provider。
// 该上游使用用户 cdkey 作为唯一鉴权与计费凭证，不需要服务端 API Key。
type AidoneProvider struct {
	baseURL string
	client  *http.Client
}

func NewAidoneProvider(baseURL string) *AidoneProvider {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = AidoneDefaultBaseURL
	}
	return &AidoneProvider{
		baseURL: baseURL,
		client:  &http.Client{Timeout: aidoneHTTPTimeout},
	}
}

func (p *AidoneProvider) Name() string { return ProviderAidone }

func (p *AidoneProvider) SubmitChatGPTPlus(ctx context.Context, input ChatGPTPlusSubmitInput) (map[string]interface{}, int, error) {
	return nil, response.CodeBadRequest, errors.New("aidone provider does not support ChatGPT Plus")
}

func (p *AidoneProvider) QueryChatGPTPlusByCardKey(ctx context.Context, cardKey string) (map[string]interface{}, int, error) {
	return nil, response.CodeBadRequest, errors.New("aidone provider does not support ChatGPT Plus")
}

func (p *AidoneProvider) GetGeminiBalance(ctx context.Context, cdkey string) (map[string]interface{}, int, error) {
	return p.doPOST(ctx, map[string]interface{}{
		"action": "get_balance",
		"cdkey":  strings.TrimSpace(cdkey),
	})
}

func (p *AidoneProvider) SubmitGemini(ctx context.Context, input GeminiSubmitInput) (map[string]interface{}, int, error) {
	taskType := strings.TrimSpace(input.TaskType)
	if taskType == "" {
		taskType = "full"
	}
	return p.doPOST(ctx, map[string]interface{}{
		"action":    "submit_task",
		"cdkey":     strings.TrimSpace(input.CDKey),
		"email":     strings.TrimSpace(input.Email),
		"password":  input.Password,
		"twofa":     strings.ReplaceAll(strings.TrimSpace(input.TwoFA), " ", ""),
		"task_type": taskType,
	})
}

func (p *AidoneProvider) QueryGeminiStatus(ctx context.Context, cdkey string, taskID int64, email string) (map[string]interface{}, int, error) {
	payload := map[string]interface{}{
		"action": "get_status",
		"cdkey":  strings.TrimSpace(cdkey),
	}
	if taskID > 0 {
		payload["task_id"] = taskID
	} else if strings.TrimSpace(email) != "" {
		payload["email"] = strings.TrimSpace(email)
	}
	return p.doPOST(ctx, payload)
}

func (p *AidoneProvider) doPOST(ctx context.Context, payload map[string]interface{}) (map[string]interface{}, int, error) {
	if p == nil {
		return nil, response.CodeInternal, errors.New("aidone provider not initialized")
	}
	if strings.TrimSpace(fmt.Sprintf("%v", payload["cdkey"])) == "" {
		return nil, response.CodeBadRequest, errors.New("请输入卡密")
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, response.CodeBadRequest, fmt.Errorf("请求参数序列化失败")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL, bytes.NewReader(raw))
	if err != nil {
		return nil, response.CodeInternal, fmt.Errorf("创建上游请求失败")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	client := p.client
	if client == nil {
		client = &http.Client{Timeout: aidoneHTTPTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, response.CodeInternal, fmt.Errorf("上游服务请求失败，请稍后重试")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, response.CodeInternal, fmt.Errorf("读取上游响应失败")
	}
	data := map[string]interface{}{}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &data); err != nil {
			return nil, response.CodeInternal, fmt.Errorf("上游响应格式错误")
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, mapHTTPStatusToBusinessCode(resp.StatusCode), errors.New(extractErrorMessage(data, resp.StatusCode))
	}
	if ok, exists := data["success"].(bool); exists && !ok {
		return nil, response.CodeBadRequest, errors.New(extractAidoneBusinessMessage(data))
	}
	return data, response.CodeOK, nil
}

func extractAidoneBusinessMessage(data map[string]interface{}) string {
	for _, key := range []string{"message", "msg", "error", "detail"} {
		if v, ok := data[key].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return "上游任务处理失败"
}
