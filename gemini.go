/**
 * Gemini CLI - Version 2.6
 * - Logic: Centralized formatting and word-wrap for stream and history.
 * - Stability: Safe UTF-8/Emoji handling using runes.
 */

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"golang.org/x/term"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

const (
	DefaultModel     = "gemini-3-flash-preview"
	HistoryFile      = "chat_history.json"
	MaxLineChars     = 120
	MaxHistoryLength = 20
)

type HistoryEntry struct {
	Role  string   `json:"role"`
	Parts []string `json:"parts"`
}

func main() {
	modelFlag := flag.String("m", DefaultModel, "Model ID")
	flag.Parse()

	ctx := context.Background()
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		fmt.Println("\033[31mError: GEMINI_API_KEY not set.\033[0m")
		return
	}

	// Initialize the Gemini client
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		return
	}
	defer client.Close()

	runChat(ctx, client, *modelFlag)
}

func runChat(ctx context.Context, client *genai.Client, modelName string) {
	model := client.GenerativeModel(modelName)
	// Provide system instructions for formatting preferences
	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text("Helpful AI. Use [31m Red, [32m Green, [33m Yellow, [34m Blue, [35m Magenta, [36m Cyan and Markdown.")},
	}

	history := loadHistory()
	displayHistory(history)

	session := model.StartChat()
	session.History = convertToGenAIHistory(history)

	fmt.Printf("%s\033[33m--- Gemini CLI v2.6 (%s) ---\033[0m\n", getPadding(), modelName)

	for {
		fmt.Print("\n" + getPadding() + "\033[36mYou > \033[0m")
		input := readLine()
		if input == "exit" || input == "quit" { break }
		if input != "" {
			handleStream(ctx, session, input, &history)
		}
	}
}

// printFormatted handles the rendering logic for both live stream and history playback
func printFormatted(text string, indent string) {
	buffer := ""
	boldActive, italicActive := false, false
	lineLength := 0
	runes := []rune(text)

	for i := 0; i < len(runes); i++ {
		char := runes[i]

		// Handle manual newlines from the model
		if char == '\n' {
			fmt.Print("\n" + indent)
			lineLength = 0
			continue
		}

		buffer += string(char)

		// Process ANSI Color Sequences (formatted as [XXm)
		if strings.HasPrefix(buffer, "[") && strings.HasSuffix(buffer, "m") {
			fmt.Print("\033" + buffer)
			buffer = ""
			continue
		}

		// Process Markdown Formatting (Bold and Italic)
		if buffer == "**" {
			if !boldActive { fmt.Print("\033[1m"); boldActive = true } else { fmt.Print("\033[22m"); boldActive = false }
			buffer = ""
			continue
		} else if buffer == "*" {
			// Lookahead check to distinguish between * and **
			if i+1 < len(runes) && runes[i+1] == '*' { continue }
			if !italicActive { fmt.Print("\033[3m"); italicActive = true } else { fmt.Print("\033[23m"); italicActive = false }
			buffer = ""
			continue
		}

		// Text output with Word-Wrap logic
		if len(buffer) > 0 && !strings.HasPrefix(buffer, "[") && !strings.HasPrefix(buffer, "*") {
			// Wrap at space/tab if the 120 character limit is reached
			if lineLength >= MaxLineChars && (char == ' ' || char == '\t') {
				fmt.Print("\n" + indent)
				lineLength = 0
			} else {
				fmt.Print(buffer)
				lineLength++
			}
			buffer = ""
		}
	}
	fmt.Print("\033[0m") // Reset formatting at the end of the message
}

