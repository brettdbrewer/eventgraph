package intelligence

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/lovyou-ai/eventgraph/go/pkg/decision"
	"github.com/lovyou-ai/eventgraph/go/pkg/event"
)

const (
	// defaultCodexReasonTimeout is the maximum time a single Reason() call
	// can run against the Codex CLI.
	defaultCodexReasonTimeout = 10 * time.Minute

	// defaultCodexOperateTimeout is the maximum time a single Operate() call
	// can run against the Codex CLI.
	defaultCodexOperateTimeout = 15 * time.Minute
)

// codexEvent is a single line from `codex exec --json` JSONL output.
type codexEvent struct {
	Type    string          `json:"type"`
	Message string          `json:"message,omitempty"` // error events
	Item    *codexItem      `json:"item,omitempty"`    // item.completed events
	Usage   *codexUsage     `json:"usage,omitempty"`   // turn.completed events
	Error   *codexTurnError `json:"error,omitempty"`   // turn.failed events
}

type codexItem struct {
	ID   string `json:"id"`
	Type string `json:"type"` // "agent_message", "command_execution"
	Text string `json:"text,omitempty"`
}

type codexUsage struct {
	InputTokens         int `json:"input_tokens"`
	CachedInputTokens   int `json:"cached_input_tokens"`
	OutputTokens        int `json:"output_tokens"`
	ReasoningTokens     int `json:"reasoning_output_tokens"`
}

type codexTurnError struct {
	Message string `json:"message"`
}

// codexCliProvider implements Provider by shelling out to the `codex` CLI.
// This uses whatever authentication Codex already has (ChatGPT Plus, API key,
// etc.) without requiring separate credentials.
type codexCliProvider struct {
	model        string
	systemPrompt string
	codexPath    string // path to codex binary, default "codex"
}

func newCodexCliProvider(cfg Config) (*codexCliProvider, error) {
	model := cfg.Model
	if model == "" {
		model = "o3" // Codex default
	}

	codexPath := "codex"
	if cfg.BaseURL != "" {
		// BaseURL repurposed as path to codex binary for testing.
		codexPath = cfg.BaseURL
	}

	if _, err := exec.LookPath(codexPath); err != nil {
		return nil, fmt.Errorf("codex CLI not found in PATH: %w", err)
	}

	return &codexCliProvider{
		model:        model,
		systemPrompt: cfg.SystemPrompt,
		codexPath:    codexPath,
	}, nil
}

func (p *codexCliProvider) Name() string  { return "codex-cli" }
func (p *codexCliProvider) Model() string { return p.model }

func (p *codexCliProvider) Reason(ctx context.Context, prompt string, history []event.Event) (decision.Response, error) {
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, defaultCodexReasonTimeout)
	defer cancel()

	var fullPrompt strings.Builder
	historyText := eventsToMessages(history)
	if historyText != "" {
		fullPrompt.WriteString(historyText)
		fullPrompt.WriteString("\n---\n\n")
	}
	fullPrompt.WriteString(prompt)

	args := []string{
		"exec",
		"--json",
		"--ephemeral",
		"-m", p.model,
	}
	if p.systemPrompt != "" {
		args = append(args, "-c", fmt.Sprintf("system_prompt=%q", p.systemPrompt))
	}
	args = append(args, "-") // read prompt from stdin

	cmd := exec.CommandContext(ctx, p.codexPath, args...)
	cmd.Stdin = strings.NewReader(fullPrompt.String())

	env := removeEnv(cmd.Environ(), "CLAUDECODE")
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := runWithProgress(cmd, "  ⏳ codex thinking"); err != nil {
		// Try to extract useful info from JSONL even on error.
		if stdout.Len() > 0 {
			text, _, parseErr := parseCodexJSONL(stdout.Bytes())
			if parseErr == nil && text != "" {
				return decision.NewResponse(text, defaultConfidence(), decision.TokenUsage{}), nil
			}
			// Include the JSONL parse error (often contains the real reason).
			if parseErr != nil {
				return decision.Response{}, fmt.Errorf("codex CLI error: %w (%v)", err, parseErr)
			}
		}
		return decision.Response{}, fmt.Errorf("codex CLI error: %w\nstderr: %s", err, stderr.String())
	}

	text, usage, err := parseCodexJSONL(stdout.Bytes())
	if err != nil {
		return decision.Response{}, err
	}

	tokenUsage := decision.TokenUsage{
		InputTokens:     usage.InputTokens,
		OutputTokens:    usage.OutputTokens,
		CacheReadTokens: usage.CachedInputTokens,
	}

	return decision.NewResponse(text, defaultConfidence(), tokenUsage), nil
}

