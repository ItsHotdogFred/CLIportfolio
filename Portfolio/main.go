package main

import (
	"fmt"
	"os"
	"encoding/json"
	"net/http"
	"strings"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	gowiki "github.com/trietmn/go-wiki"
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
	fileViewMode bool           // true if viewing a file
	fileViewport viewport.Model // dedicated viewport for file viewing
	fileContent  string         // content of the file being viewed
}

var headerstyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

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
	// Handle file view mode
	if m.fileViewMode {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "q", "esc":
				m.fileViewMode = false
				m.fileContent = ""
				m.fileViewport = viewport.Model{}
				return m, nil
			}
		case tea.WindowSizeMsg:
			headerHeight := lipgloss.Height(m.fileHeaderView())
			footerHeight := lipgloss.Height(m.fileFooterView())
			verticalMarginHeight := headerHeight + footerHeight
			m.fileViewport.Width = msg.Width
			m.fileViewport.Height = msg.Height - verticalMarginHeight
			m.fileViewport.YPosition = headerHeight
		}
		var fileCmd tea.Cmd
		m.fileViewport, fileCmd = m.fileViewport.Update(msg)
		return m, fileCmd
	}
	switch msg := msg.(type) {

	// Is it a key press?
	case tea.KeyMsg:

		// Cool, what was the actual key pressed?
		switch msg.String() {

		// These keys should exit the program.
		case "ctrl+c":
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
				s += "\nName\n------\n"
				for _, entry := range entries {
					s += fmt.Sprintf("%s\n", entry.Name())
				}
				m.text = s
				m.input.Reset()

			} else if inputValue == "help" {
				m.text = "Just get good honestly"
				m.input.Reset()
			} else if inputValue == "clear" {
				//m.history = []string{}    // Clear history
				m.clihistory = []string{} // Clear terminal output
				m.viewport.SetContent("") // Clear viewport content
				m.input.Reset()
				// Don't append 'clear' to clihistory
				break
			} else if len(inputValue) >= 4 && inputValue[:4] == "cat " {
				content, err := os.ReadFile(m.directory + "/" + inputValue[4:])
				if err != nil {
					m.text = fmt.Sprintf("Error reading file: %v", err)
					m.input.Reset()
					break
				}
				m.fileContent = string(content)
				m.fileViewMode = true
				m.fileViewport = viewport.New(80, 24)
				m.fileViewport.SetContent(m.fileContent)
				m.fileViewport.YPosition = lipgloss.Height(m.fileHeaderView())
				m.input.Reset()
			} else if inputValue == "joke" {
				m.input.Reset()
				client := &http.Client{}
				req, err := http.NewRequest("GET", "https://icanhazdadjoke.com/", nil)
				if err != nil {
					m.text = fmt.Sprintf("Error creating request: %v", err)
					m.input.Reset()
					break
				}
				req.Header.Set("Accept", "application/json")
				resp, err := client.Do(req)
				if err != nil {
					m.text = fmt.Sprintf("Error fetching joke: %v", err)
				} else {
					defer resp.Body.Close()
					
					var jokeData struct {
						ID     string `json:"id"`
						Joke   string `json:"joke"`
						Status int    `json:"status"`
					}
					
					if err := json.NewDecoder(resp.Body).Decode(&jokeData); err != nil {
						m.text = fmt.Sprintf("Error parsing joke: %v", err)
					} else {
						m.text = jokeData.Joke
					}
				}
				if err != nil {
					m.text = fmt.Sprintf("Error fetching joke: %v", err)
				}

			} else if len(inputValue) >= 5 && inputValue[:5] == "wiki " {
				m.input.Reset()
				query := inputValue[4:] // Get everything after 'wiki '
				if query == "" {
					m.text = "Please provide a search term."
				} else {
					// Perform wiki search
					m.text = "Searching Wikipedia for: " + query
					search_result, err := gowiki.Summary(query, 5, -1, false, true)
					if err != nil {
						m.text = "Error fetching Wikipedia summary: " + err.Error()
					} else {
						m.text = "\n" + search_result
					}
				}
			} else {
				m.text += " is not a valid command, try running help for commands"



				// Pressing U for some reason takes the user to the top of the viewport



				m.input.Reset()
			}
			// Only append to clihistory if not just cleared
			if inputValue != "clear" {
				m.clihistory = append(m.clihistory, m.text)
			}

			// Update viewport content and scroll to bottom
			if m.ready {
				// displayDir := m.directory
				// if displayDir == "." || displayDir == "" {
				// 	displayDir = "~"
				// } else if strings.HasPrefix(displayDir, "./") {
				// 	displayDir = "~" + displayDir[1:]
				// }

				var contentBuilder strings.Builder
				contentBuilder.WriteString(m.headerView())

				for i := 0; i < len(m.clihistory); i++ {
					contentBuilder.WriteString(m.clihistory[i])
					contentBuilder.WriteString("\n")
				}

				contentBuilder.WriteString("\n")
				// Remove this line from viewport content:
				// contentBuilder.WriteString("guest@fred:" + displayDir + "$\n")
				// Remove: contentBuilder.WriteString(m.input.View())
				// Remove: contentBuilder.WriteString("\n")

				m.viewport.SetContent(contentBuilder.String())
				m.viewport, _ = m.viewport.Update(msg)
				m.viewport.SetYOffset(1 << 16)
			}

		case "up", "ctrl+p":
			// Only allow input history navigation, do not move viewport
			if len(m.history) > 0 {
				if m.historyIndex == -1 {
					m.historyIndex = 0
				} else if m.historyIndex < len(m.history)-1 {
					m.historyIndex++
				}
				m.input.SetValue(m.history[len(m.history)-1-m.historyIndex])
			}
		case "down", "ctrl+n":
			// Only allow input history navigation, do not move viewport
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
		}

		var contentBuilder strings.Builder
		contentBuilder.WriteString(m.headerView())

		for i := 0; i < len(m.clihistory); i++ {
			contentBuilder.WriteString(m.clihistory[i])
			contentBuilder.WriteString("\n")
		}

		contentBuilder.WriteString("\n")

		m.viewport.SetContent(contentBuilder.String())
		// (No auto-scroll here)
	} else {
		m.viewport.SetContent("Initializing terminal size...")
	}

	// Only update viewport for non-arrow key events (scroll wheel, etc.)
	// Arrow keys are handled above for input history only
	// Prevent viewport from moving with up/down keys by filtering them out
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "up", "down", "ctrl+p", "ctrl+n":
			// Don't update viewport for up/down keys
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
	}
	m.viewport, _ = m.viewport.Update(msg)
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
	title := headerstyle.Render(header) + "\n"
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
	if m.fileViewMode {
		if !m.ready {
			return "Initializing file viewer..."
		}
		return fmt.Sprintf("%s\n%s\n%s\n(Press 'q' or 'esc' to exit)", m.fileHeaderView(), m.fileViewport.View(), m.fileFooterView())
	}
	if !m.ready {
		return "Initializing terminal size..."
	}
	// Show viewport (history/output) and input field at the bottom
	// Add lipgloss color to "guest@fred"
	promptStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	prompt := promptStyle.Render("guest@fred:")
	return m.viewport.View() + "\n" + prompt + (func() string {
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

// File view header/footer for pager mode
func (m model) fileHeaderView() string {
	b := lipgloss.RoundedBorder()
	b.Right = "├"
	titleStyle := lipgloss.NewStyle().BorderStyle(b).Padding(0, 1)
	title := titleStyle.Render("File Viewer")
	line := strings.Repeat("─", max(0, m.fileViewport.Width-lipgloss.Width(title)))
	return lipgloss.JoinHorizontal(lipgloss.Center, title, line)
}

func (m model) fileFooterView() string {
	b := lipgloss.RoundedBorder()
	b.Left = "┤"
	infoStyle := lipgloss.NewStyle().BorderStyle(b).Padding(0, 1)
	info := infoStyle.Render(fmt.Sprintf("%3.f%%", m.fileViewport.ScrollPercent()*100))
	line := strings.Repeat("─", max(0, m.fileViewport.Width-lipgloss.Width(info)))
	return lipgloss.JoinHorizontal(lipgloss.Center, line, info)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Index is skipping the first one in the history
// ls isn't being recoreded in history

//
//
//
//	Add arrow key handling for viewport scrolling but make it so you need to press tab to switch between input history and viewport scrolling
//	See if the Text Area in the bubbles library if writing can be disabed https://github.com/charmbracelet/bubbles
//
//
