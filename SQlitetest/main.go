package main

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

type Post struct {
	gorm.Model
	Title string
	Slug  string `gorm:"uniqueIndex:idx_slug"`
	Likes uint
	UserIP string
}

type model struct {
	textInput textinput.Model
	err       error
	UserIP    string
}

const (
	host = "localhost"
	port = "69"
)

func (p Post) String() string {
	return fmt.Sprintf("Post Title: %s, Slug: %s,", p.Title, p.Slug)
}

var db, err = gorm.Open(sqlite.Open("test.db"), &gorm.Config{})

func main() {
	// Auto-migrate the database
	db.AutoMigrate(&Post{})

	makeserver()
	// oldPost := getPost("new-slug")
	// fmt.Println(oldPost)
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
	return textinput.Blink
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
			createPost(input+fmt.Sprintf("-%d", rand.Intn(1000)), input, m.UserIP)
			return m, tea.Quit
		case tea.KeyDown:
			input := m.textInput.Value()
			fmt.Println(getPost(input))
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

func createPost(title string, slug string, UserIP string) Post {

	newPost := Post{Title: title, Slug: slug, UserIP: UserIP}
	if res := db.Create(&newPost); res.Error != nil {
		panic(res.Error)
	}
	return newPost
}

func getPost(slug string) Post {
	var targetPost Post
	if res := db.Where("slug = ?", slug).First(&targetPost); res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			return Post{Title: "Not Found", Slug: "not-found"}
		}
		panic(res.Error)
	}
	return targetPost
}