func handleStream(ctx context.Context, session *genai.ChatSession, input string, history *[]HistoryEntry) {
	p := getPadding()
	indent := p + "        " // Alignment offset for "Gemini: " label
	fmt.Printf("\n%s\033[35m[Thinking...]\033[0m", p)
	
	iter := session.SendMessageStream(ctx, genai.Text(input))
	fmt.Print("\r\033[K") // Clear the [Thinking...] line
	fmt.Printf("%s\033[35mGemini:\033[0m ", p)
	
	var fullResponse strings.Builder
	boldActive, italicActive := false, false
	lineLength := 0
	buffer := ""

	for {
		resp, err := iter.Next()
		if err == iterator.Done { break }
		if err != nil { break }
		
		for _, cand := range resp.Candidates {
			if cand.Content != nil {
				for _, part := range cand.Content.Parts {
					if txt, ok := part.(genai.Text); ok {
						chunk := string(txt)
						fullResponse.WriteString(chunk)
						
						runes := []rune(chunk)
						for i := 0; i < len(runes); i++ {
							char := runes[i]
							if char == '\n' {
								fmt.Print("\n" + indent); lineLength = 0; continue
							}
							buffer += string(char)
							
							// Color check
							if strings.HasPrefix(buffer, "[") && strings.HasSuffix(buffer, "m") {
								fmt.Print("\033" + buffer); buffer = ""; continue
							}
							// Markdown check
							if buffer == "**" {
								if !boldActive { fmt.Print("\033[1m"); boldActive = true } else { fmt.Print("\033[22m"); boldActive = false }
								buffer = ""; continue
							} else if buffer == "*" {
								if i+1 < len(runes) && runes[i+1] == '*' { continue }
								if !italicActive { fmt.Print("\033[3m"); italicActive = true } else { fmt.Print("\033[23m"); italicActive = false }
								buffer = ""; continue
							}
							// Visible char output
							if len(buffer) > 0 && !strings.HasPrefix(buffer, "[") && !strings.HasPrefix(buffer, "*") {
								if lineLength >= MaxLineChars && (char == ' ' || char == '\t') {
									fmt.Print("\n" + indent); lineLength = 0
								} else {
									fmt.Print(buffer); lineLength++
								}
								buffer = ""
							}
						}
					}
				}
			}
		}
	}
	fmt.Println()
	
	// Save the interaction to history
	*history = append(*history, HistoryEntry{Role: "user", Parts: []string{input}})
	*history = append(*history, HistoryEntry{Role: "model", Parts: []string{fullResponse.String()}})
	saveHistory(*history)
}

// --- UTILITIES ---

// getPadding calculates left margin based on terminal width and the 120-char block
func getPadding() string {
	w, _, _ := term.GetSize(int(os.Stdout.Fd()))
	if w <= 0 { w = 140 }
	p := int(float64(w-MaxLineChars-10) * 0.1)
	if p < 0 { p = 0 }
	return strings.Repeat(" ", p)
}

func displayHistory(history []HistoryEntry) {
	p := getPadding()
	indent := p + "        "
	for _, e := range history {
		if e.Role == "user" {
			fmt.Printf("\n%s\033[36mYou > \033[0m%s\n", p, e.Parts[0])
		} else {
			fmt.Printf("%s\033[35mGemini:\033[0m ", p)
			printFormatted(e.Parts[0], indent)
			fmt.Println()
		}
	}
}

func readLine() string {
	r := bufio.NewReader(os.Stdin)
	l, _ := r.ReadString('\n')
	return strings.TrimSpace(l)
}

func loadHistory() []HistoryEntry {
	d, err := os.ReadFile(HistoryFile)
	if err != nil { return []HistoryEntry{} }
	var h []HistoryEntry
	json.Unmarshal(d, &h)
	return h
}

func saveHistory(h []HistoryEntry) {
	if len(h) > MaxHistoryLength { h = h[len(h)-MaxHistoryLength:] }
	data, _ := json.MarshalIndent(h, "", "  ")
	os.WriteFile(HistoryFile, data, 0644)
}

func convertToGenAIHistory(h []HistoryEntry) []*genai.Content {
	var gh []*genai.Content
	for _, e := range h {
		gh = append(gh, &genai.Content{Role: e.Role, Parts: []genai.Part{genai.Text(e.Parts[0])}})
	}
	return gh
}
