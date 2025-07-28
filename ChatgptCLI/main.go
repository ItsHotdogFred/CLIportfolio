package main

import (
	"context"
	"fmt"
	"os"
	"time" // It's good practice to include a timeout

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/otiai10/openaigo"
)

type model struct {
	textInput    textinput.Model
	apiKey       string
	submitted    bool
	response     string
	isStreaming  bool
	chatHistory  []openaigo.Message
	showResponse bool
}

type streamUpdateMsg string

var globalResponse string
var isStreamComplete bool
var systemPrompt string = "You're an AI chatbot which is currently being used in a terminal application as a CLI. You're name is Chat-CLI, Try to keep your response short and concise, but also informative. Markdown is not supported, but you can still use text to try prettify your response like using -------------- or *"

func getResponseCmd(question, apiKey string, chatHistory []openaigo.Message) tea.Cmd {
	return func() tea.Msg {
		// Reset global state
		globalResponse = ""
		isStreamComplete = false

		// Start streaming in a goroutine
		go func() {
			client := openaigo.NewClient(apiKey)

			streamCallback := func(response openaigo.ChatCompletionResponse, done bool, err error) {
				if done {
					isStreamComplete = true
					if err != nil {
						globalResponse += fmt.Sprintf("\n\nError: %v", err)
					}
					return
				}

				if len(response.Choices) > 0 {
					globalResponse += response.Choices[0].Delta.Content
				}
			}

			// Create messages array starting with system prompt
			messages := []openaigo.Message{
				{
					Role:    "system",
					Content: systemPrompt,
				},
			}

			// Add chat history
			messages = append(messages, chatHistory...)

			// Add new user question
			messages = append(messages, openaigo.Message{
				Role:    "user",
				Content: question,
			})

			request := openaigo.ChatCompletionRequestBody{
				Model:          openaigo.GPT4o,
				Messages:       messages,
				StreamCallback: streamCallback,
			}

			ctx := context.Background()
			_, err := client.ChatCompletion(ctx, request)
			if err != nil {
				globalResponse = fmt.Sprintf("Error starting the stream request: %v", err)
				isStreamComplete = true
			}
		}()

		return streamUpdateMsg("started")
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		return streamUpdateMsg("tick")
	})
}

func initialModel(apiKey string) model {

	ti := textinput.New()
	ti.Placeholder = "Enter your question here..."
	ti.Focus()
	ti.CharLimit = 2048
	ti.Width = 50

	return model{
		textInput:    ti,
		apiKey:       apiKey,
		submitted:    false,
		response:     "",
		isStreaming:  false,
		chatHistory:  []openaigo.Message{},
		showResponse: false,
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "enter":
			if !m.submitted && m.textInput.Value() != "" {
				m.submitted = true
				m.isStreaming = true
				m.showResponse = true
				question := m.textInput.Value()

				// Add the user's question to chat history
				m.chatHistory = append(m.chatHistory, openaigo.Message{
					Role:    "user",
					Content: question,
				})

				// Clear the text input for next question
				m.textInput.SetValue("")

				// Start the streaming and the ticker
				return m, tea.Batch(getResponseCmd(question, m.apiKey, m.chatHistory), tickCmd())
			} else if m.showResponse && !m.isStreaming && m.textInput.Value() != "" {
				// User wants to ask another question
				m.submitted = true
				m.isStreaming = true
				question := m.textInput.Value()

				// Add the user's question to chat history
				m.chatHistory = append(m.chatHistory, openaigo.Message{
					Role:    "user",
					Content: question,
				})

				// Clear the text input for next question
				m.textInput.SetValue("")

				// Start the streaming and the ticker
				return m, tea.Batch(getResponseCmd(question, m.apiKey, m.chatHistory), tickCmd())
			}
		}
	case streamUpdateMsg:
		if m.isStreaming {
			// Update the response with the current global response
			m.response = globalResponse

			if isStreamComplete {
				m.isStreaming = false
				m.submitted = false

				// Add the AI's response to chat history
				if m.response != "" {
					m.chatHistory = append(m.chatHistory, openaigo.Message{
						Role:    "assistant",
						Content: m.response,
					})
				}

				// Re-focus the text input for next question
				m.textInput.Focus()
				return m, nil
			}

			// Continue ticking to get updates
			return m, tickCmd()
		}
	}

	if !m.submitted || (!m.isStreaming && m.showResponse) {
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m model) View() string {
	var output string

	// Show welcome message only when starting (no chat history and not submitted)
	if len(m.chatHistory) == 0 && !m.submitted {
		output += "=====================================\n"
		output += "     Welcome to Chat-CLI Terminal    \n"
		output += "=====================================\n\n"
		output += "Your AI assistant is ready to help!\n"
		output += "Type your question below and press Enter.\n\n"
	}

	// Show chat history
	if len(m.chatHistory) > 0 {
		output += "Chat History:\n"
		output += "=============\n\n"

		for i, msg := range m.chatHistory {
			if msg.Role == "user" {
				output += fmt.Sprintf("You: %s\n\n", msg.Content)
			} else if msg.Role == "assistant" {
				output += fmt.Sprintf("AI: %s\n\n", msg.Content)
			}

			// Add separator between exchanges (but not after the last AI response if we're streaming)
			if i < len(m.chatHistory)-1 || (m.isStreaming && msg.Role == "user") {
				output += "---\n\n"
			}
		}
	}

	// Show current streaming response
	if m.isStreaming {
		if m.response == "" {
			output += "AI is thinking...\n\n"
		} else {
			output += fmt.Sprintf("AI: %s\n\n", m.response)
		}
		output += "(streaming...)\n\n"
	}

	// Show input area
	if !m.isStreaming {
		if m.showResponse {
			output += "Ask another question:\n"
		} else {
			output += "What's your question?\n"
		}
		output += fmt.Sprintf("%s\n\n", m.textInput.View())
		output += "(Press Enter to submit, Ctrl+C to quit)"
	} else {
		output += "(Press Ctrl+C to quit)"
	}

	return output
}

func main() {
	// It's a good practice to check if the API key is actually set.
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fmt.Println("Error: OPENAI_API_KEY environment variable not set.")
		return
	}

	p := tea.NewProgram(initialModel(apiKey))
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v", err)
		os.Exit(1)
	}
}
