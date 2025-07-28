package main

import (
	"fmt"
	"os"

	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	input        textinput.Model
	viewport     viewport.Model
	ready        bool
	startingpath string
	directory    string
	text         string
	history      []string
	historyIndex int // -1 means not browsing history
	clihistory   []string
}

var clistyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("205"))

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = ""
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 20
	vp := viewport.New(80, 24) // Default size, will be updated on WindowSizeMsg
	return model{
		input:        ti,
		viewport:     vp,
		startingpath: ".",
		directory:    ".",
		text:         "nothing yet...",
		historyIndex: -1,
	}
}

// Init implements the tea.Model interface.
func (m model) Init() tea.Cmd {
	// Force a synthetic WindowSizeMsg to initialize viewport size
	return tea.Batch(
		textinput.Blink,
		func() tea.Msg {
			return tea.WindowSizeMsg{Width: 80, Height: 24}
		},
	)
}

func main() {
	p := tea.NewProgram(
		initialModel(),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd tea.Cmd
	)
	switch msg := msg.(type) {

	// Is it a key press?
	case tea.KeyMsg:

		// Cool, what was the actual key pressed?
		switch msg.String() {

		// These keys should exit the program.
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			inputValue := m.input.Value()
			text := inputValue
			m.text = text
			m.history = append(m.history, text)
			m.historyIndex = -1 // Reset history navigation on new entry
			if len(inputValue) >= 3 && inputValue[:3] == "cd " {

				if m.text != "" && m.text != "nothing yet..." {
					dirToAdd := m.text[3:] // get everything after 'cd '
					if dirToAdd == ".." {
						// If the command is 'cd ..', go up one directory
						// We need to remove the last directory from the current path
						parts := strings.Split(m.directory, "/")
						if len(parts) > 1 {
							m.directory = strings.Join(parts[:len(parts)-1], "/")
						} else {
							m.directory = m.startingpath
						}
					} else {
						if validatePath(m.directory + "/" + dirToAdd) {
							m.directory = m.directory + "/" + dirToAdd
						} else {
							m.text = "Invalid directory: " + dirToAdd
						}
					}
				}
				m.input.Reset()

			} else if inputValue == "ls" {
				// If the command is 'ls', use validatePath to list files in the current directory
				s := ""
				entries, err := os.ReadDir(m.directory)
				if err != nil {
					s += fmt.Sprintf("Error reading directory: %v\n", err)
				}
				for _, entry := range entries {
					s += fmt.Sprintf("File: %s, IsDir: %t\n", entry.Name(), entry.IsDir())
				}
				m.text = s
				m.input.Reset()

			} else if inputValue == "help" {
				m.text = "Just get good honestly"
				m.input.Reset()
			} else {
				m.text += " is not a valid command, try running help for commands"
				m.input.Reset()
			}
			m.clihistory = append(m.clihistory, m.text) // Store command in clihistory

			// Update viewport content and scroll to bottom
			if m.ready {
				displayDir := m.directory
				if displayDir == "." || displayDir == "" {
					displayDir = "~"
				} else if strings.HasPrefix(displayDir, "./") {
					displayDir = "~" + displayDir[1:]
				}

				var contentBuilder strings.Builder
				contentBuilder.WriteString(m.headerView())

				for i := 0; i < len(m.clihistory); i++ {
					contentBuilder.WriteString(clistyle.Render(m.clihistory[i]))
					contentBuilder.WriteString("\n")
				}

				contentBuilder.WriteString("\n")
				// Remove this line from viewport content:
				// contentBuilder.WriteString("guest@fred:" + displayDir + "$\n")
				// Remove: contentBuilder.WriteString(m.input.View())
				// Remove: contentBuilder.WriteString("\n")

				m.viewport.SetContent(contentBuilder.String())
				m.viewport, cmd = m.viewport.Update(msg)
				m.viewport.SetYOffset(1 << 16)
			}

		case "up":
			if len(m.history) > 0 {
				if m.historyIndex == -1 {
					m.historyIndex = 0
				} else if m.historyIndex < len(m.history)-1 {
					m.historyIndex++
				}
				m.input.SetValue(m.history[len(m.history)-1-m.historyIndex])
			}
		case "down":
			if len(m.history) > 0 {
				if m.historyIndex > 0 {
					m.historyIndex--
					m.input.SetValue(m.history[len(m.history)-1-m.historyIndex])
				} else if m.historyIndex == 0 {
					m.historyIndex = -1
					m.input.SetValue("")
				}
			}
		}
	case tea.WindowSizeMsg:
		headerHeight := lipgloss.Height(m.headerView())
		footerHeight := lipgloss.Height(m.footerView())
		verticalMarginHeight := headerHeight + footerHeight

		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - verticalMarginHeight
		m.viewport.YPosition = headerHeight
		m.ready = true
	}

	if m.ready {
		displayDir := m.directory
		if displayDir == "." || displayDir == "" {
			displayDir = "~"
		} else if strings.HasPrefix(displayDir, "./") {
			displayDir = "~" + displayDir[1:]
		}

		var contentBuilder strings.Builder
		contentBuilder.WriteString(m.headerView())

		for i := 0; i < len(m.clihistory); i++ {
			contentBuilder.WriteString(clistyle.Render(m.clihistory[i]))
			contentBuilder.WriteString("\n")
		}

		contentBuilder.WriteString("\n")
		// Remove this line from viewport content:
		// contentBuilder.WriteString("guest@fred:" + displayDir + "$\n")
		// Remove: contentBuilder.WriteString(m.input.View())
		// Remove: contentBuilder.WriteString("\n")

		m.viewport.SetContent(contentBuilder.String())
		// (No auto-scroll here)
	} else {
		m.viewport.SetContent("Initializing terminal size...")
	}

	m.viewport, cmd = m.viewport.Update(msg)
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m model) headerView() string {
	header := `
███████╗██████╗ ███████╗██████╗      ██████╗██╗     ██╗
██╔════╝██╔══██╗██╔════╝██╔══██╗    ██╔════╝██║     ██║
█████╗  ██████╔╝█████╗  ██║  ██║    ██║     ██║     ██║
██╔══╝  ██╔══██╗██╔══╝  ██║  ██║    ██║     ██║     ██║
██║     ██║  ██║███████╗██████╔╝    ╚██████╗███████╗██║
╚═╝     ╚═╝  ╚═╝╚══════╝╚═════╝      ╚═════╝╚══════╝╚═╝

		`
	title := clistyle.Render(header) + "\n"
	// line := strings.Repeat("─", max(0, m.viewport.Width-lipgloss.Width(title)))
	// return lipgloss.JoinHorizontal(lipgloss.Center, title, line)
	return title
}

// footerView returns a simple footer string.
func (m model) footerView() string {
	// info := infoStyle.Render(fmt.Sprintf("%3.f%%", m.viewport.ScrollPercent()*100))
	// line := strings.Repeat("─", max(0, m.viewport.Width-lipgloss.Width(info)))
	// return lipgloss.JoinHorizontal(lipgloss.Center, line, info)
	return "\n"
}

func (m model) View() string {
	if !m.ready {
		return "Initializing terminal size..."
	}
	// Show viewport (history/output) and input field at the bottom
	return m.viewport.View() + "\n" + "guest@fred:" + (func() string {
		displayDir := m.directory
		if displayDir == "." || displayDir == "" {
			return "~"
		} else if strings.HasPrefix(displayDir, "./") {
			return "~" + displayDir[1:]
		}
		return displayDir
	}()) + "$" + m.input.View() + "\n"
}

// validatePath checks if the given path exists and is a directory.
// It returns true if the path is valid, false otherwise.
func validatePath(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// Index is skipping the first one in the history
// ls isn't being recoreded in history
