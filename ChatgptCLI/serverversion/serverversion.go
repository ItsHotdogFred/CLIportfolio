package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync" // Add this import
	"syscall"
	"time" // needed for tickCmd

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/logging"
	"github.com/otiai10/openaigo"
)

type model struct {
	textInput       textinput.Model
	passwordInput   textinput.Model
	apiKey          string
	submitted       bool
	response        string
	isStreaming     bool
	chatHistory     []openaigo.Message
	showResponse    bool
	authenticated   bool
	authFailed      bool
	currentResponse string // Add this to track current streaming response
}

type streamUpdateMsg string

var (
	globalResponse   string
	isStreamComplete bool
	systemPrompt     = "You're an AI chatbot which is currently being used in a terminal application as a CLI. Your name is Chat-CLI. Keep responses short and concise but informative. Markdown is not supported; use plain‑text separators or asterisks for clarity."
	serverPassword   = "ctk898"
	streamMutex      sync.Mutex // Add mutex for thread safety
)

// getResponseCmd starts the OpenAI streaming in a goroutine and returns a Tea message when started.
func getResponseCmd(question, apiKey string, chatHistory []openaigo.Message) tea.Cmd {
	return func() tea.Msg {
		streamMutex.Lock()
		globalResponse = ""
		isStreamComplete = false
		streamMutex.Unlock()

		go func() {
			client := openaigo.NewClient(apiKey)

			streamCallback := func(resp openaigo.ChatCompletionResponse, done bool, err error) {
				streamMutex.Lock()
				defer streamMutex.Unlock()

				if done {
					isStreamComplete = true
					if err != nil {
						globalResponse += fmt.Sprintf("\n\nError: %v", err)
					}
					return
				}
				if len(resp.Choices) > 0 {
					globalResponse += resp.Choices[0].Delta.Content
				}
			}

			// Build messages
			messages := []openaigo.Message{
				{Role: "system", Content: systemPrompt},
			}
			messages = append(messages, chatHistory...)
			messages = append(messages, openaigo.Message{Role: "user", Content: question})

			req := openaigo.ChatCompletionRequestBody{
				Model:          openaigo.GPT4o,
				Messages:       messages,
				StreamCallback: streamCallback,
			}

			_, err := client.ChatCompletion(context.Background(), req)
			if err != nil {
				globalResponse = fmt.Sprintf("Error starting stream: %v", err)
				isStreamComplete = true
			}
		}()

		return streamUpdateMsg("started")
	}
}

// tickCmd polls every 100ms to update the view while streaming.
func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		return streamUpdateMsg("tick")
	})
}

// initialModel sets up the Bubble Tea model.
func initialModel(apiKey string) model {
	ti := textinput.New()
	ti.Placeholder = "Enter your question here..."
	ti.CharLimit = 2048
	ti.Width = 60

	pi := textinput.New()
	pi.Placeholder = "Enter password..."
	pi.Focus()
	pi.EchoMode = textinput.EchoPassword
	pi.CharLimit = 256
	pi.Width = 30

	return model{
		textInput:     ti,
		passwordInput: pi,
		apiKey:        apiKey,
		submitted:     false,
		response:      "",
		isStreaming:   false,
		chatHistory:   []openaigo.Message{},
		showResponse:  false,
		authenticated: false,
		authFailed:    false,
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
			if !m.authenticated {
				// Handle password authentication
				if m.passwordInput.Value() == serverPassword {
					m.authenticated = true
					m.authFailed = false
					m.textInput.Focus()
					return m, nil
				} else {
					m.authFailed = true
					m.passwordInput.SetValue("")
					return m, nil
				}
			} else if !m.submitted && m.textInput.Value() != "" {
				// first submit
				m.submitted = true
				m.isStreaming = true
				m.showResponse = true
				m.currentResponse = "" // Reset current response
				question := m.textInput.Value()
				m.chatHistory = append(m.chatHistory, openaigo.Message{Role: "user", Content: question})
				m.textInput.SetValue("")
				return m, tea.Batch(getResponseCmd(question, m.apiKey, m.chatHistory), tickCmd())

			} else if m.showResponse && !m.isStreaming && m.textInput.Value() != "" {
				// subsequent questions
				m.submitted = true
				m.isStreaming = true
				m.currentResponse = "" // Reset current response
				question := m.textInput.Value()
				m.chatHistory = append(m.chatHistory, openaigo.Message{Role: "user", Content: question})
				m.textInput.SetValue("")
				return m, tea.Batch(getResponseCmd(question, m.apiKey, m.chatHistory), tickCmd())
			}
		}

	case streamUpdateMsg:
		if m.isStreaming {
			streamMutex.Lock()
			m.currentResponse = globalResponse
			complete := isStreamComplete
			streamMutex.Unlock()

			if complete {
				m.isStreaming = false
				m.submitted = false
				// save assistant reply
				if m.currentResponse != "" {
					m.chatHistory = append(m.chatHistory, openaigo.Message{Role: "assistant", Content: m.currentResponse})
				}
				m.response = m.currentResponse
				m.currentResponse = ""
				m.textInput.Focus()
				return m, nil
			}
			return m, tickCmd()
		}
	}

	// update inputs based on authentication state
	if !m.authenticated {
		pi, cmd := m.passwordInput.Update(msg)
		m.passwordInput = pi
		return m, cmd
	} else if !m.submitted || (!m.isStreaming && m.showResponse) {
		ti, cmd := m.textInput.Update(msg)
		m.textInput = ti
		return m, cmd
	}

	return m, nil
}

