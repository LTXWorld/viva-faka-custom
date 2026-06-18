package recharge

import "context"

const (
	ProviderLyxazy     = "lyxazy"
	ProviderAidone     = "aidone"
	ProductChatGPTPlus = "chatgpt_plus"
	ProductGemini      = "gemini"
)

// ChatGPTPlusSubmitInput ChatGPT Plus 直充提交参数。
type ChatGPTPlusSubmitInput struct {
	CardKey     string
	SessionData interface{}
}

// GeminiSubmitInput Gemini 直充提交参数。
type GeminiSubmitInput struct {
	CDKey    string
	Email    string
	Password string
	TwoFA    string
	TaskType string
}

// Provider 直充/兑换上游供应商接口。
// 后续新增供应商时，实现该接口并在业务层选择对应 Provider 即可。
type Provider interface {
	Name() string
	SubmitChatGPTPlus(ctx context.Context, input ChatGPTPlusSubmitInput) (map[string]interface{}, int, error)
	QueryChatGPTPlusByCardKey(ctx context.Context, cardKey string) (map[string]interface{}, int, error)
	GetGeminiBalance(ctx context.Context, cdkey string) (map[string]interface{}, int, error)
	SubmitGemini(ctx context.Context, input GeminiSubmitInput) (map[string]interface{}, int, error)
	QueryGeminiStatus(ctx context.Context, cdkey string, taskID int64, email string) (map[string]interface{}, int, error)
}
