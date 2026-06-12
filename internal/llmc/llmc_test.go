package llmc

import "testing"

func TestParseModelString(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantProvider string
		wantModel    string
		wantErr      bool
	}{
		{
			name:         "valid openai model",
			input:        "openai:gpt-4",
			wantProvider: "openai",
			wantModel:    "gpt-4",
			wantErr:      false,
		},
		{
			name:         "valid gemini model",
			input:        "gemini:gemini-2.0-flash",
			wantProvider: "gemini",
			wantModel:    "gemini-2.0-flash",
			wantErr:      false,
		},
		{
			name:         "model with colon",
			input:        "openai:o1:2024-12-17",
			wantProvider: "openai",
			wantModel:    "o1:2024-12-17",
			wantErr:      false,
		},
		{
			name:         "with whitespace",
			input:        " openai : gpt-4 ",
			wantProvider: "openai",
			wantModel:    "gpt-4",
			wantErr:      false,
		},
		{
			name:         "missing colon",
			input:        "openai-gpt-4",
			wantProvider: "",
			wantModel:    "",
			wantErr:      true,
		},
		{
			name:         "empty provider",
			input:        ":gpt-4",
			wantProvider: "",
			wantModel:    "",
			wantErr:      true,
		},
		{
			name:         "empty model",
			input:        "openai:",
			wantProvider: "",
			wantModel:    "",
			wantErr:      true,
		},
		{
			name:         "empty string",
			input:        "",
			wantProvider: "",
			wantModel:    "",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, model, err := ParseModelString(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseModelString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if provider != tt.wantProvider {
				t.Errorf("ParseModelString() provider = %v, want %v", provider, tt.wantProvider)
			}
			if model != tt.wantModel {
				t.Errorf("ParseModelString() model = %v, want %v", model, tt.wantModel)
			}
		})
	}
}
