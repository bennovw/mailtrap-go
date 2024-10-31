package mailtrap

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"time"
)

const (
	libVersion = "0.1.2"

	testingAPIURL = "https://mailtrap.io/"
	sendingAPIURL = "https://send.api.mailtrap.io/"
	sandboxAPIURL = "https://sandbox.api.mailtrap.io/"

	apiSuffix = "api"

	defaultAccept = "application/json"
)

var (
	userAgent = fmt.Sprintf("mailtrap-go/%s (%s %s) go/%s", libVersion, runtime.GOOS, runtime.GOARCH, runtime.Version())
)

type client struct {
	// API key used to make authenticated API calls.
	apiKey string

	// Base URL for API requests.
	baseURL *url.URL

	// User agent used when communicating with the API.
	userAgent string

	// HTTP client used to communicate with the API.
	httpClient *http.Client
}

// SendingClient is an interface for managing communication with the Mailtrap send and sandbox APIs.
type SendingClient interface {
	Send(request *SendEmailRequest) (*SendEmailResponse, *Response, error)
	NewRequest(method, path string, body interface{}) (*http.Request, error)
	Do(req *http.Request, v interface{}) (*Response, error)

	// setBaseURL sets the base URL for the API client and is used by internal tests.
	setBaseURL(*url.URL)
}

// TestingClient manages communication with the Mailtrap testing API.
type TestingClient struct {
	client

	// Services used for communicating with the Mailtrap testing API.
	Accounts     *AccountsService
	AccountUsers *AccountUsersService
	Permissions  *PermissionsService
	Projects     *ProjectsService
	Inboxes      *InboxesService
	Messages     *MessagesService
	Attachments  *AttachmentsService
}

// NewSendingClient creates and returns a production instance of SendingClient.
func NewSendingClient(apiKey string) (SendingClient, error) {
	client, err := getClient(apiKey, sendingAPIURL)
	if err != nil {
		return nil, err
	}

	sc := &ProductionSendingClient{
		client,
	}

	return sc, nil
}

// NewSendingClient creates and returns a sandbox instance of SendingClient for development and testing.
func NewSandboxSendingClient(apiKey string, inboxID int64) (SendingClient, error) {
	client, err := getClient(apiKey, sandboxAPIURL)
	if err != nil {
		return nil, err
	}

	sc := &SandboxSendingClient{
		client:  client,
		inboxID: inboxID,
	}

	return sc, nil
}

// getClient returns a new client instance with the given API key and base URL.
func getClient(apiKey string, baseURL string) (client, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return client{}, err
	}
	u.Path += apiSuffix

	return client{
		apiKey:  apiKey,
		baseURL: u,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		userAgent: userAgent,
	}, nil
}

// NewTestingClient creates and returns an instance of TestingClient.
func NewTestingClient(apiKey string) (*TestingClient, error) {
	baseURL, err := url.Parse(testingAPIURL)
	if err != nil {
		return nil, err
	}
	baseURL.Path += apiSuffix

	client := &TestingClient{
		client: client{
			apiKey:     apiKey,
			baseURL:    baseURL,
			httpClient: http.DefaultClient,
			userAgent:  userAgent,
		},
	}

	// Create all the public services.
	client.Accounts = &AccountsService{client: &client.client}
	client.AccountUsers = &AccountUsersService{client: &client.client}
	client.Permissions = &PermissionsService{client: &client.client}
	client.Projects = &ProjectsService{client: &client.client}
	client.Inboxes = &InboxesService{client: &client.client}
	client.Messages = &MessagesService{client: &client.client}
	client.Attachments = &AttachmentsService{client: &client.client}

	return client, nil
}

func (c *client) Do(req *http.Request, v interface{}) (*Response, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() {
		if rerr := resp.Body.Close(); err == nil {
			err = rerr
		}
	}()

	response := &Response{Response: resp}
	if err := checkResponse(resp); err != nil {
		return response, err
	}

	if v != nil {
		if err := c.decode(v, resp.Body, req.Header.Get("Accept")); err != nil {
			return response, err
		}
	}

	return response, err
}

func (c *client) decode(v interface{}, body io.Reader, acceptHeader string) error {
	if body == nil {
		return nil
	}
	if s, ok := v.(*string); ok {
		data, err := io.ReadAll(body)
		if err != nil {
			return err
		}
		*s = string(data)
		return nil
	}
	if v != nil && acceptHeader == defaultAccept {
		if err := json.NewDecoder(body).Decode(v); err != nil {
			return err
		}
		return nil
	}

	return errors.New("decode() undefined response type")
}

// NewRequest creates an API request.
func (c *client) NewRequest(method, path string, body interface{}) (*http.Request, error) {
	u := c.baseURL
	u.Path = c.baseURL.Path + path

	var (
		req *http.Request
		err error
	)

	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		req, err = http.NewRequest(method, u.String(), nil)
		if err != nil {
			return nil, err
		}
	default:
		buf := new(bytes.Buffer)
		if body != nil {
			err = json.NewEncoder(buf).Encode(body)
			if err != nil {
				return nil, err
			}
		}
		req, err = http.NewRequest(method, u.String(), buf)
		if err != nil {
			return nil, err
		}
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", defaultAccept)
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	return req, nil
}

// Response is a Mailtrap response.
// This wraps the standard http.Response returned from Mailtrap.
type Response struct {
	*http.Response
}

// checkResponse checks the API response for errors and returns them if present.
// A response is considered an error if it has a status code outside the 200-299 range.
func checkResponse(r *http.Response) error {
	if c := r.StatusCode; c >= 200 && c <= 299 {
		return nil
	}
	errResponse := &ErrorResponse{Response: r}
	data, err := io.ReadAll(r.Body)
	if err == nil && len(data) > 0 {
		err := json.Unmarshal(data, errResponse)
		if err != nil {
			errResponse.Message = string(data)
		}
	}

	return errResponse
}
