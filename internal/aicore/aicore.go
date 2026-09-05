// Package aicore provides reusable AI helpers (speech transcription and text
// polishing) shared by the REST API routes and the MCP server, so the
// correction logic lives in exactly one place.
package aicore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

// Polishing mode constants.
const (
	ModeMedium = "medium"
	ModeStrong = "strong"
	ModeCustom = "custom"
	ModeVoice  = "voice"
)

// AIConfig holds the resolved provider settings for a single user.
type AIConfig struct {
	Enabled bool
	BaseURL string
	APIKey  string
	Model   string
}

// ChatMessage is a single chat completion turn.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

const mediumPrompt = "你是一个日记文本整理助手。请对用户提供的日记文本进行以下处理：\n1) 去除口语化的语气词（如「哈哈」、「呃」、「嗯」、「嘛」、「吧」的冗余使用）、多余的标点和感叹；\n2) 纠正明显的错别字、语法错误和语病；\n3) 根据内容含义自动分段，使段落结构清晰、阅读流畅；\n4) 保留原文的核心事实、情感表达和个人口吻，不要增加新的事件或情节；\n5) 输出只返回整理后的文本本身，不要包含解释、说明或额外文字。"

const strongPrompt = "你是一个日记文本改写助手。请对用户提供的日记文本进行深度重组和精简：\n1) 主动重组句子结构，使其更通顺、逻辑更清晰；\n2) 去除一切冗余、重复、流水账式的描述，保留最有意义的内容；\n3) 让表达更书面、更精炼，但仍保持自然和个人的语气；\n4) 适当补充过渡语句，使段落之间衔接自然；\n5) 不要虚构新的事件或情节；\n6) 输出只返回改写后的文本本身，不要包含解释、说明或额外文字。"

// voicePrompt is a preset specialised for turning raw speech-to-text output
// into clean, well-formatted diary prose.
const voicePrompt = "你是一个日记语音整理助手。用户会提供一段由语音转写（或类似口语输入）得到的原始文本。请进行以下处理：\n1) 去除口语化的语气词与冗余 filler（如「呃」「嗯」「啊」「那个」「然后呢」「就是说」「对吧」的反复出现），但保留自然口吻；\n2) 修正明显的同音错别字与转写错误（如「在」误为「再」、「做」误为「作」等），依据上下文判断；\n3) 在合适位置补全标点符号（逗号、句号、问号、感叹号），并按语义合理断句；\n4) 自动分段：以主题或时间转换为界，将长文本拆分为若干段落，使结构清晰、便于阅读；\n5) 保留原文的核心事实、人名、地点、事件与个人情感，不要虚构或添加新情节；\n6) 若原文已较规范，仅做最小必要修正，不要过度改写；\n7) 输出只返回整理后的文本本身，不要包含解释、说明或额外文字。"

// PolishSystemPrompt returns the system prompt for the requested polish mode.
func PolishSystemPrompt(mode, customPrompt string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", ModeMedium:
		return mediumPrompt, nil
	case ModeStrong:
		return strongPrompt, nil
	case ModeVoice:
		return voicePrompt, nil
	case ModeCustom:
		cp := strings.TrimSpace(customPrompt)
		if cp == "" {
			return "", errors.New("prompt is required for custom mode")
		}
		return cp, nil
	default:
		return "", fmt.Errorf("unsupported polish mode %q", mode)
	}
}

// ChatComplete sends a (non-streaming) chat completion and returns the
// assistant message content.
func ChatComplete(ctx context.Context, cfg AIConfig, messages []ChatMessage, stream bool) (string, error) {
	if !cfg.Enabled || cfg.APIKey == "" || cfg.BaseURL == "" || cfg.Model == "" {
		return "", errors.New("AI service is not configured")
	}
	baseURL := strings.TrimSuffix(cfg.BaseURL, "/")
	url := baseURL + "/v1/chat/completions"

	reqBody := map[string]any{
		"model":    cfg.Model,
		"messages": messages,
		"stream":   stream,
	}
	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("build AI request: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return "", fmt.Errorf("create AI request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("AI request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bodyText, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("AI returned status %d: %s", resp.StatusCode, string(bodyText))
	}

	var aiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&aiResp); err != nil {
		return "", fmt.Errorf("decode AI response: %w", err)
	}
	if len(aiResp.Choices) == 0 {
		return "", errors.New("AI returned no choices")
	}
	return strings.TrimSpace(aiResp.Choices[0].Message.Content), nil
}

// Transcribe sends audio to the upstream speech-to-text endpoint and returns
// the transcribed text.
func Transcribe(ctx context.Context, cfg AIConfig, audio []byte, filename, contentType, language, model, prompt string) (string, error) {
	if !cfg.Enabled || cfg.APIKey == "" || cfg.BaseURL == "" {
		return "", errors.New("speech recognition is not configured")
	}
	if model == "" {
		model = "whisper-1"
	}
	baseURL := strings.TrimSuffix(cfg.BaseURL, "/")
	transcribeURL := baseURL + "/v1/audio/transcriptions"

	var requestBody bytes.Buffer
	boundary := "diarum-boundary-" + fmt.Sprintf("%d", time.Now().UnixNano())
	writer := multipart.NewWriter(&requestBody)
	if err := writer.SetBoundary(boundary); err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	if err := writer.WriteField("model", model); err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	if language != "" {
		if err := writer.WriteField("language", language); err != nil {
			return "", fmt.Errorf("build request: %w", err)
		}
	}
	if prompt != "" {
		if err := writer.WriteField("prompt", prompt); err != nil {
			return "", fmt.Errorf("build request: %w", err)
		}
	}
	if err := writer.WriteField("response_format", "json"); err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	if err := writer.WriteField("temperature", "0"); err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	origName := filename
	if origName == "" {
		origName = "audio.webm"
	}
	part, err := writer.CreateFormFile("file", origName)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	if _, err := part.Write(audio); err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, transcribeURL, &requestBody)
	if err != nil {
		return "", fmt.Errorf("create speech request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if contentType != "" {
		req.Header.Set("X-Upload-Content-Type", contentType)
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("speech request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bodyText, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("speech provider returned status %d: %s", resp.StatusCode, string(bodyText))
	}

	var transcript struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&transcript); err != nil {
		return "", fmt.Errorf("decode speech response: %w", err)
	}
	return transcript.Text, nil
}
