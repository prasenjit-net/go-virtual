package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── prompter unit tests ────────────────────────────────────────────────────

func TestPrompter_NonInteractive_ReturnsDefaults(t *testing.T) {
	p := newPrompterFromReader(strings.NewReader(""), &bytes.Buffer{}, false)

	if got := p.Prompt("label", "def"); got != "def" {
		t.Errorf("Prompt: want %q, got %q", "def", got)
	}
	if got := p.PromptSecret("secret"); got != "" {
		t.Errorf("PromptSecret: want empty, got %q", got)
	}
	if got := p.PromptSelect("choose", []string{"a", "b"}, "a"); got != "a" {
		t.Errorf("PromptSelect: want %q, got %q", "a", got)
	}
	if got := p.PromptBool("bool", true); !got {
		t.Error("PromptBool: want true")
	}
}

func TestPrompter_Interactive_ReadsInput(t *testing.T) {
	input := "9090\nMy App\nMy Subtitle\n"
	p := newPrompterFromReader(strings.NewReader(input), &bytes.Buffer{}, true)

	if got := p.Prompt("Port", "8080"); got != "9090" {
		t.Errorf("want 9090, got %q", got)
	}
	if got := p.Prompt("Title", "Go Virtual"); got != "My App" {
		t.Errorf("want My App, got %q", got)
	}
	if got := p.Prompt("Sub", ""); got != "My Subtitle" {
		t.Errorf("want My Subtitle, got %q", got)
	}
}

func TestPrompter_Interactive_EmptyUsesDefault(t *testing.T) {
	// Pressing enter (empty line) should return the default.
	input := "\n"
	p := newPrompterFromReader(strings.NewReader(input), &bytes.Buffer{}, true)
	if got := p.Prompt("Port", "8080"); got != "8080" {
		t.Errorf("want default 8080, got %q", got)
	}
}

func TestPrompter_PromptSelect_ValidChoice(t *testing.T) {
	p := newPrompterFromReader(strings.NewReader("mongo\n"), &bytes.Buffer{}, true)
	got := p.PromptSelect("Type", []string{"file", "memory", "mongo"}, "file")
	if got != "mongo" {
		t.Errorf("want mongo, got %q", got)
	}
}

func TestPrompter_PromptSelect_InvalidFallsBack(t *testing.T) {
	var out bytes.Buffer
	p := newPrompterFromReader(strings.NewReader("invalid\n"), &out, true)
	got := p.PromptSelect("Type", []string{"file", "memory", "mongo"}, "file")
	if got != "file" {
		t.Errorf("want file, got %q", got)
	}
	if !strings.Contains(out.String(), "not recognised") {
		t.Error("expected 'not recognised' message")
	}
}

func TestPrompter_PromptBool(t *testing.T) {
	tests := []struct {
		input string
		def   bool
		want  bool
	}{
		{"y\n", false, true},
		{"yes\n", false, true},
		{"n\n", true, false},
		{"no\n", true, false},
		{"\n", true, true},   // empty → default
		{"\n", false, false}, // empty → default
		{"garbage\n", true, true}, // unrecognised → default
	}
	for _, tt := range tests {
		p := newPrompterFromReader(strings.NewReader(tt.input), &bytes.Buffer{}, true)
		got := p.PromptBool("confirm", tt.def)
		if got != tt.want {
			t.Errorf("input=%q def=%v: want %v, got %v", tt.input, tt.def, tt.want, got)
		}
	}
}

func TestPrompter_Section_PrintsHeader(t *testing.T) {
	var out bytes.Buffer
	p := newPrompterFromReader(strings.NewReader(""), &out, true)
	p.section("Storage")
	if !strings.Contains(out.String(), "Storage") {
		t.Error("expected section header to contain 'Storage'")
	}
}

func TestPrompter_Secret(t *testing.T) {
	p := newPrompterFromReader(strings.NewReader("mykey\n"), &bytes.Buffer{}, true)
	got := p.PromptSecret("API key")
	if got != "mykey" {
		t.Errorf("want mykey, got %q", got)
	}
}

// ── collectInitConfig tests ────────────────────────────────────────────────

func makeInput(lines ...string) string {
	return strings.Join(lines, "\n") + "\n"
}

