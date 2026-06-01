package upstream

import (
	"bytes"
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	ErrUnavailable = errors.New("upstream unavailable")
	ErrBadGateway  = errors.New("bad gateway")
)

type Client struct {
	BaseURL string
	Timeout time.Duration
	Client  *http.Client
}

func New(baseURL string, timeout time.Duration) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Timeout: timeout,
		Client: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //se pasa en alto las certificaciones 
			},
		},
	}
}
func (c *Client) Request(method, path string, body []byte, headers http.Header) (int, []byte, http.Header, error) {
	target, err := url.JoinPath(c.BaseURL, path)
	if err != nil {
		return 0, nil, nil, ErrBadGateway
	}

	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, target, reader)
	if err != nil {
		return 0, nil, nil, ErrBadGateway
	}
	if headers != nil {
		req.Header = headers.Clone()
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return 0, nil, nil, ErrUnavailable
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, nil, ErrBadGateway
	}

	return resp.StatusCode, payload, resp.Header.Clone(), nil
}
