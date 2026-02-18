package transport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"time"
)

type CustomTransport struct {
	User      string
	Password  string
	Token     string
	Path      string
	Debug     bool
	AlterBody bool
	transport http.RoundTripper
}

func NewCustomTransport() *CustomTransport {
	defTransport := http.DefaultTransport.(*http.Transport).Clone()
	return &CustomTransport{transport: defTransport}
}

func (ct *CustomTransport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	// alter the body if requested..

	if ct.AlterBody {
		content, err := alterBody(req.Body)
		//req.Body, err = alterBody(req.Body)
		if err != nil {
			return nil, err
		}

		req.Body = io.NopCloser(bytes.NewReader(content))
		req.ContentLength = int64(len(content))
	}

	// Rewrite path if provided ...
	if ct.Path != "" {
		req.URL.Path = ct.Path
	}

	if ct.User != "" && ct.Password != "" {
		log.Print("basic Auth")
		req.SetBasicAuth(ct.User, ct.Password)

	} else if ct.Token != "" {
		req.Header.Set("Authorization", "Bearer "+ct.Token)
	}

	if ct.Debug {
		fmt.Printf("------ Request (%s) ------\n", time.Now().Format("2006-01-02 15:04:05.999"))
		if out, err := httputil.DumpRequestOut(req, true); err == nil {
			fmt.Printf("%s\n", string(out))
		} else {
			fmt.Printf("httputil.DumpRequestOut error: %v", err)
		}
	}

	resp, err = ct.transport.RoundTrip(req)

	if err != nil {
		return nil, err
	}

	if ct.Debug {
		fmt.Println("------ Response ------")
		if out, err := httputil.DumpResponse(resp, true); err == nil {
			fmt.Printf("%s\n", string(out))
		} else {
			fmt.Printf("httputil.DumpResponse error: %v", err)
		}

		fmt.Printf("===============\n\n")
	}

	return resp, nil
}

func alterBody(b io.ReadCloser) ([]byte, error) {
	var nobody = []byte{}

	if b == nil || b == http.NoBody {
		// No copying needed. Preserve the magic sentinel meaning of NoBody.
		return nobody, nil
	}

	var buf bytes.Buffer

	if _, err := buf.ReadFrom(b); err != nil {
		return nobody, err
	}

	if err := b.Close(); err != nil {
		return nobody, err
	}

	// clone the body
	content := buf.Bytes()

	// unmarshal the body into a map

	m := map[string]any{}
	if err := json.Unmarshal(content, &m); err != nil {
		return nobody, nil
	}

	// replace some headers with custom ones
	if v, ok := m["max_completion_tokens"]; ok {
		delete(m, "max_completion_tokens")
		m["max_tokens"] = v
	}

	// recreate the body
	raw, err := json.Marshal(m)
	if err != nil {
		return nobody, nil
	}

	return raw, nil
}

func alterBody2(b io.ReadCloser) (io.ReadCloser, error) {
	if b == nil || b == http.NoBody {
		// No copying needed. Preserve the magic sentinel meaning of NoBody.
		return http.NoBody, nil
	}

	var buf bytes.Buffer

	if _, err := buf.ReadFrom(b); err != nil {
		return b, err
	}

	if err := b.Close(); err != nil {
		return b, err
	}

	// clone the body
	content := buf.Bytes()

	// unmarshal the body into a map
	m := map[string]any{}
	if err := json.Unmarshal(content, &m); err != nil {
		return io.NopCloser(bytes.NewReader(content)), nil
	}

	// replace some headers with custom ones
	if v, ok := m["max_completion_tokens"]; ok {
		delete(m, "max_completion_tokens")
		m["max_tokens"] = v
	}

	// recreate the body
	raw, err := json.Marshal(m)

	if err != nil {
		return io.NopCloser(bytes.NewReader(content)), nil
	}

	for len(raw) < len(content) {
		raw = append(raw, 32) // add space if needed
	}

	return io.NopCloser(bytes.NewReader(raw)), nil
}
