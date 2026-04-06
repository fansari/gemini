package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

func main() {
	ctx := context.Background()
	apiKey := os.Getenv("GEMINI_API_KEY")

	if apiKey == "" {
		fmt.Println("\033[31mError: GEMINI_API_KEY is not set.\033[0m")
		os.Exit(1)
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		fmt.Printf("\033[31mError creating client:\033[0m %v\n", err)
		return
	}
	defer client.Close()

	fmt.Println("\033[33mFetching available Gemini models...\033[0m\n")

	// Print Header
	fmt.Printf("%-35s %s\n", "ID", "DISPLAY NAME")
	fmt.Println(strings.Repeat("-", 70))

	// Iterate through available models
	iter := client.ListModels(ctx)
	for {
		model, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			fmt.Printf("\033[31mError fetching models:\033[0m %v\n", err)
			break
		}

		// Filter for models that support content generation
		supportsGen := false
		for _, method := range model.SupportedGenerationMethods {
			if method == "generateContent" {
				supportsGen = true
				break
			}
		}

		if supportsGen {
			// Remove the "models/" prefix for a cleaner ID
			shortID := strings.TrimPrefix(model.Name, "models/")
			fmt.Printf("\033[32m%-35s\033[0m %s\n", shortID, model.DisplayName)
		}
	}

	fmt.Println("\n\033[34mUsage:\033[0m Update ModelName in gemini.go with one of the IDs above.")
}
