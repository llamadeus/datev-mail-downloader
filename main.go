package main

import (
	"bufio"
	"fmt"
	"github.com/emersion/go-imap"
	"github.com/llamadeus/spba-email-client/internal"
	"github.com/spf13/viper"
	"io"
	"log"
	"os"
	"path"
	"strings"
	"sync"
)

const storageFolder = "storage"
const dbFilename = "db.txt"

var (
	dbMutex sync.Mutex
	dbFile  = path.Join(storageFolder, dbFilename)
)

func init() {
	viper.AutomaticEnv()
	viper.SetConfigFile(".env")

	err := viper.ReadInConfig()

	if err != nil && !os.IsNotExist(err) {
		log.Fatal(err)
	}
}

func main() {
	server := viper.GetString("IMAP_SERVER")
	username := viper.GetString("IMAP_USERNAME")
	password := viper.GetString("IMAP_PASSWORD")

	if len(server) == 0 {
		log.Fatal("please provide the imap server name")
	}

	if len(username) == 0 {
		log.Fatal("please provide your imap username")
	}

	if len(password) == 0 {
		log.Fatal("please provide your imap password")
	}

	os.MkdirAll(path.Join("storage", "mails"), os.ModePerm)

	db := readDB()
	m := internal.MailInit(internal.MailConfig{
		ImapServer: server,
		Username:   username,
		Password:   password,
	})
	dc := internal.DatevInit(internal.DatevCfg{
		MaxConnections: 10,
	})

	wg := sync.WaitGroup{}

	messages := m.FilterMessages("INBOX.Archive", mailFilter)
	for msg := range messages {
		msgID := msg.Envelope.MessageId

		if ok := db[msgID]; ok {
			fmt.Printf("Skipping message %s\n", msg.Envelope.Subject)
			continue
		}

		wg.Add(1)

		go func(msg *imap.Message) {
			defer wg.Done()

			if err := downloadSecureMail(dc, msg); err != nil {
				fmt.Println(err)
				return
			}

			appendDB(msgID)
		}(msg)
	}

	if m.Error() != nil {
		log.Fatalf("error: %v\n", m.Error())
	}

	wg.Wait()

	fmt.Println("Done!")
}

func downloadSecureMail(dc *internal.DatevClient, msg *imap.Message) error {
	subject := msg.Envelope.Subject
	timestamp := msg.Envelope.Date.Format("2006-01-02 15.04.05")
	sanitized := internal.SanitizeFilename(subject)
	filename := fmt.Sprintf("%s %s", timestamp, sanitized)
	filenameHTML := fmt.Sprintf("%s.html", filename)

	fmt.Printf("Processing message: %s\n", subject)

	attachment, err := internal.GetSecureMailAttachment(msg)
	if err != nil {
		fmt.Printf("> Mail has no \"secure-mail.html\" attachment")

		return nil
	}

	fmt.Printf("Downloading to: %s\n", filenameHTML)
	persistSecureEmailHTML(attachment, path.Join("storage", "mails", filenameHTML))

	if isEnvSet("DATEV_USERNAME") && isEnvSet("DATEV_PASSWORD") {
		filenameEML := fmt.Sprintf("%s.eml", filename)

		mail, err := dc.OpenSecureMail(
			attachment,
			viper.GetString("DATEV_USERNAME"),
			viper.GetString("DATEV_PASSWORD"),
		)
		if err != nil {
			return fmt.Errorf("cannot open secure mail: %s", err)
		}

		err = mail.Download(path.Join("storage", "mails", filenameEML))
		if err != nil {
			return fmt.Errorf("cannot download mail: %s", err)
		}
	}

	return nil
}

func mailFilter(msg *imap.Message) bool {
	fromSuffix := viper.GetString("MAIL_FROM_SUFFIX")

	if len(fromSuffix) == 0 {
		return true
	}

	for _, address := range msg.Envelope.From {
		if strings.HasSuffix(strings.ToLower(address.Address()), fromSuffix) {
			return true
		}
	}

	return false
}

func readDB() map[string]bool {
	file, err := os.Open(dbFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		log.Fatal("failed to open")
	}

	defer file.Close()

	db := map[string]bool{}
	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		db[scanner.Text()] = true
	}

	return db
}

func appendDB(str string) {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	file, err := os.OpenFile(dbFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal("failed to open")
	}

	defer file.Close()

	_, _ = file.WriteString(fmt.Sprintf("%s\n", str))
}

func persistSecureEmailHTML(body io.Reader, filepath string) {
	out, err := os.Create(filepath)
	if err != nil {
		fmt.Printf("cannot create file: %v\n", err)
		return
	}

	defer out.Close()

	_, err = io.Copy(out, body)
}

func isEnvSet(key string) bool {
	if !viper.IsSet(key) {
		return false
	}

	return len(viper.GetString(key)) > 0
}
