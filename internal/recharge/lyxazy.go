package recharge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dujiao-next/internal/http/response"
)

const (
	LyxazyDefaultBaseURL = "https://www.lyxazy.top/verify"
	lyxazyHTTPTimeout    = 30 * time.Second
)

// LyxazyProvider lyxazy 官方直充 API Provider。
type LyxazyProvider struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func NewLyxazyProvider(baseURL, apiKey string) *LyxazyProvider {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = LyxazyDefaultBaseURL
	}
	return &LyxazyProvider{
		baseURL: baseURL,
		apiKey:  strings.TrimSpace(apiKey),
		client:  &http.Client{Timeout: lyxazyHTTPTimeout},
	}
}

func (p *LyxazyProvider) Name() string { return ProviderLyxazy }

func (p *LyxazyProvider) SubmitChatGPTPlus(ctx context.Context, input ChatGPTPlusSubmitInput) (map[string]interface{}, int, error) {
	payload := map[string]interface{}{
		"card_key":     strings.TrimSpace(input.CardKey),
		"product_line": "plus",
		"session_data": input.SessionData,
	}
	return p.doJSON(ctx, http.MethodPost, "/api/plus/jobs", payload)
}

func (p *LyxazyProvider) QueryChatGPTPlusByCardKey(ctx context.Context, cardKey string) (map[string]interface{}, int, error) {
	path := "/api/plus/jobs/by-card-key?card_key=" + url.QueryEscape(strings.TrimSpace(cardKey))
	return p.doJSON(ctx, http.MethodGet, path, nil)
}

func (p *LyxazyProvider) doJSON(ctx context.Context, method, path string, payload interface{}) (map[string]interface{}, int, error) {
	if p == nil || strings.TrimSpace(p.apiKey) == "" {
		return nil, response.CodeInternal, errors.New("LYXAZY API Key 未配置")
	}

	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, response.CodeBadRequest, fmt.Errorf("请求参数序列化失败")
		}
		body = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, p.baseURL+path, body)
	if err != nil {
		return nil, response.CodeInternal, fmt.Errorf("创建上游请求失败")
	}
	req.Header.Set("X-API-Key", p.apiKey)
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := p.client
	if client == nil {
		client = &http.Client{Timeout: lyxazyHTTPTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, response.CodeInternal, fmt.Errorf("上游服务请求失败，请稍后重试")
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, response.CodeInternal, fmt.Errorf("读取上游响应失败")
	}

	data := map[string]interface{}{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &data); err != nil {
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil, response.CodeInternal, fmt.Errorf("上游响应格式错误")
			}
			return nil, mapHTTPStatusToBusinessCode(resp.StatusCode), fmt.Errorf("上游服务返回错误：HTTP %d", resp.StatusCode)
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, mapHTTPStatusToBusinessCode(resp.StatusCode), errors.New(extractErrorMessage(data, resp.StatusCode))
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

func extractErrorMessage(data map[string]interface{}, status int) string {
	for _, key := range []string{"message", "msg", "detail", "error", "error_code"} {
		if v, ok := data[key].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return fmt.Sprintf("上游服务返回错误：HTTP %d", status)
}
