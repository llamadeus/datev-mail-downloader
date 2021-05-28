# datev-mail-downloader

DATEV helps to secure communication between two parties using encrypted mails. Recipients will receive a mail containing a "secure-mail.html" file.

Unfortunately, the naming of this file "secure-email.html" makes it cumbersome to store this email attachment in a local file system, as each file must be renamed individually.

This tool automatically browses the archive mailbox of the specified IMAP account, stores the secure-mail.html attachments locally and renames them so that they can be found again.

## Setup

Copy `.env.example` and provide the required credentials.

```shell
cp .env.example .env
```

# Run

Simply run the Go program using the Go CLI.

```shell
go run main.go
```
