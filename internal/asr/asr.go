package asr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const generationURL = "https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation"

type Client struct {
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}
}

type RecognizeRequest struct {
	Model      string                 `json:"model"`
	Input      recognizeInput         `json:"input"`
	Parameters recognizeParameters    `json:"parameters"`
	Resources  []any                  `json:"resources"`
}

type recognizeInput struct {
	Messages []recognizeMessage `json:"messages"`
}

type recognizeMessage struct {
	Role    string           `json:"role"`
	Content []recognizeAudio `json:"content"`
}

type recognizeAudio struct {
	Audio string `json:"audio"`
}

type recognizeParameters struct {
	Format string `json:"format"`
}

type recognizeResponse struct {
	Output struct {
		Text string `json:"text"`
	} `json:"output"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// Recognize 使用阿里云 FUN-ASR（fun-asr-realtime）将 Base64 音频转为文字。
// audioBase64 为纯 Base64 字符串（不含 data URI 前缀）。
func (c *Client) Recognize(apiKey, audioBase64, mimeType, format string) (string, error) {
	if strings.TrimSpace(apiKey) == "" {
		return "", fmt.Errorf("请先配置阿里云 API Key")
	}
	if strings.TrimSpace(audioBase64) == "" {
		return "", fmt.Errorf("音频数据为空")
	}

	if mimeType == "" {
		mimeType = "audio/wav"
	}
	if format == "" {
		format = mimeToFormat(mimeType)
	}

	dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, audioBase64)
	body := RecognizeRequest{
		Model: "fun-asr-realtime",
		Input: recognizeInput{
			Messages: []recognizeMessage{
				{
					Role: "user",
					Content: []recognizeAudio{
						{Audio: dataURI},
					},
				},
			},
		},
		Parameters: recognizeParameters{Format: format},
		Resources:  []any{},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, generationURL, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-DashScope-SSE", "disable")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("调用语音识别接口失败: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("语音识别接口返回 %d: %s", resp.StatusCode, string(raw))
	}

	var result recognizeResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("解析识别结果失败: %w", err)
	}
	if result.Code != "" && result.Code != "Success" {
		return "", fmt.Errorf("语音识别失败: %s", result.Message)
	}
	text := strings.TrimSpace(result.Output.Text)
	if text == "" {
		return "", fmt.Errorf("未识别到有效语音内容")
	}
	return text, nil
}

func mimeToFormat(mime string) string {
	switch {
	case strings.Contains(mime, "wav"):
		return "wav"
	case strings.Contains(mime, "mp3"):
		return "mp3"
	case strings.Contains(mime, "webm"):
		return "webm"
	case strings.Contains(mime, "ogg"):
		return "opus"
	default:
		return "wav"
	}
}
