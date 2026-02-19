package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/timer"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/oleg-yurchenko/chatrooms/internal/client"
)

type model struct {
	messageLog *bytes.Buffer
	viewport   viewport.Model
	ready      bool
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
		ready:      false,
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
	cmds := make([]tea.Cmd, 0)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if !m.ready {
			// TODO: figure out how to get this hard-coded value of 2
			m.viewport = viewport.New(msg.Width, msg.Height-2)
			m.viewport.SetContent(m.messageLog.String())
			m.viewport.SetYOffset(m.input.PromptStyle.GetHeight())
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 2
		}

	case timer.TickMsg:
		m.timer, cmd = m.timer.Update(msg)
		cmds = append(cmds, cmd)

	case timer.TimeoutMsg:
		msgs := m.client.FetchMessages()

		if len(msgs) > 0 {
			for _, message := range msgs {
				// NOTE: name and message are not sanitized -- should fix!
				m.messageLog.WriteString(fmt.Sprintf("[%s] %s\n", message.Nickname, message.Data))
			}
			m.viewport.SetContent(m.messageLog.String())
		}

		m.timer = timer.NewWithInterval(time.Duration(time.Millisecond*500), time.Duration(time.Millisecond*500))
		cmd = m.timer.Init()
		cmds = append(cmds, cmd)

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			text := m.input.Value()
			m.client.SendMessage(text)
			m.input.Reset()

		case tea.KeyEsc:
			cmds = append(cmds, tea.Quit)
		}

	case error:
		m.err = msg
	}

	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	if len(cmds) == 0 {
		return m, nil
	} else {
		return m, tea.Batch(cmds...)
	}
}

func (m model) View() string {
	return fmt.Sprintf("%s\n%s\n", m.viewport.View(), m.input.View())
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
