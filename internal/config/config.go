// Package config holds the types that describe how a database is set up: the LLM endpoint and the
// tuning knobs.

package config

import (
	"fmt"
	"math"
	"time"

	"github.com/genosis18m/Long-term-memory-go/internal/common"
)

// MemHopConfig is one database's assembled configuration: where the file lives, which endpoint
// answers, which tuning applies.
type MemHopConfig struct {
	DBPath   string         `json:"db_path"`
	LLM      LlmConfig      `json:"llm"`
	Defaults MemHopDefaults `json:"defaults"`
}

// LlmConfig holds LLM provider settings.
type LlmConfig struct {
	APIURL          string `json:"api_url"`
	APIKey          string `json:"api_key"`
	Model           string `json:"model"`
	TimeoutSecs     int    `json:"timeout_secs"`
	MaxOutputTokens int    `json:"max_output_tokens"`
}

// MaxTimeoutSecs is the longest request timeout the client can be given: past it the seconds no longer
// fit a time.Duration once scaled to nanoseconds.
const MaxTimeoutSecs = int64(math.MaxInt64 / int64(time.Second))

// Validate reports whether one LLM endpoint is fully specified: there is no fallback for a call it
// cannot make, so a half-filled one is refused at the boundary.
func (c LlmConfig) Validate() error {
	if c.APIURL == "" || c.APIKey == "" || c.Model == "" {
		return common.NewError(common.ErrConfig, "LLM.APIURL, LLM.APIKey and LLM.Model are required")
	}
	// Compared in int64 so a 32-bit build — where no int can reach the ceiling — carries
	// the same rule instead of a constant that would not fit.
	if int64(c.TimeoutSecs) > MaxTimeoutSecs {
		return common.NewError(common.ErrConfig, fmt.Sprintf(
			"LLM.TimeoutSecs %d is past the longest timeout the client can hold (%d s): a longer one wraps the duration around, and the HTTP client reads a non-positive timeout as no timeout at all, which leaves a hung endpoint blocking this agent domain forever",
			c.TimeoutSecs, MaxTimeoutSecs))
	}
	return nil
}
