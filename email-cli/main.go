package main

import (
	"bytes"
	"fmt"
	"html/template"
	"net/smtp"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func sendmailsimple(subject string, body string, to []string) {
	auth := smtp.PlainAuth(
		"",
		"rapgallagher06@gmail.com",
		"fstnwtkxkhsbvneg",
		"smtp.gmail.com",
	)

	msg := "Subject: " + subject + "\n" + body

	err := smtp.SendMail(
		"smtp.gmail.com:587",
		auth,
		"rapgallagher06@gmail.com",
		to,
		[]byte(msg),
	)
	if err != nil {
		fmt.Println("Error sending email:", err)
	}
}

func sendmailsimpleHTML(subject string, templatePath string, to []string) {

	var body bytes.Buffer

	t, err := template.ParseFiles(templatePath)
	if err != nil {
		fmt.Println("Error parsing template:", err)
		return
	}

	t.Execute(&body, struct{ Name string }{Name: "Ryan"})

	auth := smtp.PlainAuth(
		"",
		"rapgallagher06@gmail.com",
		"fstnwtkxkhsbvneg",
		"smtp.gmail.com",
	)

	headers := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";"

	msg := "Subject: " + subject + "\n" + headers + "\n\n" + body.String()

	err = smtp.SendMail(
		"smtp.gmail.com:587",
		auth,
		"rapgallagher06@gmail.com",
		to,
		[]byte(msg),
	)
	if err != nil {
		fmt.Println("Error sending email:", err)
	}
}

func main() {
	// sendmailsimple(
	// "Another Subject",
	// "Sup dude",
	//  []string{"rapgallagher06@gmail.com"})
	sendmailsimpleHTML(
		"HTML Email Test",
		"./test.html",
		[]string{"rapgallagher06@gmail.com"},
	)
}

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "https://itsfred.dev"
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 50

	return model{
		textInput: ti,
	}
}

type model struct {
	textInput textinput.Model
	status    int
	err       error
}

func (m model) Init() tea.Cmd {
	// Start the blinking cursor in the text input.
	return textinput.Blink
}

// Add textinput handling to the Update method.
