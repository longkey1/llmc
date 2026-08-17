package llmc

import (
	"context"
	"fmt"
	"time"
)

// ToolChatter is implemented by providers that support function calling.
// It is an optional extension of Provider; callers detect support via a type
// assertion.
type ToolChatter interface {
	// ChatWithTools sends one request with the given tools available.
	// messages is the full conversation history including the newest user
	// message (unlike ChatWithHistory, there is no separate newMessage:
	// on later loop iterations the history ends with assistant tool calls
	// and their tool results, not a user message).
	// Canceling ctx aborts the in-flight API request.
	ChatWithTools(ctx context.Context, systemPrompt string, messages []Message, tools []ToolDef) (*TurnResult, error)
}

// ToolExecutor executes a single tool call. It must always return a result;
// failures are reported via ToolResult.IsError.
type ToolExecutor interface {
	Execute(ctx context.Context, call ToolCall) ToolResult
}

// LoopHooks lets the caller observe tool-loop progress (spinner, logging).
// All fields are optional.
type LoopHooks struct {
	// OnRequest is called right before each API request and returns a
	// function called when the request completes (e.g. start/stop a
	// spinner). Tool execution happens outside this window so interactive
	// confirmation prompts don't race with the spinner.
	OnRequest func() func()
	// OnToolCall is called before executing a tool call.
	OnToolCall func(call ToolCall)
	// OnToolDone is called after a tool call finished (or was denied).
	OnToolDone func(call ToolCall, result ToolResult)
}

// MaxToolIterations bounds the number of model requests in one tool loop.
const MaxToolIterations = 10

// RunToolLoop drives the tool-calling conversation loop: it sends the
// history to the model, executes any requested tool calls, feeds the results
// back, and repeats until the model answers with plain text. It returns the
// final text and every message generated during the loop (assistant tool
// calls, tool results, and the final assistant message) for session
// persistence. history must already end with the new user message.
func RunToolLoop(ctx context.Context, p ToolChatter, systemPrompt string, history []Message,
	tools []ToolDef, exec ToolExecutor, hooks LoopHooks) (string, []Message, error) {
	messages := make([]Message, len(history))
	copy(messages, history)

	var appended []Message

	for range MaxToolIterations {
		turn, err := chatOnce(ctx, p, systemPrompt, messages, tools, hooks)
		if err != nil {
			return "", nil, err
		}

		if len(turn.ToolCalls) == 0 {
			final := Message{
				Role:        "assistant",
				Content:     turn.Text,
				ToolCalls:   nil,
				ToolCallID:  "",
				ToolName:    "",
				ToolIsError: false,
				Timestamp:   time.Now(),
			}
			return turn.Text, append(appended, final), nil
		}

		assistantMsg := Message{
			Role:        "assistant",
			Content:     turn.Text,
			ToolCalls:   turn.ToolCalls,
			ToolCallID:  "",
			ToolName:    "",
			ToolIsError: false,
			Timestamp:   time.Now(),
		}
		messages = append(messages, assistantMsg)
		appended = append(appended, assistantMsg)

		for _, call := range turn.ToolCalls {
			if hooks.OnToolCall != nil {
				hooks.OnToolCall(call)
			}
			result := exec.Execute(ctx, call)
			if hooks.OnToolDone != nil {
				hooks.OnToolDone(call, result)
			}
			toolMsg := Message{
				Role:        "tool",
				Content:     result.Content,
				ToolCalls:   nil,
				ToolCallID:  result.ToolCallID,
				ToolName:    result.Name,
				ToolIsError: result.IsError,
				Timestamp:   time.Now(),
			}
			messages = append(messages, toolMsg)
			appended = append(appended, toolMsg)
		}
	}

	return "", nil, fmt.Errorf("tool loop exceeded %d iterations", MaxToolIterations)
}

// chatOnce performs one API request, wrapped in the OnRequest hook.
func chatOnce(ctx context.Context, p ToolChatter, systemPrompt string, messages []Message,
	tools []ToolDef, hooks LoopHooks) (*TurnResult, error) {
	if hooks.OnRequest != nil {
		done := hooks.OnRequest()
		defer done()
	}
	return p.ChatWithTools(ctx, systemPrompt, messages, tools)
}
