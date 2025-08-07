package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

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
	"github.com/mdp/qrterminal/v3"
	gowiki "github.com/trietmn/go-wiki"
)

type model struct {
	input               textinput.Model
	viewport            viewport.Model
	ready               bool
	startingpath        string
	directory           string
	text                string
	history             []string
	historyIndex        int // -1 means not browsing history
	clihistory          []string
	fileViewMode        bool           // true if viewing a file
	fileViewport        viewport.Model // dedicated viewport for file viewing
	fileContent         string         // content of the file being viewed
	commandautocomplete []string
	fileautocomplete    []string
	autocompletelist    []string
}

var headerstyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

const (
	host = ""
	port = "2222"
)

const isServer = false // Set to true to enable the server, false to disable it

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
	m := initialModel()
	return m, []tea.ProgramOption{tea.WithAltScreen(), tea.WithMouseCellMotion()}
}

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = ""
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 60
	vp := viewport.New(0, 0)
	return model{
		input:               ti,
		viewport:            vp,
		startingpath:        ".",
		directory:           ".",
		text:                "nothing yet...",
		historyIndex:        -1,
		clihistory:          []string{headerView(), "Welcome to Fred's Portfolio CLI!\n\nNavigation:\n• Use scroll wheel or arrow keys to browse command history\n• Use Page Up/Page Down to navigate viewport\n• Type 'help' to see all available commands\n\nGet started with 'ls' to explore or 'help' for guidance."},
		commandautocomplete: []string{"help", "ls", "pwd", "cd", "cat", "whoami", "date", "version", "neofetch", "skills", "contact", "qr", "coinflip", "echo", "joke", "wiki", "clear", "exit", "yoda"},
	}
}