func TestCollectInitConfig_Defaults(t *testing.T) {
	// All empty input → use all defaults.
	input := makeInput(
		"", // port
		"", // title
		"", // subtitle
		"", // storage type
		"", // session store type
		"n", // enable AI
	)
	p := newPrompterFromReader(strings.NewReader(input), &bytes.Buffer{}, true)
	cfg := collectInitConfig(p)

	if cfg.Server.Port != 8080 {
		t.Errorf("port: want 8080, got %d", cfg.Server.Port)
	}
	if cfg.Storage.Type != "file" {
		t.Errorf("storage type: want file, got %q", cfg.Storage.Type)
	}
	if cfg.Session.StoreType != "memory" {
		t.Errorf("session store: want memory, got %q", cfg.Session.StoreType)
	}
	if cfg.AI.Provider != "" {
		// Interactive "n" to AI should clear the provider.
		t.Errorf("AI provider: want empty (user said no), got %q", cfg.AI.Provider)
	}
}

func TestCollectInitConfig_MongoStorage(t *testing.T) {
	input := makeInput(
		"",     // port default
		"",     // title default
		"",     // subtitle default
		"mongo", // storage type
		"mongodb://db:27017", // URI
		"mydb",  // database
		"",      // session store default
		"n",     // no AI
	)
	p := newPrompterFromReader(strings.NewReader(input), &bytes.Buffer{}, true)
	cfg := collectInitConfig(p)

	if cfg.Storage.Type != "mongo" {
		t.Errorf("storage type: want mongo, got %q", cfg.Storage.Type)
	}
	if cfg.Storage.Mongo.URI != "mongodb://db:27017" {
		t.Errorf("mongo URI: want mongodb://db:27017, got %q", cfg.Storage.Mongo.URI)
	}
	if cfg.Storage.Mongo.Database != "mydb" {
		t.Errorf("mongo db: want mydb, got %q", cfg.Storage.Mongo.Database)
	}
}

func TestCollectInitConfig_RedisSession(t *testing.T) {
	input := makeInput(
		"",     // port default
		"",     // title
		"",     // subtitle
		"",     // storage default
		"redis", // session store
		"redis:6380", // addr
		"secret123",  // password
		"myapp:",      // key prefix
		"n",          // no AI
	)
	p := newPrompterFromReader(strings.NewReader(input), &bytes.Buffer{}, true)
	cfg := collectInitConfig(p)

	if cfg.Session.StoreType != "redis" {
		t.Errorf("session store: want redis, got %q", cfg.Session.StoreType)
	}
	if cfg.Session.Redis.Addr != "redis:6380" {
		t.Errorf("redis addr: want redis:6380, got %q", cfg.Session.Redis.Addr)
	}
}

func TestCollectInitConfig_OpenAI(t *testing.T) {
	input := makeInput(
		"",        // port
		"",        // title
		"",        // subtitle
		"",        // storage
		"",        // session
		"y",       // enable AI
		"openai",  // provider
		"sk-test", // key
		"gpt-4o",  // model
	)
	p := newPrompterFromReader(strings.NewReader(input), &bytes.Buffer{}, true)
	cfg := collectInitConfig(p)

	if cfg.AI.Provider != "openai" {
		t.Errorf("provider: want openai, got %q", cfg.AI.Provider)
	}
	if cfg.AI.OpenAI.APIKey != "sk-test" {
		t.Errorf("openai key: want sk-test, got %q", cfg.AI.OpenAI.APIKey)
	}
	if cfg.AI.OpenAI.Model != "gpt-4o" {
		t.Errorf("openai model: want gpt-4o, got %q", cfg.AI.OpenAI.Model)
	}
}

func TestCollectInitConfig_Claude(t *testing.T) {
	input := makeInput(
		"",            // port
		"",            // title
		"",            // subtitle
		"",            // storage
		"",            // session
		"y",           // enable AI
		"claude",      // provider
		"sk-ant-test", // key
		"",            // model default
	)
	p := newPrompterFromReader(strings.NewReader(input), &bytes.Buffer{}, true)
	cfg := collectInitConfig(p)

	if cfg.AI.Provider != "claude" {
		t.Errorf("provider: want claude, got %q", cfg.AI.Provider)
	}
	if cfg.AI.Claude.APIKey != "sk-ant-test" {
		t.Errorf("claude key: want sk-ant-test, got %q", cfg.AI.Claude.APIKey)
	}
}

func TestCollectInitConfig_CustomPort(t *testing.T) {
	input := makeInput(
		"3000", // custom port
		"",     // title
		"",     // subtitle
		"",     // storage
		"",     // session
		"n",    // no AI
	)
	p := newPrompterFromReader(strings.NewReader(input), &bytes.Buffer{}, true)
	cfg := collectInitConfig(p)

	if cfg.Server.Port != 3000 {
		t.Errorf("port: want 3000, got %d", cfg.Server.Port)
	}
}

