/**
 * Gemini CLI - Version 2.5
 * - Fixed: History is now correctly formatted (Colors, Bold, Wrap).
 * - Refactored: Centralized formatting logic in printFormatted().
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

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer client.Close()

	runChat(ctx, client, *modelFlag)
}

func runChat(ctx context.Context, client *genai.Client, modelName string) {
	model := client.GenerativeModel(modelName)
	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text("Helpful AI. Use [31m Red, [32m Green, [33m Yellow, [34m Blue, [35m Magenta, [36m Cyan and Markdown.")},
	}

	history := loadHistory()
	displayHistory(history)

	session := model.StartChat()
	session.History = convertToGenAIHistory(history)

	fmt.Printf("%s\033[33m--- Gemini CLI v2.5 (%s) ---\033[0m\n", getPadding(), modelName)

	for {
		fmt.Print("\n" + getPadding() + "\033[36mYou > \033[0m")
		input := readLine()
		if input == "exit" || input == "quit" { break }
		if input != "" {
			handleStream(ctx, session, input, &history)
		}
	}
}

// printFormatted übernimmt das schicke Rendering für Stream UND History
func printFormatted(text string, indent string) {
	buffer := ""
	boldActive, italicActive := false, false
	lineLength := 0
	runes := []rune(text)

	for i := 0; i < len(runes); i++ {
		char := runes[i]

		if char == '\n' {
			fmt.Print("\n" + indent)
			lineLength = 0
			continue
		}

		buffer += string(char)

		// 1. Farben
		if strings.HasPrefix(buffer, "[") && strings.HasSuffix(buffer, "m") {
			fmt.Print("\033" + buffer)
			buffer = ""
			continue
		}

		// 2. Markdown
		if buffer == "**" {
			if !boldActive { fmt.Print("\033[1m"); boldActive = true } else { fmt.Print("\033[22m"); boldActive = false }
			buffer = ""
			continue
		} else if buffer == "*" {
			if i+1 < len(runes) && runes[i+1] == '*' { continue }
			if !italicActive { fmt.Print("\033[3m"); italicActive = true } else { fmt.Print("\033[23m"); italicActive = false }
			buffer = ""
			continue
		}

		// 3. Output & Wrap
		if len(buffer) > 0 && !strings.HasPrefix(buffer, "[") && !strings.HasPrefix(buffer, "*") {
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
	fmt.Print("\033[0m") // Reset am Ende jeder Nachricht
}

func handleStream(ctx context.Context, session *genai.ChatSession, input string, history *[]HistoryEntry) {
	p := getPadding()
	indent := p + "        "
	fmt.Printf("\n%s\033[35m[Thinking...]\033[0m", p)
	
	iter := session.SendMessageStream(ctx, genai.Text(input))
	fmt.Print("\r\033[K") 
	fmt.Printf("%s\033[35mGemini:\033[0m ", p)
	
	var fullResponse strings.Builder
	// Wir streamen jetzt einfach den Text in den Builder und rufen für die Anzeige 
	// eine leicht modifizierte Version unserer Logik auf (oder nutzen den Builder am Ende)
	// Für echtes Live-Streaming bauen wir die Logik hier direkt ein:
	
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
							if strings.HasPrefix(buffer, "[") && strings.HasSuffix(buffer, "m") {
								fmt.Print("\033" + buffer); buffer = ""; continue
							}
							if buffer == "**" {
								if !boldActive { fmt.Print("\033[1m"); boldActive = true } else { fmt.Print("\033[22m"); boldActive = false }
								buffer = ""; continue
							} else if buffer == "*" {
								if i+1 < len(runes) && runes[i+1] == '*' { continue }
								if !italicActive { fmt.Print("\033[3m"); italicActive = true } else { fmt.Print("\033[23m"); italicActive = false }
								buffer = ""; continue
							}
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
	
	*history = append(*history, HistoryEntry{Role: "user", Parts: []string{input}})
	*history = append(*history, HistoryEntry{Role: "model", Parts: []string{fullResponse.String()}})
	saveHistory(*history)
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

// --- RESTLICHE HELPER BLEIBEN GLEICH ---

func getPadding() string {
	w, _, _ := term.GetSize(int(os.Stdout.Fd()))
	if w <= 0 { w = 140 }
	p := int(float64(w-MaxLineChars-10) * 0.1)
	if p < 0 { p = 0 }
	return strings.Repeat(" ", p)
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
