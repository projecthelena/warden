package dockerapi

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/projecthelena/warden/internal/db"
)

const maxResponseBytes = 4 << 20

type Container struct {
	ID     string            `json:"id"`
	Name   string            `json:"name"`
	Image  string            `json:"image"`
	State  string            `json:"state"`
	Status string            `json:"status"`
	Health string            `json:"health,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
}

type ContainerState struct {
	Status       string
	Running      bool
	Paused       bool
	Restarting   bool
	OOMKilled    bool
	ExitCode     int
	Error        string
	Health       string
	HealthOutput string
	RestartCount int
}

type Client struct {
	httpClient *http.Client
	baseURL    string
}

func NewClient(host db.DockerHost, timeout time.Duration) (*Client, error) {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	transport := &http.Transport{}
	baseURL := strings.TrimRight(host.Endpoint, "/")

	if strings.HasPrefix(host.Endpoint, "unix://") {
		socket := strings.TrimPrefix(host.Endpoint, "unix://")
		if socket == "" || !strings.HasPrefix(socket, "/") {
			return nil, fmt.Errorf("docker socket must be an absolute path")
		}
		transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{Timeout: timeout}).DialContext(ctx, "unix", socket)
		}
		baseURL = "http://docker"
	} else {
		parsed, err := url.Parse(host.Endpoint)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return nil, fmt.Errorf("docker endpoint must use unix://, http://, or https://")
		}
		if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
			return nil, fmt.Errorf("docker endpoint must not contain credentials, a path, query, or fragment")
		}
		if parsed.Scheme != "https" && (host.CACert != "" || host.ClientCert != "" || host.ClientKey != "") {
			return nil, fmt.Errorf("docker TLS certificates require an https:// endpoint")
		}
	}

	if strings.HasPrefix(baseURL, "https://") {
		tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
		if host.CACert != "" {
			roots, err := x509.SystemCertPool()
			if err != nil || roots == nil {
				roots = x509.NewCertPool()
			}
			if !roots.AppendCertsFromPEM([]byte(host.CACert)) {
				return nil, fmt.Errorf("invalid Docker CA certificate")
			}
			tlsConfig.RootCAs = roots
		}
		if host.ClientCert != "" || host.ClientKey != "" {
			if host.ClientCert == "" || host.ClientKey == "" {
				return nil, fmt.Errorf("both Docker client certificate and key are required")
			}
			certificate, err := tls.X509KeyPair([]byte(host.ClientCert), []byte(host.ClientKey))
			if err != nil {
				return nil, fmt.Errorf("invalid Docker client certificate or key: %w", err)
			}
			tlsConfig.Certificates = []tls.Certificate{certificate}
		}
		transport.TLSClientConfig = tlsConfig
	}

	return &Client{httpClient: &http.Client{Transport: transport, Timeout: timeout}, baseURL: baseURL}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	var value string
	if err := c.get(ctx, "/_ping", &value); err != nil {
		return err
	}
	if value != "OK" {
		return fmt.Errorf("docker daemon returned %q", value)
	}
	return nil
}

func (c *Client) Containers(ctx context.Context) ([]Container, error) {
	var raw []struct {
		ID     string            `json:"Id"`
		Names  []string          `json:"Names"`
		Image  string            `json:"Image"`
		State  string            `json:"State"`
		Status string            `json:"Status"`
		Labels map[string]string `json:"Labels"`
	}
	if err := c.get(ctx, "/containers/json?all=1", &raw); err != nil {
		return nil, err
	}
	containers := make([]Container, 0, len(raw))
	for _, item := range raw {
		name := ""
		if len(item.Names) > 0 {
			name = strings.TrimPrefix(item.Names[0], "/")
		}
		containers = append(containers, Container{ID: item.ID, Name: name, Image: item.Image, State: item.State, Status: item.Status, Health: healthFromStatus(item.Status), Labels: item.Labels})
	}
	sort.Slice(containers, func(i, j int) bool { return containers[i].Name < containers[j].Name })
	return containers, nil
}

func (c *Client) Inspect(ctx context.Context, selector string) (ContainerState, error) {
	containers, err := c.Containers(ctx)
	if err != nil {
		return ContainerState{}, err
	}
	var match *Container
	for i := range containers {
		container := &containers[i]
		if container.Name == selector || container.ID == selector || strings.HasPrefix(container.ID, selector) {
			match = container
			break
		}
	}
	if match == nil {
		return ContainerState{}, fmt.Errorf("container %q not found", selector)
	}

	var raw struct {
		RestartCount int `json:"RestartCount"`
		State        struct {
			Status     string `json:"Status"`
			Running    bool   `json:"Running"`
			Paused     bool   `json:"Paused"`
			Restarting bool   `json:"Restarting"`
			OOMKilled  bool   `json:"OOMKilled"`
			ExitCode   int    `json:"ExitCode"`
			Error      string `json:"Error"`
			Health     *struct {
				Status string `json:"Status"`
				Log    []struct {
					Output string `json:"Output"`
				} `json:"Log"`
			} `json:"Health"`
		} `json:"State"`
	}
	if err := c.get(ctx, "/containers/"+url.PathEscape(match.ID)+"/json", &raw); err != nil {
		return ContainerState{}, err
	}
	state := ContainerState{Status: raw.State.Status, Running: raw.State.Running, Paused: raw.State.Paused, Restarting: raw.State.Restarting, OOMKilled: raw.State.OOMKilled, ExitCode: raw.State.ExitCode, Error: raw.State.Error, RestartCount: raw.RestartCount}
	if raw.State.Health != nil {
		state.Health = raw.State.Health.Status
		if logs := raw.State.Health.Log; len(logs) > 0 {
			state.HealthOutput = strings.TrimSpace(logs[len(logs)-1].Output)
		}
	}
	return state, nil
}

func (c *Client) get(ctx context.Context, path string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("cannot reach Docker host: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("docker API returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	reader := io.LimitReader(resp.Body, maxResponseBytes)
	if text, ok := dst.(*string); ok {
		body, err := io.ReadAll(reader)
		if err == nil {
			*text = strings.TrimSpace(string(body))
		}
		return err
	}
	if err := json.NewDecoder(reader).Decode(dst); err != nil {
		return fmt.Errorf("invalid Docker API response: %w", err)
	}
	return nil
}

func healthFromStatus(status string) string {
	start := strings.LastIndex(status, "(")
	if start < 0 || !strings.HasSuffix(status, ")") {
		return ""
	}
	health := strings.TrimSuffix(status[start+1:], ")")
	health = strings.TrimPrefix(health, "health: ")
	switch health {
	case "healthy", "unhealthy", "starting":
		return health
	default:
		return ""
	}
}
