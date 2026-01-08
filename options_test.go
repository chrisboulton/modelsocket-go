package modelsocket

import "testing"

func TestGenOption_MaxTokens(t *testing.T) {
	cfg := genConfig{}
	WithMaxTokens(500)(&cfg)

	if cfg.maxTokens == nil {
		t.Fatal("maxTokens is nil")
	}
	if *cfg.maxTokens != 500 {
		t.Errorf("maxTokens = %d, want 500", *cfg.maxTokens)
	}
}

func TestGenOption_Temperature(t *testing.T) {
	cfg := genConfig{}
	WithTemperature(0.8)(&cfg)

	if cfg.temperature == nil {
		t.Fatal("temperature is nil")
	}
	if *cfg.temperature != 0.8 {
		t.Errorf("temperature = %f, want 0.8", *cfg.temperature)
	}
}

func TestGenOption_TopP(t *testing.T) {
	cfg := genConfig{}
	WithTopP(0.9)(&cfg)

	if cfg.topP == nil {
		t.Fatal("topP is nil")
	}
	if *cfg.topP != 0.9 {
		t.Errorf("topP = %f, want 0.9", *cfg.topP)
	}
}

func TestGenOption_TopK(t *testing.T) {
	cfg := genConfig{}
	WithTopK(40)(&cfg)

	if cfg.topK == nil {
		t.Fatal("topK is nil")
	}
	if *cfg.topK != 40 {
		t.Errorf("topK = %d, want 40", *cfg.topK)
	}
}

func TestGenOption_StopStrings(t *testing.T) {
	cfg := genConfig{}
	WithStopStrings("STOP", "END", "DONE")(&cfg)

	if len(cfg.stopStrings) != 3 {
		t.Fatalf("len(stopStrings) = %d, want 3", len(cfg.stopStrings))
	}
	if cfg.stopStrings[0] != "STOP" {
		t.Errorf("stopStrings[0] = %s, want STOP", cfg.stopStrings[0])
	}
}

func TestGenOption_Hidden(t *testing.T) {
	cfg := genConfig{}
	WithHidden()(&cfg)

	if !cfg.hidden {
		t.Error("hidden = false, want true")
	}
}

func TestGenOption_Seed(t *testing.T) {
	cfg := genConfig{}
	WithSeed(42)(&cfg)

	if cfg.seed == nil {
		t.Fatal("seed is nil")
	}
	if *cfg.seed != 42 {
		t.Errorf("seed = %d, want 42", *cfg.seed)
	}
}

func TestGenOption_Role(t *testing.T) {
	tests := []struct {
		name   string
		option GenOption
		want   Role
	}{
		{"User", GenerateAsUser(), RoleUser},
		{"Assistant", GenerateAsAssistant(), RoleAssistant},
		{"System", GenerateAsSystem(), RoleSystem},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := genConfig{}
			tt.option(&cfg)
			if cfg.role != tt.want {
				t.Errorf("role = %s, want %s", cfg.role, tt.want)
			}
		})
	}
}

func TestAppendOption_Role(t *testing.T) {
	tests := []struct {
		name   string
		option AppendOption
		want   Role
	}{
		{"User", AsUser(), RoleUser},
		{"Assistant", AsAssistant(), RoleAssistant},
		{"System", AsSystem(), RoleSystem},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := appendConfig{}
			tt.option(&cfg)
			if cfg.role != tt.want {
				t.Errorf("role = %s, want %s", cfg.role, tt.want)
			}
		})
	}
}

func TestAppendOption_Echo(t *testing.T) {
	cfg := appendConfig{}
	WithEcho()(&cfg)

	if !cfg.echo {
		t.Error("echo = false, want true")
	}
}

func TestOpenOption_SkipPrelude(t *testing.T) {
	cfg := openConfig{}
	WithSkipPrelude()(&cfg)

	if !cfg.skipPrelude {
		t.Error("skipPrelude = false, want true")
	}
}

func TestOpenOption_Toolbox(t *testing.T) {
	tb := NewToolbox()
	cfg := openConfig{}
	WithToolbox(tb)(&cfg)

	if cfg.toolbox != tb {
		t.Error("toolbox not set correctly")
	}
}

func TestGenConfig_ToSeqGenData(t *testing.T) {
	cfg := genConfig{}
	GenerateAsAssistant()(&cfg)
	WithMaxTokens(100)(&cfg)
	WithTemperature(0.7)(&cfg)
	WithStopStrings("END")(&cfg)

	data := cfg.toSeqGenData()

	if data.Role != "assistant" {
		t.Errorf("Role = %s, want assistant", data.Role)
	}
	if *data.MaxTokens != 100 {
		t.Errorf("MaxTokens = %d, want 100", *data.MaxTokens)
	}
	if *data.Temperature != 0.7 {
		t.Errorf("Temperature = %f, want 0.7", *data.Temperature)
	}
	if len(data.StopStrings) != 1 || data.StopStrings[0] != "END" {
		t.Errorf("StopStrings = %v, want [END]", data.StopStrings)
	}
}
