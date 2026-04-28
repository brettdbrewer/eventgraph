package intelligence

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCodexJSONL_HappyPath(t *testing.T) {
	jsonl := `{"type":"thread.started","thread_id":"abc123"}
{"type":"turn.started"}
{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"Hello world"}}
{"type":"turn.completed","usage":{"input_tokens":100,"cached_input_tokens":50,"output_tokens":20,"reasoning_output_tokens":5}}`

	text, usage, err := parseCodexJSONL([]byte(jsonl))
	require.NoError(t, err)
	assert.Equal(t, "Hello world", text)
	assert.Equal(t, 100, usage.InputTokens)
	assert.Equal(t, 50, usage.CachedInputTokens)
	assert.Equal(t, 20, usage.OutputTokens)
	assert.Equal(t, 5, usage.ReasoningTokens)
}

func TestParseCodexJSONL_MultipleMessages(t *testing.T) {
	jsonl := `{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"first"}}
{"type":"item.completed","item":{"id":"item_1","type":"command_execution","command":"ls"}}
{"type":"item.completed","item":{"id":"item_2","type":"agent_message","text":"second"}}
{"type":"turn.completed","usage":{"input_tokens":200,"output_tokens":40}}`

	text, _, err := parseCodexJSONL([]byte(jsonl))
	require.NoError(t, err)
	assert.Equal(t, "second", text, "should return last agent message")
}

func TestParseCodexJSONL_TurnFailed(t *testing.T) {
	jsonl := `{"type":"turn.started"}
{"type":"turn.failed","error":{"message":"model not supported"}}`

	_, _, err := parseCodexJSONL([]byte(jsonl))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "model not supported")
}

func TestParseCodexJSONL_ErrorEvent(t *testing.T) {
	jsonl := `{"type":"error","message":"invalid request"}`

	_, _, err := parseCodexJSONL([]byte(jsonl))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid request")
}

func TestParseCodexJSONL_NoMessages(t *testing.T) {
	jsonl := `{"type":"turn.started"}
{"type":"turn.completed","usage":{"input_tokens":10}}`

	_, _, err := parseCodexJSONL([]byte(jsonl))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no agent messages")
}

func TestParseCodexJSONL_SkipsMalformedLines(t *testing.T) {
	jsonl := `not json at all
{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"works"}}
also not json
{"type":"turn.completed","usage":{"input_tokens":10}}`

	text, _, err := parseCodexJSONL([]byte(jsonl))
	require.NoError(t, err)
	assert.Equal(t, "works", text)
}

func TestParseCodexJSONL_TurnFailedWithMessage(t *testing.T) {
	// If there's a message AND an error, the message wins (partial success).
	jsonl := `{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"partial result"}}
{"type":"turn.failed","error":{"message":"context cancelled"}}`

	text, _, err := parseCodexJSONL([]byte(jsonl))
	require.NoError(t, err)
	assert.Equal(t, "partial result", text)
}

func TestNewCodexCliProvider_Defaults(t *testing.T) {
	// This test only works if codex is in PATH — skip otherwise.
	p, err := newCodexCliProvider(Config{Provider: "codex-cli"})
	if err != nil {
		t.Skipf("codex not in PATH: %v", err)
	}
	assert.Equal(t, "codex-cli", p.Name())
	assert.Equal(t, "o3", p.Model(), "default model should be o3")
}

func TestNewCodexCliProvider_CustomModel(t *testing.T) {
	p, err := newCodexCliProvider(Config{Provider: "codex-cli", Model: "o4-mini"})
	if err != nil {
		t.Skipf("codex not in PATH: %v", err)
	}
	assert.Equal(t, "o4-mini", p.Model())
}
