package coach

import (
	"context"
	"errors"
	"fmt"
	"os"
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
	return Response{}, fmt.Errorf("%s: %w", operation, err)
}
