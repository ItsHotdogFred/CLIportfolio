package main

// go get github.com/glebarez/sqlite
// go get gorm.io/gorm
// go get github.com/charmbracelet/bubbles/textinput
// go get github.com/charmbracelet/bubbletea
// go get github.com/charmbracelet/log
// go get github.com/charmbracelet/ssh
// go get github.com/charmbracelet/wish
// go get github.com/charmbracelet/wish/activeterm
// go get github.com/charmbracelet/wish/bubbletea
// go get github.com/charmbracelet/wish/logging

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"
)

type Journal struct {
	gorm.Model
	Title    string
	Slug     string `gorm:"uniqueIndex:idx_slug"`
	UserIP   string `gorm:"index:idx_user_ip"` // Index for faster lookups by UserIP
	Contents string `gorm:"type:text"`         // Use text type for larger content
}

type model struct {
	textInput textinput.Model
	err       error
	UserIP    string
	Journal   []Journal // This will hold the Journals created by the user
	cursor    int
}

const (
	host = "localhost"
	port = "69"
)

// func (p Journal) String() string {
// 	return fmt.Sprintf("Journal Title: %s, Slug: %s,", p.Title, p.Slug)
// }

var db, err = gorm.Open(sqlite.Open("test.db"), &gorm.Config{})

func main() {
	// Auto-migrate the database
	db.AutoMigrate(&Journal{})

	makeserver()
	// oldJournal := getJournal("new-slug")
	// fmt.Println(oldJournal)
}

func initialModel(userIP string) model {
	ti := textinput.New()
	ti.Placeholder = "Add something to the database"
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 40

	return model{
		textInput: ti,
		err:       nil,
		UserIP:    userIP,
	}
}

func (m model) Init() tea.Cmd {
	//return textinput.Blink
	fmt.Println("Would you like to open or create a journal entry?)")
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyUp:
			input := m.textInput.Value()
			createJournal(input+fmt.Sprintf("-%d", rand.Intn(1000)), input, m.UserIP)
			return m, tea.Quit
		case tea.KeyDown:
			input := m.textInput.Value()
			fmt.Println(getJournal(input))
		}
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n", m.err)
	}
	return fmt.Sprintf(
		"Enter a title and then use the up arrow to add it to the database and the down arrow to find it.\n\n%s",
		m.textInput.View(),
	)
}

func teaHandler(s ssh.Session) (tea.Model, []tea.ProgramOption) {
	userIP := s.RemoteAddr().String()
	host, _, err := net.SplitHostPort(userIP)
	if err != nil {
		host = userIP // Fallback to the full address if SplitHostPort fails
	}

	return initialModel(host), []tea.ProgramOption{tea.WithAltScreen()}
}

func makeserver() {
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

func createJournal(title string, slug string, UserIP string) Journal {

	newJournal := Journal{Title: title, Slug: slug, UserIP: UserIP}
	if res := db.Create(&newJournal); res.Error != nil {
		panic(res.Error)
	}
	return newJournal
}

func getJournal(slug string) Journal {
	var targetJournal Journal
	if res := db.Where("slug = ?", slug).First(&targetJournal); res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			return Journal{Title: "Not Found", Slug: "not-found"}
		}
		panic(res.Error)
	}
	return targetJournal
}
