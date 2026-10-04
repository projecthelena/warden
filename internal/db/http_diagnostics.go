package db

import "encoding/json"

// HTTPDiagnostics keeps attempts and redirects separate. Durations are milliseconds;
// absent phases were not observed, rather than measured as taking zero time.
type HTTPDiagnostics struct {
	RetryMode     string               `json:"retryMode,omitempty"`
	RetryDecision string               `json:"retryDecision,omitempty"`
	TotalMS       float64              `json:"totalMs"`
	Attempts      []HTTPAttempt        `json:"attempts"`
	External      *ExternalObservation `json:"external,omitempty"`
}

type HTTPAttempt struct {
	Status       string    `json:"status"`
	FailurePhase string    `json:"failurePhase,omitempty"`
	TotalMS      float64   `json:"totalMs"`
	Hops         []HTTPHop `json:"hops"`
	Truncated    bool      `json:"truncated,omitempty"`
}

type HTTPPhase struct {
	Exchange   int     `json:"exchange"`
	Name       string  `json:"name"`
	StartMS    float64 `json:"startMs"`
	DurationMS float64 `json:"durationMs"`
	Complete   bool    `json:"complete"`
	Failed     bool    `json:"failed,omitempty"`
}

type HTTPHop struct {
	Host         string      `json:"host"`
	TotalMS      float64     `json:"totalMs"`
	Phases       []HTTPPhase `json:"phases"`
	RemoteIP     string      `json:"remoteIp,omitempty"`
	Reused       *bool       `json:"reused,omitempty"`
	StatusCode   int         `json:"statusCode,omitempty"`
	FailurePhase string      `json:"failurePhase,omitempty"`
	Truncated    bool        `json:"truncated,omitempty"`
}

type ExternalObservation struct {
	ObservedAt   string `json:"observedAt,omitempty"`
	Outcome      string `json:"outcome"`
	StatusCode   int    `json:"statusCode,omitempty"`
	FailurePhase string `json:"failurePhase,omitempty"`
}

func diagnosticsJSON(d *HTTPDiagnostics) any {
	if d == nil {
		return nil
	}
	b, _ := json.Marshal(d)
	return string(b)
}

func parseDiagnostics(s string) *HTTPDiagnostics {
	var d HTTPDiagnostics
	if json.Unmarshal([]byte(s), &d) != nil {
		return nil
	}
	return &d
}
