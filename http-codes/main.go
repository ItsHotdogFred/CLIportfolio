package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// AppState defines the different states of our application.
type AppState int

const (
	StateGetURL AppState = iota
	StateChecking
	StateDone
)

type model struct {
	state     AppState
	textInput textinput.Model
	status    int
	err       error
}

// checkServer makes an HTTP GET request to the given URL and returns a message
// with the result. It's a tea.Cmd because it's an asynchronous operation.
func checkServer(url string) tea.Cmd {
	return func() tea.Msg {
		c := &http.Client{Timeout: 10 * time.Second}
		res, err := c.Get(url)

		if err != nil {
			return errMsg{err}
		}
		return statusMsg(res.StatusCode)
	}
}

// Custom message types for our application.
type (
	statusMsg int
	errMsg    struct{ err error }
)

// Error implements the error interface for our custom error message.
func (e errMsg) Error() string { return e.err.Error() }

// initialModel creates the initial state of our application.
func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "https://itsfred.dev"
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 50

	return model{
		state:     StateGetURL,
		textInput: ti,
	}
}

// Init is called when the application starts.
func (m model) Init() tea.Cmd {
	// Start the blinking cursor in the text input.
	return textinput.Blink
}

// Update handles all incoming messages and updates the model accordingly.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	// Handle key presses
	case tea.KeyMsg:
		switch m.state {
		// If we're in the input state, handle text input.
		case StateGetURL:
			switch msg.Type {
			case tea.KeyEnter:
				// When Enter is pressed, get the URL and start checking.
				m.state = StateChecking
				return m, checkServer(m.textInput.Value())
			case tea.KeyCtrlC:
				return m, tea.Quit
			}
		// If we're done, any key press will quit.
		case StateDone:
			if msg.Type == tea.KeyCtrlC {
				return m, tea.Quit
			}
			return m, tea.Quit
		// If we're checking, only Ctrl+C will do anything.
		case StateChecking:
			if msg.Type == tea.KeyCtrlC {
				return m, tea.Quit
			}
		}

	// Handle the result of our HTTP check.
	case statusMsg:
		m.state = StateDone
		m.status = int(msg)
		return m, nil

	// Handle any errors from our HTTP check.
	case errMsg:
		m.state = StateDone
		m.err = msg
		return m, nil
	}

	// If we're in the input state, pass the message to the text input component.
	if m.state == StateGetURL {
		m.textInput, cmd = m.textInput.Update(msg)
	}

	return m, cmd
}

// View renders the UI based on the current model state.
func (m model) View() string {
	// If there was an error, just show that.
	if m.err != nil {
		return fmt.Sprintf("\nWe had some trouble: %v\n\n(press any key to quit)", m.err)
	}

	// Render the view based on the application state.
	switch m.state {
	case StateGetURL:
		return fmt.Sprintf(
			"Enter a URL to check:\n\n%s\n\n(ctrl+c to quit)",
			m.textInput.View(),
		)

	// For checking and done states, we show the status line.
	default:
		url := m.textInput.Value()
		s := fmt.Sprintf("Checking %s... ", url)

		if m.status > 0 {
			s += fmt.Sprintf("%d %s!", m.status, http.StatusText(m.status))
		}

		if m.state == StateDone {
			s += "\n\n(press any key to quit)"
		}
		return "\n" + s + "\n"
	}
}

func main() {
	p := tea.NewProgram(initialModel())
	if err := p.Start(); err != nil {
		fmt.Printf("Alas, there's been an error: %v\n", err)
		os.Exit(1)
	}
}
