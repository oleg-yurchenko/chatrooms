package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/timer"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/oleg-yurchenko/chatrooms/internal/client"
)

type model struct {
	messageLog *bytes.Buffer
	input      textinput.Model
	timer      timer.Model
	err        error
	client     *client.Client
}

func initialModel(client *client.Client) model {
	ti := textinput.New()
	ti.Focus()

	tmr := timer.NewWithInterval(time.Duration(time.Millisecond*500), time.Duration(time.Millisecond*500))

	return model{
		messageLog: new(bytes.Buffer),
		input:      ti,
		timer:      tmr,
		err:        nil,
		client:     client,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.timer.Init())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case timer.TickMsg:
		m.timer, cmd = m.timer.Update(msg)
		return m, cmd

	case timer.TimeoutMsg:
		msgs := m.client.FetchMessages()

		for _, message := range msgs {
			// NOTE: name and message are not sanitized -- should fix!
			m.messageLog.WriteString(fmt.Sprintf("[%s] %s\n", message.Nickname, message.Data))
		}

		cmd = m.timer.Init()

		return m, cmd

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			text := m.input.Value()
			m.client.SendMessage(text)
			m.input.Reset()

			return m, nil

		case tea.KeyEsc:
			return m, tea.Quit
		}

	case error:
		m.err = msg
		return m, nil
	}

	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m model) View() string {
	return fmt.Sprintf("%s\n%s\n", m.messageLog.String(), m.input.View())
}

func main() {
	fmt.Println("Client")

	name := flag.String("name", "anon", "The name to use in the chatroom")
	addr := flag.String("addr", "127.0.0.1", "IP address of the desired destination")
	port := flag.Int("port", 11337, "The port that the chatroom is listening on")

	flag.Parse()

	cfg := &client.ClientConfig{
		Addr: *addr,
		Port: *port,
		Name: *name,
	}
	user, err := client.MakeClient(*cfg)
	if err != nil {
		log.Printf("Failed to initialize connection to server: %v", err)
		return
	}

	err = user.Establish()
	if err != nil {
		log.Printf("Failed to establish connection to server: %v", err)
	}

	prog := tea.NewProgram(initialModel(user))
	if _, err := prog.Run(); err != nil {
		log.Fatalf("%v", err)
	}

	user.Exit()
}
