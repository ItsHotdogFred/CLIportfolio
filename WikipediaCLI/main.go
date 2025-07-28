package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"
	gowiki "github.com/trietmn/go-wiki"
)

var (
	titleStyle = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		b.Right = "├"
		return lipgloss.NewStyle().BorderStyle(b).Padding(0, 1)
	}()

	infoStyle = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		b.Left = "┤"
		return titleStyle.BorderStyle(b)
	}()

	summaryTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("12")).
				Bold(true).
				Underline(true).
				Margin(1, 0)

	summaryContentStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#ffffff")).
				PaddingLeft(2).
				MarginBottom(1)

	contentTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("12")).
				Bold(true).
				Underline(true).
				Margin(1, 0)

	contentStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#cccccc")).
			PaddingLeft(1)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ff0000")).
			Bold(true).
			Margin(1, 0)
)

type searchResultMsg struct {
	content string
	err     error
}

func searchCmd(query string) tea.Cmd {
	return func() tea.Msg {
		content, err := search(query)
		return searchResultMsg{content: content, err: err}
	}
}

type model struct {
	textinput    textinput.Model
	viewport     viewport.Model
	spinner      spinner.Model
	query        string
	searching    bool
	content      string
	ready        bool
	showViewport bool
}

const (
	host = "localhost"
	port = "234"
)

func startServer() {
	s, err := wish.NewServer(
		wish.WithAddress(net.JoinHostPort(host, port)),
		wish.WithHostKeyPath(".ssh/id_ed25519"),
		wish.WithMiddleware(
			bubbletea.Middleware(teaHandler),
			activeterm.Middleware(), // Bubble Tea apps usually require a PTY.
			logging.Middleware(),
		),
	)
	if err != nil {
		log.Error("Could not start server", "error", err)
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	log.Info("Starting SSH server", "host", host, "port", port)
	go func() {
		if err = s.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			log.Error("Could not start server", "error", err)
			done <- nil
		}
	}()

	<-done
	log.Info("Stopping SSH server")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer func() { cancel() }()
	if err := s.Shutdown(ctx); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
		log.Error("Could not stop server", "error", err)
	}
}

func teaHandler(s ssh.Session) (tea.Model, []tea.ProgramOption) {

	// Use the Bubble Tea renderer for SSH sessions
	// renderer := bubbletea.MakeRenderer(s) // Not used, can be added for advanced styling

	// Pass the renderer to the model if you want to use it for styling (optional)
	m := initialModel()
	// Optionally, you could set m.ready = true and m.showViewport = false to always start in search mode
	return m, []tea.ProgramOption{tea.WithAltScreen(), tea.WithMouseCellMotion()}
}

func main() {
	// Allow running as SSH server with a flag
	startServer()

	// p := tea.NewProgram(
	// 	initialModel(),
	// 	tea.WithAltScreen(),       // use the full size of the terminal
	// 	tea.WithMouseCellMotion(), // turn on mouse support for scrolling
	// )
	// if _, err := p.Run(); err != nil {
	// 	fmt.Printf("Alas, there's been an error: %v", err)
	// }
}

func initialModel() model {
	printWikiLogo()
	fmt.Println("Welcome to the Wikipedia CLI!")
	ti := textinput.New()
	ti.Placeholder = "Enter your search query (e.g., 'Python programming')"
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 50

	// Style the text input
	ti.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#21"))
	ti.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff"))
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))

	// Initialize spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return model{
		textinput:    ti,
		spinner:      s,
		searching:    false,
		content:      "",
		ready:        false,
		showViewport: false,
	}
}

func printWikiLogo() {
	fmt.Print(`
 _    _ _ _    _                _ _       
| |  | (_) |  (_)              | (_)      
| |  | |_| | ___ _ __   ___  __| |_  __ _ 
| |/\| | | |/ / | '_ \ / _ \/ _` + "`" + `| |/ _` + "`" + `| |
\  /\  / |   <| | |_) |  __/ (_| | | (_| |
 \/  \/|_|_|\_\_| .__/ \___|\__,_|_|\__,_|
				| |                       
				|_|     

`)
}

func search(query string) (string, error) {
	// Search for the Wikipedia page title
	search_result, err := gowiki.Summary(query, 5, -1, false, true)
	if err != nil {
		return errorStyle.Render("Error fetching summary: " + err.Error()), err
	}

	// Get the page
	page, err := gowiki.GetPage(query, -1, false, true)
	if err != nil {
		return errorStyle.Render("Error fetching page: " + err.Error()), err
	}

	// Get the content of the page
	content, err := page.GetContent()
	if err != nil {
		return errorStyle.Render("Error fetching content: " + err.Error()), err
	}

	// Format the output beautifully
	var result strings.Builder

	// Add summary section
	result.WriteString(summaryTitleStyle.Render("📋 SUMMARY"))
	result.WriteString("\n")
	result.WriteString(summaryContentStyle.Render(formatText(search_result, 80)))
	result.WriteString("\n\n")

	// Add content section
	result.WriteString(contentTitleStyle.Render("📖 FULL CONTENT"))
	result.WriteString("\n")
	result.WriteString(contentStyle.Render(formatText(content, 80)))

	return result.String(), nil
}

