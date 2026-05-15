// Minimal end-to-end example for the LLMHub Go SDK.
//
// Usage:
//
//	export LLMHUB_API_KEY=sk-llmh-...
//	export LLMHUB_BASE_URL=http://localhost:8080   # optional, defaults to public prod
//	go run ./example -model deepseek-chat -prompt "hello"
//	go run ./example -model deepseek-chat -prompt "hello" -stream
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	llmhub "github.com/llmhub/llmhub-go-sdk"
)

func main() {
	var (
		model  = flag.String("model", "deepseek-chat", "platform SKU id")
		prompt = flag.String("prompt", "Hi! In one sentence, why is the sky blue?", "user message")
		stream = flag.Bool("stream", false, "stream the response")
	)
	flag.Parse()

	apiKey := os.Getenv("LLMHUB_API_KEY")
	if apiKey == "" {
		log.Fatal("LLMHUB_API_KEY is required")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	c, err := llmhub.New(llmhub.Config{
		APIKey:  apiKey,
		BaseURL: os.Getenv("LLMHUB_BASE_URL"),
	})
	if err != nil {
		log.Fatalf("init: %v", err)
	}
	// Use a function so we can exit non-zero AFTER c.Close() flushes
	// the async usage-report queue. log.Fatalf would skip defers.
	exitCode := 0
	defer func() {
		_ = c.Close()
		os.Exit(exitCode)
	}()

	req := &llmhub.ChatRequest{
		Model: *model,
		Messages: []llmhub.ChatMessage{
			{Role: "user", Content: *prompt},
		},
	}

	if *stream {
		s, err := c.ChatStream(ctx, req)
		if err != nil {
			log.Printf("stream: %v", err); exitCode = 1; return
		}
		for chunk := range s.Chunks() {
			if chunk.Delta != nil {
				if str, ok := chunk.Delta.Content.(string); ok && str != "" {
					fmt.Print(str)
				}
			}
			if chunk.Done && chunk.Usage != nil {
				fmt.Printf("\n[usage: %d in / %d out]\n", chunk.Usage.PromptTokens, chunk.Usage.CompletionTokens)
			}
		}
		if err := s.Err(); err != nil {
			log.Printf("stream finished with error: %v", err); exitCode = 1; return
		}
		return
	}

	start := time.Now()
	resp, err := c.Chat(ctx, req)
	if err != nil {
		log.Printf("chat: %v", err); exitCode = 1; return
	}
	if len(resp.Choices) > 0 {
		fmt.Println(resp.Choices[0].Message.Content)
	}
	fmt.Printf("\n[%s · %d in / %d out tokens]\n",
		time.Since(start).Round(time.Millisecond),
		resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
}