// Init implements the tea.Model interface.
func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func main() {
	if isServer {
		startServer()
	} else {
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
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
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
			exitInstructionHeight := 1 // For the "(Press 'q' or 'esc' to exit)" line
			verticalMarginHeight := headerHeight + footerHeight + exitInstructionHeight
			m.fileViewport.Width = msg.Width
			m.fileViewport.Height = msg.Height - verticalMarginHeight
			m.fileViewport.YPosition = 0
		}
		var fileCmd tea.Cmd
		m.fileViewport, fileCmd = m.fileViewport.Update(msg)
		return m, fileCmd
	}
	switch msg := msg.(type) {

	// Is it a key press?
	case tea.KeyMsg:

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
						parts := strings.Split(m.directory, "/")
						if len(parts) > 1 {
							m.directory = strings.Join(parts[:len(parts)-1], "/")
						} else {
							m.directory = m.startingpath
						}
					} else {
						// Check if trying to access a hidden directory
						if strings.HasPrefix(dirToAdd, ".") {
							m.text = "Access denied: Hidden directories are not accessible"
						} else if validatePath(m.directory + "/" + dirToAdd) {
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

				// Define styles for folders and files
				folderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#90EE90")) // Pastel green
				fileStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#DDA0DD"))   // Pastel purple

				for _, entry := range entries {
					// Skip hidden files/folders (those starting with .)
					if !strings.HasPrefix(entry.Name(), ".") {
						if entry.IsDir() {
							s += folderStyle.Render("📁 "+entry.Name()) + "\n"
						} else {
							s += fileStyle.Render("📄 "+entry.Name()) + "\n"
						}
					}
				}
				m.text = s
				m.input.Reset()

			} else if inputValue == "help" {
				m.text = `Available Commands:
===================

Navigation:
  pwd        - Show current directory
  ls         - List files and directories
  cd <dir>   - Change directory (use '..' to go up)
  cat <file> - View file contents in pager mode

System Info:
  whoami     - Show current user
  date       - Show current date
  version    - Show CLI version and build info
  neofetch   - Display system information with ASCII art

Portfolio:
  skills     - Show my technical skills
  contact    - Show contact information
  qr <text>  - Generate QR code for text
  coinflip   - Flip a coin (heads or tails)
Utilities:
  echo <text> - Echo back the provided text
  joke        - Get a random dad joke
  wiki <term> - Search Wikipedia for a term
  clear       - Clear the terminal output
  help        - Show this help message
  exit        - Exit the CLI

Navigation Tips:
  - Use up/down arrows to browse command history
  - Use Page Up/Page Down to navigate viewport
  - Press 'q' or 'esc' to exit file viewer
  - Use 'cd ..' to go to parent directory

Examples:
  cd Portfolio   - Navigate to Portfolio directory
  cat README.md  - View README file
  wiki golang    - Search Wikipedia for 'golang'
  echo Hello!    - Display 'Hello!'`
				m.input.Reset()
			} else if inputValue == "clear" {
				m.clihistory = []string{headerView()} // Reset history but keep header
				m.input.Reset()
				// Don't append 'clear' to clihistory
				break
			} else if len(inputValue) >= 4 && inputValue[:4] == "cat " {
				filename := inputValue[4:]
				// Check if trying to access a hidden file
				if strings.HasPrefix(filename, ".") {
					m.text = "Access denied: Hidden files are not accessible"
					m.input.Reset()
					break
				}
				content, err := os.ReadFile(m.directory + "/" + filename)
				if err != nil {
					m.text = fmt.Sprintf("Error reading file: %v", err)
					m.input.Reset()
					break
				}
				m.fileContent = string(content)
				m.fileViewMode = true
				// Initialize with proper size that will be updated by WindowSizeMsg
				m.fileViewport = viewport.New(80, 20)
				m.fileViewport.SetContent(m.fileContent)
				m.fileViewport.YPosition = 0
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
				query := inputValue[5:] // Get everything after 'wiki '
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
			} else if inputValue == "pwd" {
				m.text = "Current directory: " + m.directory
				m.input.Reset()
			} else if inputValue == "exit" {
				return m, tea.Quit
			} else if inputValue == "whoami" {
				m.text = "Current user: guest"
				m.input.Reset()
			} else if inputValue == "date" {
				m.text = "Current date: " + time.Now().Format("2006-01-02")
				m.input.Reset()
			} else if len(inputValue) >= 5 && inputValue[:5] == "echo " {
				m.text = "Echoing: " + inputValue[5:]
				m.input.Reset()
			} else if inputValue == "neofetch" {
				style := lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
				neofetchStyle := style
				m.text = neofetchStyle.Render(fmt.Sprintf(`
				.88888888:.              guest@fred-cli
			   88888888.88888.           -----------------
			 .8888888888888888.         OS: Fred's Portfolio CLI
			 888888888888888888         Kernel: Go Runtime
			 88' _`+"`"+`88'_  `+"`"+`88888         Uptime: Running since startup
			 88 88 88 88  88888         Shell: Go CLI v1.0
			 88_88_::_88_:88888         Resolution: Terminal Based
			 88:::,::,:::::8888         Terminal: Bubbles Tea
			 88`+"`"+`:::::::::`+"`"+`8888          CPU: %s
			.88  `+"`"+`::::`+"`"+`    8:88.        Memory: Efficient Go runtime
		   8888            `+"`"+`8:888.      Language: Go
		 .8888`+"`"+`             `+"`"+`888888.    Platform: %s
		.8888:..  .::.  ...:`+"`"+`8888888:.   
	   .8888.`+"`"+`     :`+"`"+`     `+"`"+`::`+"`"+`88:88888  
	  .8888        `+"`"+`         `+"`"+`.888:8888. 
	 888:8         .           888:88888 
   .888:88        .:           88:88888:
   8888888.       ::           88:888888 
   `+"`"+`.::.888.      ::          .88888888  
  .::::::.888.    ::         :::`+"`"+`8888`+"`"+`.  :
 ::::::::::.888   `+"`"+`         .::::::::::::
 ::::::::::::.8    `+"`"+`      .:8::::::::::::.
.::::::::::::::.        .:888:::::::::::::
:::::::::::::::88:.__..:88888::::::::::::`+"`"+`
 `+"`"+``+"`"+`.:::::::::::88888888888.88:::::::::  
	   `+"`"+``+"`"+`:::_:`+"`"+` -- `+"`"+``+"`"+` -`+"`"+`-`+"`"+` `+"`"+``+"`"+`:_::::      
`, runtime.GOARCH, runtime.GOOS))
				m.input.Reset()
			} else if inputValue == "version" {
				m.text += " verson 1.0.0, built with Go " + runtime.Version() + " on " + runtime.GOOS + "/" + runtime.GOARCH
				m.input.Reset()
			} else if inputValue == "skills" {
				m.text = `
Skills:
================
• Go Programming
• Terminal/CLI Development
• Web Development
• Game Development
• GDscript (Godot programming language)
• LLMS (Large Language Models)
`
				m.input.Reset()
			} else if inputValue == "contact" {
				m.text = "You can find me on:\n- GitHub:   github.com/ItsHotdogFred\n- Itch.io:  itshotdogfred.itch.io\n- Email:    cli@itsfred.dev"
				m.input.Reset()
			} else if len(inputValue) >= 3 && inputValue[:3] == "qr " {
				m.text = "Generating QR code for: " + inputValue[3:]
				qrterminal.Generate(inputValue[3:], qrterminal.L, os.Stdout)
				// Generate QR code to a string buffer instead of stdout
				var qrBuffer strings.Builder
				qrterminal.Generate(inputValue[3:], qrterminal.L, &qrBuffer)
				m.text = "QR code for: " + inputValue[3:] + "\n\n" + qrBuffer.String()
				m.input.Reset()
			} else if inputValue == "coinflip" {
				var num float64 = rand.Float64()
				if num < 0.5 {
					m.text += " Result: Heads"
				} else {
					m.text += " Result: Tails"
				}
				m.input.Reset()
			} else if len(inputValue) >= 5 && inputValue[:5] == "yoda " {
				text := inputValue[5:]
				words := strings.Fields(text)
				var yodaText string

				if len(words) < 2 {
					yodaText = text + ", mmm."
				} else {
					// Simple Yoda transformation: move some words around and add Yoda-isms
					var result []string

					// If sentence starts with "I am", change to "Am I"
					if len(words) >= 2 && strings.ToLower(words[0]) == "i" && strings.ToLower(words[1]) == "am" {
						result = append(result, strings.Title(words[1]), strings.ToLower(words[0]))
						result = append(result, words[2:]...)
					} else if len(words) >= 3 {
						// Move last word or phrase to beginning
						result = append(result, words[len(words)-1])
						result = append(result, words[:len(words)-1]...)
					} else {
						result = words
					}

					// Add Yoda-isms
					yodaisms := []string{", mmm.", ", yes.", ", hmm.", ", indeed."}
					ending := yodaisms[rand.Intn(len(yodaisms))]

					yodaText = strings.Join(result, " ") + ending
				}
				m.text = "Yoda says: " + yodaText
				m.input.Reset()
			} else {
				m.text += " is not a valid command, try running help for commands"
				m.input.Reset()
			}
			// Only append to clihistory if not just cleared
			if inputValue != "clear" {
				m.clihistory = append(m.clihistory, m.text)
			}

		// Autocomplete handling
		case "tab":
			if m.input.Value() == "" {
				return m, nil // No input to autocomplete
			}

			m.autocompletelist = []string{} // Reset autocompletion list

			entries, _ := os.ReadDir(m.directory)
			m.fileautocomplete = []string{}
			for _, entry := range entries {
				if !strings.HasPrefix(entry.Name(), ".") {
					m.fileautocomplete = append(m.fileautocomplete, entry.Name())
				}
			}

			// Perform autocompletion logic here
			// For example, you could suggest commands based on the current input
			m.autocompletelist = append(m.commandautocomplete, m.fileautocomplete...)
			input := m.input.Value()

			// Split input into words to get the current word being typed
			words := strings.Fields(input)
			if len(words) == 0 {
				return m, nil
			}

			currentWord := words[len(words)-1]
			var bestMatch string

			for _, option := range m.autocompletelist {
				// Check if option starts with the current word (prefix match)
				if strings.HasPrefix(strings.ToLower(option), strings.ToLower(currentWord)) {
					if bestMatch == "" {
						bestMatch = option
					} else {
						// If we have multiple matches, prefer files over commands
						// and among files, prefer the shortest complete filename
						isCurrentFile := false
						isBestFile := false

						// Check if current option is a file (contains extension)
						for _, fileOption := range m.fileautocomplete {
							if option == fileOption {
								isCurrentFile = true
								break
							}
						}

						// Check if best match is a file
						for _, fileOption := range m.fileautocomplete {
							if bestMatch == fileOption {
								isBestFile = true
								break
							}
						}

						// Prefer files over commands, and shorter files over longer files
						if isCurrentFile && !isBestFile {
							bestMatch = option
						} else if isCurrentFile && isBestFile && len(option) < len(bestMatch) {
							bestMatch = option
						} else if !isCurrentFile && !isBestFile && len(option) < len(bestMatch) {
							bestMatch = option
						}
					}
				}
			}

			// If we found a match, replace only the current word
			if bestMatch != "" {
				if len(words) == 1 {
					// Only one word, replace entirely
					m.input.SetValue(bestMatch)
				} else {
					// Multiple words, replace only the last word
					words[len(words)-1] = bestMatch
					m.input.SetValue(strings.Join(words, " "))
				}
				// Position cursor at the end of the input
				m.input.CursorEnd()
			}

			return m, nil

		case "up", "ctrl+p":
			// Navigate command history upward
			if len(m.history) > 0 {
				if m.historyIndex == -1 {
					m.historyIndex = 0
				} else if m.historyIndex < len(m.history)-1 {
					m.historyIndex++
				}
				m.input.SetValue(m.history[len(m.history)-1-m.historyIndex])
			}
			return m, nil

		case "down", "ctrl+n":
			// Navigate command history downward
			if len(m.history) > 0 {
				if m.historyIndex > 0 {
					m.historyIndex--
				} else if m.historyIndex == 0 {
					m.historyIndex = -1
					m.input.SetValue("")
				}
				if m.historyIndex >= 0 {
					m.input.SetValue(m.history[len(m.history)-1-m.historyIndex])
				}
			}
			return m, nil

		case "pageup":
			// Page up for viewport
			m.viewport.LineUp(m.viewport.Height / 2)
			return m, nil

		case "pagedown":
			// Page down for viewport
			m.viewport.LineDown(m.viewport.Height / 2)
			return m, nil
		}

	case tea.WindowSizeMsg:
		// The prompt line acts as the footer for the main view.
		// We account for the prompt line itself plus a newline.
		promptHeight := 2
		verticalMarginHeight := promptHeight

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
			m.viewport.YPosition = 0 // Viewport starts at the top
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMarginHeight
		}
	}

	// This block now correctly handles setting the viewport content
	// after any command is run or the window is resized.
	var contentBuilder strings.Builder
	for i := 0; i < len(m.clihistory); i++ {
		contentBuilder.WriteString(m.clihistory[i])
		contentBuilder.WriteString("\n")
	}
	m.viewport.SetContent(contentBuilder.String())

	// After an enter press, scroll to the bottom of the viewport
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "enter" {
		m.viewport.GotoBottom()
	}

	// Handle input and viewport updates
	// First check if it's a key message that should not move the viewport
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "ctrl+p", "ctrl+n", "u", "k", "b", "d", "f", "j", "pageup", "pagedown":
			// Don't update viewport for these keys, only update input
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
	}

	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func headerView() string {
	header := `
███████╗██████╗ ███████╗██████╗      ██████╗██╗     ██╗
██╔════╝██╔══██╗██╔════╝██╔══██╗    ██╔════╝██║     ██║
█████╗  ██████╔╝█████╗  ██║  ██║    ██║     ██║     ██║
██╔══╝  ██╔══██╗██╔══╝  ██║  ██║    ██║     ██║     ██║
██║     ██║  ██║███████╗██████╔╝    ╚██████╗███████╗██║
╚═╝     ╚═╝  ╚═╝╚══════╝╚═════╝      ╚═════╝╚══════╝╚═╝
        `
	title := headerstyle.Render(header)
	return title
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
	// Add lipgloss color to "guest@fred"
	promptStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	prompt := promptStyle.Render("guest@fred:")

	// Construct the prompt line which now acts as our footer
	promptLine := prompt + (func() string {
		displayDir := m.directory
		if displayDir == "." || displayDir == "" {
			return "~"
		} else if strings.HasPrefix(displayDir, "./") {
			return "~" + displayDir[1:]
		}
		return displayDir
	}()) + "$" + m.input.View()

	// Assemble the final view correctly. The header is now inside the viewport.
	return fmt.Sprintf("%s\n%s",
		m.viewport.View(),
		promptLine,
	)
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
