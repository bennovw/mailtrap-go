package mailtrap

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestSendEmailService_Marshal(t *testing.T) {
	testJSONMarshal(t, &SendEmailRequest{}, "{}")

	req := emailRequestMock()
	want := `{
	  "from": {
	    "email": "ches@example.com",
	    "name": "Ches"
	  },
	  "to": [
	    {
	  	  "email": "johndoe@example.com",
		  "name": "John Doe"
		},
		{
		  "email": "mike@example.com",
		  "name": "Mike"
		}
	  ],
	  "cc": [
	    {
	  	  "email": "info@example.com",
		  "name": "Example LLC"
		}
	  ],
	  "bcc": [
	    {
	  	  "email": "dontreply@example.com"
		}
	  ],
	  "attachments": [
	    {
	  	  "content": "PGh0bWw+CiAgICA8aGVhZD4KICAgICAgICA8dGl0bGU+YjY0PC90aXRsZT4KICAgIDwvaGVhZD4KICAgIDxib2R5PgogICAgPHA+SGVsbG8sIHdvcmxkITwvcD4KICAgIDwvYm9keT4KPC9odG1sPg==",
	  	  "filename": "index.html",
		  "type": "text/html",
		  "disposition": "attachment"
	    }
	  ],
	  "custom_variables": {
	    "user_id": "1",
	    "batch_id": "2"
	  },
	  "headers": {
	    "X-Message-Source": "mail.example.com"
	  },
	  "subject": "Your Example Order Confirmation",
	  "text": "Congratulations on your order no.123",
	  "category": "API Client"
	}`

	testJSONMarshal(t, req, want)
}

func TestSendEmailService_Send(t *testing.T) {
	client, mux, teardown := setupSendingClient()
	defer teardown()

	mux.HandleFunc("/send", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		fmt.Fprint(w, `{"success":true,"message_ids":["0c7fd939-02cf-11ed-88c2-0a58a9feac02","5e7fd111-11cf-oi3w-88c2-0a58a9feac02"]}`)
	})

	email := emailRequestMock()
	sendResp, _, err := client.Send(email)
	if err != nil {
		t.Errorf("SendEmail.Send returned error: %v", err)
	}

	emailResp := &SendEmailResponse{
		Success: true,
		MessageIDs: []string{
			"0c7fd939-02cf-11ed-88c2-0a58a9feac02",
			"5e7fd111-11cf-oi3w-88c2-0a58a9feac02",
		},
	}
	if !reflect.DeepEqual(sendResp, emailResp) {
		t.Errorf("SendEmail.Send returned %v, want %v", sendResp, emailResp)
	}

	_, _, err = client.Send(nil)
	if err == nil {
		t.Error("SendEmail.Send bad request, err = nil, want error")
	}

	// Assert that the client is ProductionSendingClient type to access its internal fields
	c, ok := client.(*ProductionSendingClient);
	if !ok {
		t.Errorf("SendEmail.Send sc is not ProductionSendingClient")
	}

	testNewRequestAndDoFail(t, "SendEmail.Send", &c.client, func() (*Response, error) {
		deliveredEmailIDs, resp, err := client.Send(email)
		if deliveredEmailIDs != nil {
			t.Errorf("SendEmail.Send client.BaseURL.Host=%v sendEmailResp=%#v, want nil", c.baseURL.Host, deliveredEmailIDs)
		}
		return resp, err
	})
}

func TestSendEmailService_Send_notValidEmailFrom(t *testing.T) {
	client, _, teardown := setupSendingClient()
	defer teardown()

	email := &SendEmailRequest{To: []EmailAddress{{Email: "test@example.com"}}}
	_, _, err := client.Send(email)
	if err.Error() != "'from' address is required" {
		t.Errorf("SendEmail.Send returned error: %v", err)
	}
}