// formatText wraps text to specified width and adds proper spacing
func formatText(text string, width int) string {
	if len(text) == 0 {
		return "No content available."
	}

	// Clean up the text
	text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	text = strings.TrimSpace(text)

	// Split into paragraphs
	paragraphs := strings.Split(text, "\n\n")
	var formatted strings.Builder

	for i, paragraph := range paragraphs {
		if i > 0 {
			formatted.WriteString("\n\n")
		}

		// Wrap the paragraph
		words := strings.Fields(paragraph)
		var line strings.Builder

		for _, word := range words {
			if line.Len()+len(word)+1 > width {
				formatted.WriteString(line.String())
				formatted.WriteString("\n")
				line.Reset()
			}

			if line.Len() > 0 {
				line.WriteString(" ")
			}
			line.WriteString(word)
		}

		if line.Len() > 0 {
			formatted.WriteString(line.String())
		}
	}

	return formatted.String()
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			if m.showViewport {
				// Exit viewport mode and return to search
				m.showViewport = false
				m.textinput.Focus()
				return m, nil
			}
			return m, tea.Quit
		case "enter":
			if m.showViewport {
				return m, nil
			}
			if m.searching {
				return m, nil
			} else {
				m.searching = true
				m.query = m.textinput.Value()
				m.textinput.Reset() // Reset the input after search

				// Start the search command and spinner
				return m, tea.Batch(searchCmd(m.query), m.spinner.Tick)
			}
		}

	case searchResultMsg:
		m.searching = false
		if msg.err != nil {
			m.textinput.Focus()
			return m, nil
		}

		// Set up viewport with content and default size
		m.content = msg.content
		m.viewport = viewport.New(80, 24) // Default terminal size
		m.viewport.SetContent(m.content)
		m.showViewport = true
		m.ready = true

		return m, nil

	case tea.WindowSizeMsg:
		if m.showViewport {
			headerHeight := lipgloss.Height(m.headerView())
			footerHeight := lipgloss.Height(m.footerView())
			verticalMarginHeight := headerHeight + footerHeight

			if !m.ready {
				m.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
				m.viewport.YPosition = headerHeight
				m.viewport.SetContent(m.content)
				m.ready = true
			} else {
				m.viewport.Width = msg.Width
				m.viewport.Height = msg.Height - verticalMarginHeight
			}
		}
	}

	// Update spinner when searching
	if m.searching {
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	if m.showViewport && m.ready {
		// Handle keyboard and mouse events in the viewport
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	} else if !m.showViewport && !m.searching {
		m.textinput, cmd = m.textinput.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if m.showViewport {
		if !m.ready {
			return "\n  Loading Wikipedia content..."
		}
		return fmt.Sprintf("%s\n%s\n%s", m.headerView(), m.viewport.View(), m.footerView())
	}

	// Style the search interface
	searchTitle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("12")).
		Bold(true).
		Render("🔍 Wikipedia Search")

	var instructions string
	if m.searching {
		instructions = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffaa00")).
			Bold(true).
			Render(fmt.Sprintf("%s Searching Wikipedia for '%s'...", m.spinner.View(), m.query))
	} else {
		instructions = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Italic(true).
			Render("(Enter to search • Esc to quit)")
	}

	return fmt.Sprintf(
		"%s\n\n%s\n\n%s",
		searchTitle,
		m.textinput.View(),
		instructions,
	) + "\n"
}

func (m model) headerView() string {
	var title string
	if m.query != "" {
		title = titleStyle.Render(fmt.Sprintf("Wikipedia: %s", m.query))
	} else {
		title = titleStyle.Render("Wikipedia CLI")
	}
	line := strings.Repeat("─", max(0, m.viewport.Width-lipgloss.Width(title)))
	return lipgloss.JoinHorizontal(lipgloss.Center, title, line)
}

func (m model) footerView() string {
	info := infoStyle.Render(fmt.Sprintf("%3.f%% | ↑↓ scroll | ESC return to search | q quit", m.viewport.ScrollPercent()*100))
	line := strings.Repeat("─", max(0, m.viewport.Width-lipgloss.Width(info)))
	return lipgloss.JoinHorizontal(lipgloss.Center, line, info)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
