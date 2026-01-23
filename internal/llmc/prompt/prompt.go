package prompt

import (
	"fmt"

	"github.com/BurntSushi/toml"
)

// Prompt represents the structure of a TOML prompt file
type Prompt struct {
	System    string  `toml:"system"`
	User      string  `toml:"user"`
	Model     *string `toml:"model,omitempty"`
	WebSearch *bool   `toml:"web_search,omitempty"`
}

// LoadPrompt loads a prompt file and returns its contents
func LoadPrompt(filePath string) (*Prompt, error) {
	var prompt Prompt
	if _, err := toml.DecodeFile(filePath, &prompt); err != nil {
		return nil, fmt.Errorf("error decoding prompt file: %v", err)
	}
	return &prompt, nil
}
