package tools

type Preview struct {
	Operation  string `json:"operation"`
	Path       string `json:"path,omitempty"`
	From       string `json:"from,omitempty"`
	To         string `json:"to,omitempty"`
	Diff       string `json:"diff,omitempty"`
	BeforeHash string `json:"beforeHash,omitempty"`
	CanApply   bool   `json:"canApply"`
	Message    string `json:"message,omitempty"`
}

type Result struct {
	Status    string `json:"status"`
	Operation string `json:"operation"`
	Path      string `json:"path,omitempty"`
	From      string `json:"from,omitempty"`
	To        string `json:"to,omitempty"`
	Message   string `json:"message,omitempty"`
}
