/**
 * Gemini CLI - Version 1.1 (Go Edition)
 * - Features: Chat, Model Listing (-l), Model Selection (-m)
 * - Supports: Multi-line input with \, Word-wrapping, Table safety.
 */

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/google/generative-ai-go/genai"
	"golang.org/x/term"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

const (
	DefaultModel     = "gemini-3-flash-preview"
	HistoryFile      = "chat_history.json"
	MaxWidth         = 100
	OffsetFactor     = 0.20
	MaxHistoryLength = 20
)

type HistoryEntry struct {
	Role  string   `json:"role"`
	Parts []string `json:"parts"`
}

var currentColumn = 0

func main() {
	// --- 1. Flag Definition ---
	listFlag := flag.Bool("l", false, "List available models")
	flag.BoolVar(listFlag, "list", false, "List available models")

	modelFlag := flag.String("m", DefaultModel, "Specific model ID to use")
	flag.StringVar(modelFlag, "model", DefaultModel, "Specific model ID to use")

	flag.Parse()

	ctx := context.Background()
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		fmt.Println("\033[31mError: GEMINI_API_KEY environment variable not set.\033[0m")
		return
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		return
	}
	defer client.Close()

	// --- 2. Route to List or Chat ---
	if *listFlag {
		listModels(ctx, client)
		return
	}

	runChat(ctx, client, *modelFlag)
}

// --- Logic: Model Listing ---

func listModels(ctx context.Context, client *genai.Client) {
	padding := getPadding()
	fmt.Printf("\n%s\033[33mFetching available Gemini models...\033[0m\n\n", padding)
	fmt.Printf("%s%-35s %s\n", padding, "ID", "DISPLAY NAME")
	fmt.Printf("%s%s\n", padding, strings.Repeat("-", 70))

	iter := client.ListModels(ctx)
	for {
		m, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			break
		}

		// Filter for generation models
		supportsGen := false
		for _, method := range m.SupportedGenerationMethods {
			if method == "generateContent" {
				supportsGen = true
				break
			}
		}

		if supportsGen {
			shortID := strings.TrimPrefix(m.Name, "models/")
			fmt.Printf("%s\033[32m%-35s\033[0m %s\n", padding, shortID, m.DisplayName)
		}
	}
	fmt.Println()
}

// --- Logic: Main Chat Loop ---

func runChat(ctx context.Context, client *genai.Client, modelName string) {
	model := client.GenerativeModel(modelName)
	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text("You are a helpful, professional, and grounded AI assistant. Respond in the user's language.")},
	}

	history := loadHistory()
	displayHistory(history)

	session := model.StartChat()
	session.History = convertToGenAIHistory(history)

	// Handle SIGINT (Ctrl+C)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Printf("\n%s\033[33m[System] Bye!\033[0m\n", getPadding())
		os.Exit(0)
	}()

	fmt.Printf("%s\033[33m--- Gemini Pro Go v1.1 (%s) ---\033[0m\n", getPadding(), modelName)

	for {
		// REMOVED: The \n\n\033[1A "floating" trick
		fmt.Print("\n" + getPadding() + "\033[36mYou > \033[0m")

		var fullInput []string
		for {
			line := readLine()

			if strings.HasSuffix(line, "\\") {
				fullInput = append(fullInput, strings.TrimSuffix(line, "\\"))
				fmt.Print(getPadding() + "\033[2m  ... \033[0m")
				continue
			}

			fullInput = append(fullInput, line)
			break
		}

		input := strings.Join(fullInput, "\n")

		if strings.ToLower(input) == "exit" || strings.ToLower(input) == "quit" {
			fmt.Printf("%s\033[33m[System] Bye!\033[0m\n", getPadding())
			break
		}

		if input != "" {
		// Sanitize input to ensure it is valid UTF-8
		// strings.ToValidUTF8 replaces invalid byte sequences with the second argument
		sanitizedInput := strings.ToValidUTF8(input, "") 

		handleStream(ctx, session, sanitizedInput, &history)
		}
	}
}

// --- Logic & Rendering ---

func getPadding() string {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		width = 120
	}
	available := width - MaxWidth
	padSize := int(float64(available) * OffsetFactor)
	if padSize < 0 {
		padSize = 0
	}
	return strings.Repeat(" ", padSize)
}

