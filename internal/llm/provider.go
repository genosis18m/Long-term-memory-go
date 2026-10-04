// Package llm is the LLM transport: a Provider is a thin go-openai wrapper offering one chat call, a
// truncation-escalation retry and an output ceiling.

package llm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/genosis18m/Long-term-memory-go/internal/common"
	"github.com/genosis18m/Long-term-memory-go/internal/config"
	openai "github.com/sashabaranov/go-openai"
)

const (
	// defaultTimeoutSecs 是 LlmConfig.TimeoutSecs 未设置时的 HTTP 超时。.
	defaultTimeoutSecs = 120
	// defaultMaxOutputTokens 是 LlmConfig.MaxOutputTokens 未设置时的输出上限。.
	defaultMaxOutputTokens = 8192
)

// Provider 是 go-openai 客户端的薄封装。.
type Provider struct {
	client          *openai.Client
	model           string
	maxOutputTokens int
}

// budgets reads the two budgets the same way every other tuning argument here is read: an unfilled
// value (0 or below) takes the library default.
func budgets(llm config.LlmConfig) (timeoutSecs, maxOutputTokens int) {
	timeoutSecs, maxOutputTokens = llm.TimeoutSecs, llm.MaxOutputTokens
	if timeoutSecs <= 0 {
		timeoutSecs = defaultTimeoutSecs
	}
	if maxOutputTokens <= 0 {
		maxOutputTokens = defaultMaxOutputTokens
	}
	return timeoutSecs, maxOutputTokens
}

// New 从一份 LLM 配置创建 Provider。.
func New(llm config.LlmConfig) *Provider {
	timeoutSecs, maxTokens := budgets(llm)
	oc := openai.DefaultConfig(llm.APIKey)
	oc.BaseURL = normalizeBaseURL(llm.APIURL)
	oc.HTTPClient = &http.Client{Timeout: time.Duration(timeoutSecs) * time.Second}
	return &Provider{
		client:          openai.NewClientWithConfig(oc),
		model:           llm.Model,
		maxOutputTokens: maxTokens,
	}
}

// normalizeBaseURL 确保 BaseURL 以 /v1 结尾（go-openai 不自动补），.
func normalizeBaseURL(raw string) string {
	u := strings.TrimRight(strings.TrimSpace(raw), "/")
	if before, ok := strings.CutSuffix(u, "/chat/completions"); ok {
		u = before
	}
	if !strings.HasSuffix(u, "/v1") {
		u += "/v1"
	}
	return u
}

// Chat 执行一次非流式 chat completion，对 429/5xx 做指数退避重试.
func (p *Provider) Chat(ctx context.Context, system, user string, maxTokens int) (string, error) {
	req := openai.ChatCompletionRequest{
		Model: p.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: system},
			{Role: openai.ChatMessageRoleUser, Content: user},
		},
		MaxTokens:        maxTokens,
		Temperature:      0.0,
		TopP:             1.0,
		PresencePenalty:  0.0,
		FrequencyPenalty: 0.0,
		Stream:           false,
	}
	delays := []time.Duration{500 * time.Millisecond, 2 * time.Second}
	var lastErr error
	for attempt := 0; attempt <= len(delays); attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return "", common.NewError(common.ErrCancelled,
					"llm call abandoned before its retry", ctx.Err())
			case <-time.After(delays[attempt-1]):
			}
		}
		resp, err := p.client.CreateChatCompletion(ctx, req)
		if err != nil {
			if cerr := ctx.Err(); cerr != nil {
				// The caller's context is gone, so whatever the HTTP stack reported is that cancellation travelling
				// through it.
				return "", common.NewError(common.ErrCancelled, "llm call cancelled", cerr)
			}
			status, msg := httpError(err)
			if status > 0 {
				lastErr = common.NewError(common.ErrLLM, fmt.Sprintf("llm api: %d - %s", status, msg))
				if !retryable(status) {
					return "", lastErr
				}
				continue
			}
			return "", common.NewError(common.ErrLLM, "llm api call failed", err)
		}
		if len(resp.Choices) == 0 {
			return "", common.NewError(common.ErrLLM, "llm response has no choices")
		}
		if resp.Choices[0].FinishReason == openai.FinishReasonLength {
			return "", common.NewError(common.ErrLLM,
				fmt.Sprintf("llm response hit the output ceiling at max_tokens=%d", maxTokens),
				common.ErrTruncated)
		}
		return resp.Choices[0].Message.Content, nil
	}
	// The last attempt's failure is what the caller gets, and lastErr holds it: the loop only ends here
	// when that attempt failed on a status worth retrying.
	return "", lastErr
}

// httpError 从 go-openai 错误中提取 HTTP 状态码与消息体；RequestError 优先.
func httpError(err error) (int, string) {
	var reqErr *openai.RequestError
	if errors.As(err, &reqErr) && reqErr.HTTPStatusCode > 0 {
		return reqErr.HTTPStatusCode, clampEcho(reqErr.Body)
	}
	var apiErr *openai.APIError
	if errors.As(err, &apiErr) && apiErr.HTTPStatusCode > 0 {
		return apiErr.HTTPStatusCode, apiErr.Message
	}
	return 0, ""
}

// maxUpstreamEcho 是上游错误正文的死额度：正文原样进错误对象，既回给调用方又被.
const maxUpstreamEcho = 256

// clampEcho keeps the head of a body that is not ours in length or charset.
func clampEcho(body []byte) string {
	if len(body) <= maxUpstreamEcho {
		return string(body)
	}
	return strings.ToValidUTF8(string(body[:maxUpstreamEcho]), "") + "…"
}

// retryable 判断状态码是否属于值得重试的瞬时错误（429 与 5xx）。.
func retryable(status int) bool {
	switch status {
	case http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	}
	return false
}
