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

func TestResolveLatestModel(t *testing.T) {
	openaiModels := []ModelInfo{
		{ID: "gpt-4o"},
		{ID: "gpt-4o-2024-11-20"},
		{ID: "gpt-4o-2024-08-06"},
		{ID: "gpt-4o-mini"},
		{ID: "gpt-4o-mini-2024-07-18"},
		{ID: "gpt-4"},
		{ID: "gpt-4-0613"},
		{ID: "gpt-4-0125-preview"},
		{ID: "gpt-4-1106-preview"},
	}

	tests := []struct {
		name    string
		models  []ModelInfo
		base    string
		want    string
		wantErr bool
	}{
		{
			name:   "picks latest dated snapshot, not the mini family",
			models: openaiModels,
			base:   "gpt-4o",
			want:   "gpt-4o-2024-11-20",
		},
		{
			name:   "mini family resolves to its own dated snapshot",
			models: openaiModels,
			base:   "gpt-4o-mini",
			want:   "gpt-4o-mini-2024-07-18",
		},
		{
			name:   "preview variants excluded by default",
			models: openaiModels,
			base:   "gpt-4",
			want:   "gpt-4-0613",
		},
		{
			name: "base containing a preview marker includes preview variants",
			models: []ModelInfo{
				{ID: "gpt-4-0125-preview"},
				{ID: "gpt-4-1106-preview"},
			},
			base: "gpt-4-0125-preview",
			want: "gpt-4-0125-preview",
		},
		{
			name:   "exact-only match resolves to the base itself",
			models: []ModelInfo{{ID: "gemini-2.0-flash"}, {ID: "gemini-1.5-pro"}},
			base:   "gemini-2.0-flash",
			want:   "gemini-2.0-flash",
		},
		{
			name:    "no match returns an error",
			models:  openaiModels,
			base:    "o1",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveLatestModel(tt.models, tt.base)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (resolved %q)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("ResolveLatestModel(%q) = %q, want %q", tt.base, got, tt.want)
			}
		})
	}
}
