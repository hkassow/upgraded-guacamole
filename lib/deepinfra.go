package lib

// DeepInfra client: streams chat completions from the hosted Qwen models, and reads recipes
// from photos with the vision model.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

// var (not const) so tests can point it at a fake server
var deepInfraChatURL = "https://api.deepinfra.com/v1/openai/chat/completions"

const (
	deepInfraTextModel = "Qwen/Qwen3-235B-A22B-Instruct-2507" // .09 in .55 out
	//deepInfraTextModel = "Qwen/Qwen3-32B"				 // .08 in .28 out

	deepInfraVisionModel = "Qwen/Qwen3-VL-235B-A22B-Instruct" // .20 in .88 out
	//deepInfraVisionModel = "Qwen/Qwen3-VL-30B-A3B-Instruct" // .15 in .60 out
)

// ---------------------------------------------------------------------------
// OpenAI-compatible request/response shapes (DeepInfra speaks this dialect)
// ---------------------------------------------------------------------------

type diContentPart struct {
	Type     string      `json:"type"`
	Text     string      `json:"text,omitempty"`
	ImageURL *diImageURL `json:"image_url,omitempty"`
}

type diImageURL struct {
	URL string `json:"url"`
}

type diMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // string OR []diContentPart
}

type diChatRequest struct {
	Model         string           `json:"model"`
	Messages      []diMessage      `json:"messages"`
	Temperature   float64          `json:"temperature"`
	MaxTokens     int              `json:"max_tokens,omitempty"`
	Stream        bool             `json:"stream"`
	StreamOptions *diStreamOptions `json:"stream_options,omitempty"`
}

type diStreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type diUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// one "data:" event of a streamed chat completion
type diStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"` // "length" means the reply was cut off
	} `json:"choices"`
	Usage *diUsage `json:"usage,omitempty"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// DeepInfra replies are streamed, so a slow-but-working generation can be told apart from a stuck
// one: a call fails when no data arrives for a while, not after a fixed total time.
var (
	// waiting for the first data covers prompt processing, which is slow for big photo prompts
	deepInfraFirstDataTimeout = 3 * time.Minute
	// once tokens are flowing, this long without any means the stream is stuck
	deepInfraIdleTimeout = 60 * time.Second
	// hard backstop for a reply that keeps streaming forever
	deepInfraMaxDuration = 15 * time.Minute
	// how often to log progress on a long reply
	deepInfraProgressInterval = 60 * time.Second
)

// Caps a runaway reply (e.g. the model repeating itself). A large recipe's JSON is a few thousand
// tokens; hitting this shows up as finish_reason "length".
const deepInfraMaxTokens = 8192

var deepInfraClient = &http.Client{} // timeouts come from the request context, see callDeepInfra

func callDeepInfra(req diChatRequest) (string, error) {
	apiKey, err := LoadSecret("DEEPINFRA_API_KEY")
	if err != nil {
		return "", fmt.Errorf("loading DeepInfra API key: %w", err)
	}

	req.Stream = true
	req.StreamOptions = &diStreamOptions{IncludeUsage: true}
	if req.MaxTokens == 0 {
		req.MaxTokens = deepInfraMaxTokens
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshaling deepinfra request: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), deepInfraMaxDuration)
	defer cancel()

	// cancels the request if the stream goes quiet; reset every time data arrives
	var stalled atomic.Bool
	idle := time.AfterFunc(deepInfraFirstDataTimeout, func() {
		stalled.Store(true)
		cancel()
	})
	defer idle.Stop()

	httpReq, err := http.NewRequestWithContext(ctx, "POST", deepInfraChatURL, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	start := time.Now()
	receivedData := false
	failure := func(what string, err error) error {
		elapsed := time.Since(start).Round(time.Second)
		if stalled.Load() {
			wait := deepInfraIdleTimeout
			if !receivedData {
				wait = deepInfraFirstDataTimeout
			}
			return fmt.Errorf("deepinfra %s: no data received for %s (%s total): %w", what, wait, elapsed, err)
		}
		return fmt.Errorf("deepinfra %s after %s: %w", what, elapsed, err)
	}

	resp, err := deepInfraClient.Do(httpReq)
	if err != nil {
		return "", failure("request failed", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		return "", fmt.Errorf("deepinfra returned status %d: %s", resp.StatusCode, string(data))
	}

	var content strings.Builder
	var finishReason string
	var usage *diUsage
	lastProgressLog := start

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64<<10), 4<<20)
	for scanner.Scan() {
		receivedData = true
		idle.Reset(deepInfraIdleTimeout)

		// server-sent events: "data: {...}" lines, blank separators, ": comment" keep-alives
		data, ok := strings.CutPrefix(scanner.Text(), "data:")
		if !ok {
			continue
		}
		data = strings.TrimSpace(data)
		if data == "[DONE]" {
			break
		}

		var chunk diStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return "", fmt.Errorf("decoding deepinfra stream: %w (raw: %s)", err, data)
		}
		if chunk.Error != nil {
			return "", fmt.Errorf("deepinfra error: %s", chunk.Error.Message)
		}
		if chunk.Usage != nil {
			usage = chunk.Usage
		}
		for _, choice := range chunk.Choices {
			content.WriteString(choice.Delta.Content)
			if choice.FinishReason != nil && *choice.FinishReason != "" {
				finishReason = *choice.FinishReason
			}
		}

		if time.Since(lastProgressLog) >= deepInfraProgressInterval {
			lastProgressLog = time.Now()
			log.Printf("deepinfra: %s still generating after %s (%d characters so far)",
				req.Model, time.Since(start).Round(time.Second), content.Len())
		}
	}
	if err := scanner.Err(); err != nil {
		return "", failure("stream failed", err)
	}
	if content.Len() == 0 {
		return "", fmt.Errorf("deepinfra returned no content (finish_reason: %q)", finishReason)
	}

	// log how long calls take so the timeouts can be tuned against real recipes
	usageText := "unknown"
	if usage != nil {
		usageText = fmt.Sprintf("%d prompt + %d completion", usage.PromptTokens, usage.CompletionTokens)
	}
	log.Printf("deepinfra: %s took %s (tokens: %s, finish_reason: %q)",
		req.Model, time.Since(start).Round(time.Second), usageText, finishReason)
	if finishReason == "length" {
		log.Printf("deepinfra: %s reply was cut off at the token limit, the recipe JSON is probably incomplete", req.Model)
	}

	return content.String(), nil
}

func ParseRecipeCallDeepInfra(recipeText string) (*RecipeParsed, error) {
	return extractRecipeJSON(recipeText)
}

func imageDataURI(imageBase64 string) string {
	mime := "image/jpeg"

	if raw, err := base64.StdEncoding.DecodeString(imageBase64); err == nil {
		detected := http.DetectContentType(raw)
		if len(detected) >= len("image/") && detected[:6] == "image/" {
			mime = detected
		}
	}

	return fmt.Sprintf("data:%s;base64,%s", mime, imageBase64)
}

func ParseRecipeImageCallDeepInfra(images []string) (*RecipeParsed, error) {
	transcript, err := TranscribeRecipeImages(images)
	if err != nil {
		return &RecipeParsed{}, err
	}
	return extractRecipeJSON(transcript)
}

// TranscribeRecipeImages reads the recipe text from photos with the vision model. This is the slow
// and expensive step of an image job, so the worker saves its result for retries.
func TranscribeRecipeImages(images []string) (string, error) {
	if len(images) == 0 {
		return "", fmt.Errorf("no images provided")
	}

	content := make([]diContentPart, 0, len(images)+1)
	for _, img := range images {
		content = append(content, diContentPart{
			Type:     "image_url",
			ImageURL: &diImageURL{URL: imageDataURI(img)},
		})
	}
	content = append(content, diContentPart{
		Type: "text",
		Text: "Transcribe this recipe. If multiple images are provided, treat them as pages of the same recipe.",
	})

	transcript, err := callDeepInfra(diChatRequest{
		Model:       deepInfraVisionModel,
		Temperature: 0,
		Messages: []diMessage{
			{Role: "system", Content: recipeImageSystemPrompt},
			{
				Role:    "user",
				Content: content,
			},
		},
	})

	if err != nil {
		return "", fmt.Errorf("image transcription failed: %w", err)
	}
	if strings.TrimSpace(transcript) == "" {
		return "", fmt.Errorf("image transcription returned no text")
	}

	return transcript, nil
}