func TestSendEmailService_Send_notValidEmailTo(t *testing.T) {
	client, _, teardown := setupSendingClient()
	defer teardown()

	email := &SendEmailRequest{From: EmailAddress{Email: "test@example.com"}}
	_, _, err := client.Send(email)
	if err.Error() != "'to' address is required" {
		t.Errorf("SendEmail.Send returned error: %v", err)
	}

	email = &SendEmailRequest{From: EmailAddress{Email: "test@example.com"}, To: []EmailAddress{{Email: ""}}}
	_, _, err = client.Send(email)
	if err.Error() != "'email' is required in 'to' address" {
		t.Errorf("SendEmail.Send returned error: %v", err)
	}
}

func TestSendEmailService_Send_notValidAttachmentIfExist(t *testing.T) {
	client, _, teardown := setupSendingClient()
	defer teardown()

	email := &SendEmailRequest{
		From:        EmailAddress{Email: "test@example.com"},
		To:          []EmailAddress{{Email: "email@example.com"}},
		Attachments: []EmailAttachment{{}},
	}

	_, _, err := client.Send(email)
	if err.Error() != "'content' is required in attachment; 'filename' is required in attachment" {
		t.Errorf("SendEmail.Send returned error: %v", err)
	}
}

func TestSendEmailService_Send_missedSubject(t *testing.T) {
	client, _, teardown := setupSendingClient()
	defer teardown()

	email := &SendEmailRequest{
		From:    EmailAddress{Email: "test@example.com"},
		To:      []EmailAddress{{Email: "email@example.com"}},
		Subject: "",
	}

	_, _, err := client.Send(email)
	if err.Error() != "'subject' is required" {
		t.Errorf("SendEmail.Send returned error: %v", err)
	}
}

func TestSendEmailService_Send_textOrHTMLRequired(t *testing.T) {
	client, _, teardown := setupSendingClient()
	defer teardown()

	email := &SendEmailRequest{
		From:    EmailAddress{Email: "test@example.com"},
		To:      []EmailAddress{{Email: "email@example.com"}},
		Subject: "Subj.",
	}

	_, _, err := client.Send(email)
	if err.Error() != "one of 'text' or 'html' is required" {
		t.Errorf("SendEmail.Send returned error: %v", err)
	}
}

func TestSendEmailService_Send_categoryTooLong(t *testing.T) {
	client, _, teardown := setupSendingClient()
	defer teardown()

	email := &SendEmailRequest{
		From:     EmailAddress{Email: "test@example.com"},
		To:       []EmailAddress{{Email: "email@example.com"}},
		Subject:  "Subj.",
		Text:     "Test",
		Category: strings.Repeat("c", 260),
	}

	_, _, err := client.Send(email)
	if err.Error() != "'category' is greater than 255 chars" {
		t.Errorf("SendEmail.Send returned error: %v", err)
	}
}

func emailRequestMock() *SendEmailRequest {
	return &SendEmailRequest{
		From: EmailAddress{
			Email: "ches@example.com",
			Name:  "Ches",
		},
		To: []EmailAddress{
			{Email: "johndoe@example.com", Name: "John Doe"},
			{Email: "mike@example.com", Name: "Mike"},
		},
		Cc:  []EmailAddress{{Email: "info@example.com", Name: "Example LLC"}},
		Bcc: []EmailAddress{{Email: "dontreply@example.com"}},
		Attachments: []EmailAttachment{
			{
				Content:     "PGh0bWw+CiAgICA8aGVhZD4KICAgICAgICA8dGl0bGU+YjY0PC90aXRsZT4KICAgIDwvaGVhZD4KICAgIDxib2R5PgogICAgPHA+SGVsbG8sIHdvcmxkITwvcD4KICAgIDwvYm9keT4KPC9odG1sPg==",
				AttachType:  "text/html",
				Filename:    "index.html",
				Disposition: "attachment",
			},
		},
		CustomVars: map[string]string{
			"user_id":  "1",
			"batch_id": "2",
		},
		Headers: map[string]string{
			"X-Message-Source": "mail.example.com",
		},
		Subject:  "Your Example Order Confirmation",
		Text:     "Congratulations on your order no.123",
		Category: "API Client",
	}
}
