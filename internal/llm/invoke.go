package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"xengineer-voice-calendar/internal/tracing"
)

// invokeLLM 调用 DashScope 对话接口，并记录 tracing 事件（请求摘要、原始响应、提取文本）。
func (c *Client) invokeLLM(ctx context.Context, operation, apiKey, systemPrompt, userText string) (content string, rawBody []byte, err error) {
	ctx, span := tracing.StartSpan(ctx, operation)
	defer span.End()

	tracing.SetAttrs(ctx, map[string]string{
		"llm.model":       defaultModel,
		"llm.user_text":   userText,
		"llm.prompt_hint": truncateForTrace(systemPrompt, 512),
	})

	reqBody := chatRequest{Model: defaultModel}
	reqBody.Input.Messages = []chatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userText},
	}
	reqBody.Parameters.ResultFormat = "message"

	payload, err := json.Marshal(reqBody)
	if err != nil {
		tracing.RecordError(ctx, err)
		return "", nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, generationURL, bytes.NewReader(payload))
	if err != nil {
		tracing.RecordError(ctx, err)
		return "", nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	tracing.Event(ctx, "llm.request", map[string]string{
		"url":         generationURL,
		"payload_len": fmt.Sprintf("%d", len(payload)),
	})

	resp, err := c.httpClient.Do(req)
	if err != nil {
		tracing.RecordError(ctx, err)
		return "", nil, fmt.Errorf("调用大模型接口失败: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err = io.ReadAll(resp.Body)
	if err != nil {
		tracing.RecordError(ctx, err)
		return "", nil, err
	}

	tracing.Event(ctx, "llm.response_raw", map[string]string{
		"http_status":  fmt.Sprintf("%d", resp.StatusCode),
		"body":         string(rawBody),
		"body_len":     fmt.Sprintf("%d", len(rawBody)),
	})

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("大模型接口返回 %d: %s", resp.StatusCode, string(rawBody))
		tracing.RecordError(ctx, err)
		return "", rawBody, err
	}

	var result chatResponse
	if err := json.Unmarshal(rawBody, &result); err != nil {
		tracing.RecordError(ctx, err)
		return "", rawBody, fmt.Errorf("解析大模型响应失败: %w", err)
	}
	if result.Code != "" && result.Code != "Success" {
		err = fmt.Errorf("大模型调用失败: %s", result.Message)
		tracing.RecordError(ctx, err)
		return "", rawBody, err
	}

	content = ""
	if len(result.Output.Choices) > 0 {
		content = extractMessageContent(result.Output.Choices[0].Message.Content)
	} else {
		content = strings.TrimSpace(result.Output.Text)
	}

	tracing.Event(ctx, "llm.content_extracted", map[string]string{
		"content": content,
	})

	return content, rawBody, nil
}

func truncateForTrace(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