func renderFormatted(text string, isDimmed bool, skipPadding bool) {
	padding := getPadding()
	if isDimmed {
		fmt.Print("\033[2m")
	}

	// Handle tables separately: Check for pipes or horizontal lines
	if strings.Contains(text, "|") || strings.Contains(text, "+-") {
		lines := strings.Split(text, "\n")
		for _, line := range lines {
			fmt.Print(padding + line + "\n")
		}
		return
	}

	// Convert Markdown bold (**) to ANSI bold escape codes before processing
	processedText := strings.ReplaceAll(text, "**", "\033[1m")

	lines := strings.Split(processedText, "\n")
	for i, line := range lines {
		// Apply vertical spacing and padding
		if i > 0 {
			fmt.Print("\n" + padding)
			currentColumn = 0
		} else if !skipPadding {
			fmt.Print(padding)
		}

		// Split by spaces but preserve them by using Split instead of Fields.
		// This prevents words like "Da" and "du" from merging into "Dadu".
		parts := strings.Split(line, " ")
		for j, part := range parts {
			// Calculate the visible length of the word by stripping hidden ANSI codes
			visiblePart := stripANSI(part)
			partLen := len(visiblePart)

			// Wrap line if it exceeds MaxWidth
			if currentColumn+partLen > MaxWidth && currentColumn > 0 {
				fmt.Print("\n" + padding)
				currentColumn = 0
			}

			// Output the word (including ANSI formatting)
			fmt.Print(part)
			currentColumn += partLen

			// Re-insert the space that was removed by Split(" "), 
			// unless it's the very last word of the line.
			if j < len(parts)-1 {
				fmt.Print(" ")
				currentColumn++
			}
		}
	}
	// Always reset formatting at the end to prevent "bleeding" styles
	fmt.Print("\033[0m")
}

// stripANSI removes common terminal escape sequences to get the actual visible string length.
func stripANSI(str string) string {
	s := strings.ReplaceAll(str, "\033[1m", "") // Bold
	s = strings.ReplaceAll(s, "\033[0m", "") // Reset
	s = strings.ReplaceAll(s, "\033[2m", "") // Dimmed
	return s
}

func displayHistory(history []HistoryEntry) {
	if len(history) == 0 {
		return
	}
	padding := getPadding()
	fmt.Printf("\n%s\033[2m\033[33m--- Previous Conversation ---\033[0m\n", padding)
	for _, entry := range history {
		roleColor := "\033[35m"
		roleName := "Gemini"
		if entry.Role == "user" {
			roleColor = "\033[36m"
			roleName = "You"
		}
		fmt.Printf("%s%s%s:\033[0m ", padding, roleColor, roleName)
		currentColumn = 0
		if len(entry.Parts) > 0 {
			renderFormatted(entry.Parts[0], true, true)
		}
		fmt.Print("\n\n")
	}
	fmt.Printf("%s\033[2m\033[33m------------------------------\033[0m\n\n", padding)
}


func handleStream(ctx context.Context, session *genai.ChatSession, input string, history *[]HistoryEntry) {
	padding := getPadding()
	fmt.Printf("\n%s\033[2m\033[35m[Thinking...]\033[0m", padding)

	iter := session.SendMessageStream(ctx, genai.Text(input))

	// Clear the thinking indicator before starting output
	fmt.Print("\r\033[K")
	fmt.Printf("%s\033[35mGemini:\033[0m ", padding)

	currentColumn = 0
	var fullResponse strings.Builder
	
	for {
		resp, err := iter.Next()
		if err != nil {
			// Catch specific API errors like safety blocks or token limits
			if err != iterator.Done {
				fmt.Printf("\n%s\033[31m[API Error]: %v\033[0m\n", padding, err)
			}
			break
		}

		// Handle cases where the model returns no candidates due to safety filters
		if len(resp.Candidates) == 0 {
			fmt.Printf("\n%s\033[33m[System]: No response generated. This might be due to safety filters.\033[0m\n", padding)
			break
		}

		for _, cand := range resp.Candidates {
			if cand.Content != nil {
				for _, part := range cand.Content.Parts {
					if txt, ok := part.(genai.Text); ok {
						content := string(txt)
						fullResponse.WriteString(content)
						renderFormatted(content, false, true)
					}
				}
			}
		}
	}

	fmt.Print("\n")

	// Only save to history if we actually received a response
	if fullResponse.Len() > 0 {
		*history = append(*history, HistoryEntry{Role: "user", Parts: []string{input}})
		*history = append(*history, HistoryEntry{Role: "model", Parts: []string{fullResponse.String()}})
		saveHistory(*history)
	}
}

// --- Helpers & Persistence ---

func readLine() string {
	reader := bufio.NewReader(os.Stdin)
	text, _ := reader.ReadString('\n')
	return strings.TrimSpace(text)
}

func loadHistory() []HistoryEntry {
	data, err := os.ReadFile(HistoryFile)
	if err != nil {
		return []HistoryEntry{}
	}
	var h []HistoryEntry
	json.Unmarshal(data, &h)
	return h
}

func saveHistory(h []HistoryEntry) {
	if len(h) > MaxHistoryLength {
		h = h[len(h)-MaxHistoryLength:]
	}
	data, _ := json.MarshalIndent(h, "", "  ")
	os.WriteFile(HistoryFile, data, 0644)
}

func convertToGenAIHistory(h []HistoryEntry) []*genai.Content {
	var gh []*genai.Content
	for _, entry := range h {
		gh = append(gh, &genai.Content{
			Role:  entry.Role,
			Parts: []genai.Part{genai.Text(entry.Parts[0])},
		})
	}
	return gh
}