func (m model) View() string {
	var b strings.Builder

	if !m.authenticated {
		// Password authentication screen
		b.WriteString("=====================================\n")
		b.WriteString("     Chat-CLI Terminal Access        \n")
		b.WriteString("=====================================\n\n")

		if m.authFailed {
			b.WriteString("❌ Incorrect password. Please try again.\n\n")
		} else {
			b.WriteString("Please enter the password to access Chat-CLI:\n\n")
		}

		b.WriteString(m.passwordInput.View() + "\n\n")
		b.WriteString("(Enter = submit • Ctrl+C = quit)")
		return b.String()
	}

	// Header only on first run after authentication
	if len(m.chatHistory) == 0 && !m.submitted && !m.isStreaming {
		b.WriteString("=====================================\n")
		b.WriteString("     Welcome to Chat-CLI Terminal    \n")
		b.WriteString("=====================================\n\n")
		b.WriteString("Your AI assistant is ready!\nType your question and press Enter.\n\n")
	}

	// Print chat history (only completed messages)
	if len(m.chatHistory) > 0 {
		b.WriteString("Chat History:\n=============\n\n")
		for i, msg := range m.chatHistory {
			prefix := "You: "
			if msg.Role == "assistant" {
				prefix = "AI:  "
			}
			b.WriteString(prefix + msg.Content + "\n\n")
			// Add separator between messages, but not after the last one unless streaming
			if i < len(m.chatHistory)-1 {
				b.WriteString("---\n\n")
			}
		}

		// Add separator before streaming response if we have history
		if m.isStreaming {
			b.WriteString("---\n\n")
		}
	}

	// Show current streaming response (only once)
	if m.isStreaming {
		if m.currentResponse == "" {
			b.WriteString("AI is thinking...\n\n")
		} else {
			b.WriteString("AI: " + m.currentResponse + "\n\n")
		}
		b.WriteString("(streaming...)\n\n")
		b.WriteString("(Ctrl+C to quit)")
		return b.String()
	}

	// Input prompt (only when not streaming)
	if len(m.chatHistory) > 0 {
		b.WriteString("\nAsk another question:\n")
	} else {
		b.WriteString("What's your question?\n")
	}
	b.WriteString(m.textInput.View() + "\n\n")
	b.WriteString("(Enter = submit • Ctrl+C = quit)")

	return b.String()
}

// sshHandler wraps your Bubble Tea app so Wish can serve it over SSH.
func sshHandler(sess ssh.Session) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(sess, "Error: OPENAI_API_KEY not set")
		sess.Exit(1)
		return
	}

	p := tea.NewProgram(
		initialModel(apiKey),
		tea.WithInput(sess),
		tea.WithOutput(sess),
		tea.WithEnvironment(sess.Environ()),
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(sess, "Error running program: %v\n", err)
		sess.Exit(1)
	}
}

func main() {
	// generate a host key once: ssh-keygen -t ed25519 -f ssh_host_ed25519_key
	server, err := wish.NewServer(
		wish.WithAddress("0.0.0.0:2323"),
		wish.WithHostKeyPath("ssh_host_ed25519_key"),
		wish.WithMiddleware(
			logging.Middleware(),
			func(next ssh.Handler) ssh.Handler {
				return ssh.Handler(sshHandler)
			},
		),
	)
	if err != nil {
		fmt.Println("Failed to start SSH server:", err)
		os.Exit(1)
	}

	// graceful shutdown on SIGINT/SIGTERM
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signals
		fmt.Println("\nShutting down server...")
		server.Close()
		os.Exit(0)
	}()

	fmt.Println("Chat‑CLI SSH server listening on port 2323")
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
		fmt.Println("Server error:", err)
		os.Exit(1)
	}
}
