package coach

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/greg/telegram-ai-nutrition-coach-bot/internal/llm"
)

const llmTimeoutUserMessage = "⏱ The AI service took too long to respond. Please try again in a moment."

func isLLMTimeout(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, context.DeadlineExceeded) || os.IsTimeout(err)
}

func responseFromLLMError(err error, operation string) (Response, error) {
	if err == nil {
		return Response{}, nil
	}
	if isLLMTimeout(err) {
		return Response{Text: llmTimeoutUserMessage}, nil
	}
	if llm.IsRateLimited(err) {
		return Response{Text: llmRateLimitedMessage}, nil
	}
	status := llm.StatusCodeOf(err)
	if status > 0 {
		return Response{}, fmt.Errorf("%s: openrouter http %d: %w", operation, status, err)
	}
	return Response{}, fmt.Errorf("%s: %w", operation, err)
}