// ── renderDefaultConfigYAML tests ─────────────────────────────────────────

func TestRenderConfigYAML_FileStorage(t *testing.T) {
	cfg := defaultInitConfig()
	yaml := renderDefaultConfigYAML(cfg)

	mustContain := []string{
		"port:",
		"storage:",
		`type: "file"`,
		"# mongo:",      // commented out when file storage
		"session:",
		"redis:",
		"ai:",
		"provider:",
	}
	for _, s := range mustContain {
		if !strings.Contains(yaml, s) {
			t.Errorf("YAML missing %q", s)
		}
	}
}

func TestRenderConfigYAML_MongoStorage(t *testing.T) {
	cfg := defaultInitConfig()
	cfg.Storage.Type = "mongo"
	cfg.Storage.Mongo.URI = "mongodb://localhost:27017"
	cfg.Storage.Mongo.Database = "testdb"

	yaml := renderDefaultConfigYAML(cfg)
	if !strings.Contains(yaml, `type: "mongo"`) {
		t.Error("YAML should contain mongo type")
	}
	if !strings.Contains(yaml, "mongodb://localhost:27017") {
		t.Error("YAML should contain mongo URI")
	}
	if !strings.Contains(yaml, "testdb") {
		t.Error("YAML should contain database name")
	}
}

func TestRenderConfigYAML_RedisSession(t *testing.T) {
	cfg := defaultInitConfig()
	cfg.Session.StoreType = "redis"
	cfg.Session.Redis.Addr = "redis:6379"
	cfg.Session.Redis.KeyPrefix = "app:"

	yaml := renderDefaultConfigYAML(cfg)
	if !strings.Contains(yaml, `storeType: "redis"`) {
		t.Error("YAML should contain redis storeType")
	}
	if !strings.Contains(yaml, "redis:6379") {
		t.Error("YAML should contain redis addr")
	}
}

// ── runInit integration test ───────────────────────────────────────────────

func TestRunInit_NonInteractive_CreatesFiles(t *testing.T) {
	dir := t.TempDir()

	// Override flags used by runInit.
	origPath := initPath
	origForce := initForce
	origNoInteractive := initNoInteractive
	defer func() {
		initPath = origPath
		initForce = origForce
		initNoInteractive = origNoInteractive
	}()

	initPath = dir
	initForce = false
	initNoInteractive = true

	if err := runInit(nil, nil); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	configFile := filepath.Join(dir, "config.yaml")
	if _, err := os.Stat(configFile); err != nil {
		t.Errorf("config.yaml not created: %v", err)
	}

	content, _ := os.ReadFile(configFile)
	if !strings.Contains(string(content), "port:") {
		t.Error("config.yaml missing port:")
	}
}

func TestRunInit_Force_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "config.yaml")
	_ = os.WriteFile(configFile, []byte("old"), 0644)

	origPath, origForce, origNI := initPath, initForce, initNoInteractive
	defer func() { initPath, initForce, initNoInteractive = origPath, origForce, origNI }()

	initPath = dir
	initForce = true
	initNoInteractive = true

	if err := runInit(nil, nil); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	content, _ := os.ReadFile(configFile)
	if string(content) == "old" {
		t.Error("config.yaml was not overwritten")
	}
}

func TestRunInit_NoForce_FailsOnExisting(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "config.yaml")
	_ = os.WriteFile(configFile, []byte("old"), 0644)

	origPath, origForce, origNI := initPath, initForce, initNoInteractive
	defer func() { initPath, initForce, initNoInteractive = origPath, origForce, origNI }()

	initPath = dir
	initForce = false
	initNoInteractive = true

	if err := runInit(nil, nil); err == nil {
		t.Error("expected error when config exists and --force not set")
	}
}

func TestCollectInitConfig_InteractiveNoAI_ClearsProvider(t *testing.T) {
// Explicitly saying "n" to AI in interactive mode should clear the provider.
input := makeInput(
"", // port
"", // title
"", // subtitle
"", // storage
"", // session
"n", // AI: NO
)
p := newPrompterFromReader(strings.NewReader(input), &bytes.Buffer{}, true)
cfg := collectInitConfig(p)

if cfg.AI.Provider != "" {
t.Errorf("expected provider cleared when AI disabled interactively, got %q", cfg.AI.Provider)
}
}
