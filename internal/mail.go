package internal

import (
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"log"
)

type MailConfig struct {
	ImapServer string
	Username   string
	Password   string
}

type Mail struct {
	client *client.Client
	err    error
}

type FilterFn func(message *imap.Message) bool

func MailInit(cfg MailConfig) *Mail {
	m := Mail{}

	c, err := client.DialTLS(cfg.ImapServer, nil)
	if err == nil {
		err = c.Login(cfg.Username, cfg.Password)
	}

	m.client = c
	m.err = err

	return &m
}

func (m *Mail) ListMailboxes() {
	if m.err != nil {
		return
	}

	mailboxes := make(chan *imap.MailboxInfo, 10)
	done := make(chan error, 1)
	go func () {
		done <- m.client.List("", "*", mailboxes)
	}()

	log.Println("Mailboxes:")
	for m := range mailboxes {
		log.Println("* " + m.Name)
	}

	m.err = <-done
}

func (m *Mail) FilterMessages(inbox string, filterFn FilterFn) chan *imap.Message {
	if m.err != nil {
		return nil
	}

	mbox, err := m.client.Select(inbox, false)
	if err != nil {
		m.err = err
		return nil
	}

	seqset := new(imap.SeqSet)
	messages := make(chan *imap.Message, 10)
	filtered := make(chan *imap.Message, 10)

	seqset.AddRange(1, mbox.Messages)

	go func() {
		m.err = m.client.Fetch(seqset, []imap.FetchItem{imap.FetchEnvelope, imap.FetchRFC822}, messages)
	}()

	go func() {
		for message := range messages {
			if filterFn(message) {
				filtered <- message
			}
		}

		close(filtered)
	}()

	return filtered
}

func (m *Mail) Error() error {
	return m.err
}

func (m *Mail) Close() {
	m.err = m.client.Logout()
}
