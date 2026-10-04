// Copyright (c) 2026 qyiun666
// SPDX-License-Identifier: MIT OR Apache-2.0

// Package llmops hosts the LLM-assisted cognitive capabilities of the memory engine: keyword
// extraction, L2 consolidation and L1->L0 distillation.
package llmops

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/genosis18m/Long-term-memory-go/internal/common"
)

// Chat is the injected chat-completion transport.
type Chat interface {
	Chat(ctx context.Context, system, user string, maxTokens int) (string, error)
	ChatWithRetry(ctx context.Context, system, user string, primaryMax, retryMax int) (string, error)
	MaxOutputTokens() int
}

// ConsolidationMaxTokens is the L2 consolidation output ceiling and the widest budget keyword
// extraction ever asks for.
const ConsolidationMaxTokens = 8192

// escalationCeiling is the widest output budget a retry may ask for: what the endpoint was configured
// to accept, since asking above it is a refused request.
func escalationCeiling(chat Chat) int {
	if n := chat.MaxOutputTokens(); n > 0 {
		return n
	}
	return ConsolidationMaxTokens
}

// minTokens keeps a call's output cap at or below the configured ceiling.
func minTokens(configured, ceiling int) int {
	if configured <= 0 || configured > ceiling {
		return ceiling
	}
	return configured
}

// jsonExchange is one strict-JSON exchange with the model.
type jsonExchange struct {
	what        string
	system      string
	user        string
	formatRetry string
	ceiling     int
}

// askJSON runs one exchange and hands the reply to parse, which decides whether it honoured the
// contract.
func askJSON[T any](ctx context.Context, chat Chat, call jsonExchange,
	parse func(string) (T, error)) (T, error) {
	budget := minTokens(chat.MaxOutputTokens(), call.ceiling)
	var zero T
	response, err := chat.ChatWithRetry(ctx, call.system, call.user, budget, escalationCeiling(chat))
	if err != nil {
		return zero, err
	}
	out, perr := parse(response)
	if perr == nil {
		return out, nil
	}
	retry, rerr := chat.Chat(ctx, call.system, call.user+call.formatRetry, budget)
	if rerr != nil {
		return zero, common.NewError(common.CodeOf(rerr),
			call.what+" format retry after an off-contract reply", errors.Join(perr, rerr))
	}
	return parse(retry)
}

// parseUint64Flex parses a JSON number or quoted string as uint64, decimal first then hex (0x prefix).
func parseUint64Flex(raw json.RawMessage) (uint64, error) {
	s := strings.Trim(strings.TrimSpace(string(raw)), `"`)
	if s == "" || s == "null" {
		return 0, common.NewError(common.ErrLLM, "empty uint64 value")
	}
	if v, err := strconv.ParseUint(s, 10, 64); err == nil {
		return v, nil
	}
	return strconv.ParseUint(strings.TrimPrefix(s, "0x"), 16, 64)
}

// stripCodeBlocks removes ```lang ...
func stripCodeBlocks(s string) string {
	trimmed := strings.TrimSpace(s)
	if !strings.HasPrefix(trimmed, "```") {
		return trimmed
	}
	body := trimmed[3:]
	if nl := strings.IndexByte(body, '\n'); nl >= 0 {
		body = body[nl+1:]
	} else {
		body = ""
	}
	if before, _, found := strings.CutLast(body, "```"); found {
		body = before
	}
	return strings.TrimSpace(body)
}
