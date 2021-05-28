package internal

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-message"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

const DatevWebAppURL = "https://demvreply.datevnet.de/web.app"

type DatevCfg struct {
	MaxConnections int
}

type DatevClient struct {
	client http.Client
}

type SecureMail struct {
	dc  *DatevClient
	res *http.Response
}

func DatevInit(cfg DatevCfg) *DatevClient {
	return &DatevClient{
		client: http.Client{
			Transport: &http.Transport{
				MaxConnsPerHost: cfg.MaxConnections,
			},
		},
	}
}

func GetSecureMailAttachment(msg *imap.Message) (io.Reader, error) {
	for _, literal := range msg.Body {
		entity, err := message.Read(literal)
		if err != nil {
			return nil, err
		}

		multiPartReader := entity.MultipartReader()

		for {
			ent, err := multiPartReader.NextPart()
			if err != nil {
				if err != io.EOF {
					return nil, err
				}

				break
			}

			kind, params, err := ent.Header.ContentType()
			if err != nil {
				return nil, err
			}

			if kind != "text/html" {
				continue
			}

			if params["name"] == "secure-email.html" {
				return ent.Body, nil
			}
		}
	}

	return nil, fmt.Errorf("mail has no body")
}

func (dc *DatevClient) OpenSecureMail(reader io.Reader, username string, password string) (*SecureMail, error) {
	openForm, err := getFormValues(reader, "form")

	// Open secure mail
	openRes, err := dc.sendDatevRequest(openForm)
	defer openRes.Body.Close()
	if err != nil {
		return nil, err
	}

	loginForm, err := getFormValues(openRes.Body, "#content > form")
	if err != nil {
		return nil, err
	}

	loginForm.Set("email", username)
	loginForm.Set("password", password)

	res, err := dc.sendDatevRequest(loginForm)
	if err != nil {
		return nil, err
	}

	return &SecureMail{
		dc:  dc,
		res: res,
	}, nil
}

func (dc *DatevClient) sendDatevRequest(form url.Values) (_ *http.Response, err error) {
	req, _ := http.NewRequest("POST", DatevWebAppURL, strings.NewReader(form.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	res, err := dc.client.Do(req)
	if err != nil {
		return
	}

	return res, err
}

func (sm *SecureMail) Download(filepath string) error {
	form, err := getFormValues(sm.res.Body, "#content > ul.nav.nav-pills:last-of-type > li:last-child > form")
	defer sm.res.Body.Close()
	if err != nil {
		return err
	}

	form.Set("access", "raw")

	res, err := sm.dc.sendDatevRequest(form)
	defer res.Body.Close()
	if err != nil {
		return err
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}

	defer out.Close()
	_, err = io.Copy(out, res.Body)

	return err
}

func getFormValues(reader io.Reader, formSelector string) (url.Values, error) {
	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		return nil, err
	}

	formNode := doc.Find(formSelector).First()
	if formNode == nil {
		return nil, fmt.Errorf("cannot find form in secure mail")
	}

	form := url.Values{}
	formNode.Find("input[type=hidden]").Each(func(_ int, selection *goquery.Selection) {
		name, hasName := selection.Attr("name")
		value, hasValue := selection.Attr("value")

		if hasName && hasValue {
			form.Set(name, value)
		}
	})

	return form, nil
}