func (p *codexCliProvider) Operate(ctx context.Context, task decision.OperateTask) (decision.OperateResult, error) {
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, defaultCodexOperateTimeout)
	defer cancel()

	if task.WorkDir == "" {
		return decision.OperateResult{}, fmt.Errorf("Operate requires WorkDir")
	}
	if task.Instruction == "" {
		return decision.OperateResult{}, fmt.Errorf("Operate requires Instruction")
	}

	args := []string{
		"exec",
		"--json",
		"--ephemeral",
		"-m", p.model,
		"-C", task.WorkDir,
		"--dangerously-bypass-approvals-and-sandbox",
	}
	if p.systemPrompt != "" {
		args = append(args, "-c", fmt.Sprintf("system_prompt=%q", p.systemPrompt))
	}
	args = append(args, "-") // read from stdin

	cmd := exec.CommandContext(ctx, p.codexPath, args...)
	cmd.Stdin = strings.NewReader(task.Instruction)

	env := removeEnv(cmd.Environ(), "CLAUDECODE")
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := runWithProgress(cmd, "  ⏳ codex working"); err != nil {
		if stdout.Len() > 0 {
			text, _, parseErr := parseCodexJSONL(stdout.Bytes())
			if parseErr == nil && text != "" {
				return decision.OperateResult{Summary: text}, nil
			}
			if parseErr != nil {
				return decision.OperateResult{}, fmt.Errorf("codex CLI operate error: %w (%v)", err, parseErr)
			}
		}
		return decision.OperateResult{}, fmt.Errorf("codex CLI operate error: %w\nstderr: %s", err, stderr.String())
	}

	text, usage, err := parseCodexJSONL(stdout.Bytes())
	if err != nil {
		return decision.OperateResult{}, err
	}

	return decision.OperateResult{
		Summary: text,
		Usage: decision.TokenUsage{
			InputTokens:     usage.InputTokens,
			OutputTokens:    usage.OutputTokens,
			CacheReadTokens: usage.CachedInputTokens,
		},
	}, nil
}

// parseCodexJSONL extracts the final agent message text and usage from Codex
// JSONL output. It collects all agent_message items and returns the last
// turn.completed usage block.
func parseCodexJSONL(data []byte) (string, codexUsage, error) {
	var messages []string
	var usage codexUsage
	var turnError string

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var ev codexEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue // skip unparseable lines
		}

		switch ev.Type {
		case "item.completed":
			if ev.Item != nil && ev.Item.Type == "agent_message" && ev.Item.Text != "" {
				messages = append(messages, ev.Item.Text)
			}
		case "turn.completed":
			if ev.Usage != nil {
				usage = *ev.Usage
			}
		case "turn.failed":
			if ev.Error != nil {
				turnError = ev.Error.Message
			}
		case "error":
			if ev.Message != "" {
				turnError = ev.Message
			}
		}
	}

	if turnError != "" && len(messages) == 0 {
		return "", usage, fmt.Errorf("codex turn failed: %s", turnError)
	}

	if len(messages) == 0 {
		return "", usage, fmt.Errorf("codex returned no agent messages")
	}

	// Return the last agent message (most recent response).
	return messages[len(messages)-1], usage, nil
}

// Compile-time checks.
var (
	_ Provider          = (*codexCliProvider)(nil)
	_ decision.IOperator = (*codexCliProvider)(nil)
)
