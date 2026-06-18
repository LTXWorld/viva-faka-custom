package recharge

import "context"

const (
	ProviderLyxazy     = "lyxazy"
	ProductChatGPTPlus = "chatgpt_plus"
)

// ChatGPTPlusSubmitInput ChatGPT Plus 直充提交参数。
type ChatGPTPlusSubmitInput struct {
	CardKey     string
	SessionData interface{}
}

// Provider 直充/兑换上游供应商接口。
// 后续新增供应商时，实现该接口并在业务层选择对应 Provider 即可。
type Provider interface {
	Name() string
	SubmitChatGPTPlus(ctx context.Context, input ChatGPTPlusSubmitInput) (map[string]interface{}, int, error)
	QueryChatGPTPlusByCardKey(ctx context.Context, cardKey string) (map[string]interface{}, int, error)
}
