package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/prasenjit/go-virtual/internal/config"
	gvsync "github.com/prasenjit/go-virtual/internal/sync"
	"github.com/spf13/viper"
)

type testWatcher struct{ err error }

func (w testWatcher) Watch(context.Context, gvsync.Handler) error { return w.err }

type testLogger struct {
	infos  []string
	errors []string
}

func (l *testLogger) Info(msg string, _ ...any)  { l.infos = append(l.infos, msg) }
func (l *testLogger) Error(msg string, _ ...any) { l.errors = append(l.errors, msg) }

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	fn()
	_ = w.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	return string(out)
}

func TestPrintInitSummary(t *testing.T) {
	cfg := defaultInitConfig()
	cfg.Server.Port = 9090
	cfg.Storage.Type = "mongo"
	cfg.Storage.Mongo.URI = "mongodb://localhost:27017"
	cfg.Storage.Mongo.Database = "gv"
	cfg.Session.StoreType = "redis"
	cfg.Session.Redis.Addr = "redis:6379"
	cfg.Session.Redis.KeyPrefix = "gv:"
	cfg.AI.Provider = "openai"

	out := captureStdout(t, func() { printInitSummary(cfg) })
	for _, want := range []string{"Configuration summary", "9090", "mongo", "redis", "openai"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestBuildChangeHandler_NotifiesRoutes(t *testing.T) {
	routeNotifyCh := make(chan struct{}, 2)
	handler := buildChangeHandler(routeNotifyCh, nil)

	handler(gvsync.ChangeEvent{Collection: "specs", Operation: gvsync.ChangeOpInsert, DocumentID: "s1"})
	handler(gvsync.ChangeEvent{Collection: "operations", Operation: gvsync.ChangeOpReplace, DocumentID: "o1"})
	handler(gvsync.ChangeEvent{Collection: "global_store", Operation: gvsync.ChangeOpDelete, DocumentID: "k1"})
	handler(gvsync.ChangeEvent{Collection: "other", Operation: gvsync.ChangeOpInsert, DocumentID: "x"})

	if len(routeNotifyCh) != 2 {
		t.Fatalf("expected 2 route notifications, got %d", len(routeNotifyCh))
	}
}

func TestRunWatcher_LogsLifecycle(t *testing.T) {
	logger := &testLogger{}
	errBoom := errors.New("boom")
	runWatcher(context.Background(), testWatcher{err: errBoom}, func(gvsync.ChangeEvent) {}, logger, "polling")

	if len(logger.infos) != 1 {
		t.Fatalf("expected one info log, got %#v", logger.infos)
	}
	if len(logger.errors) != 1 {
		t.Fatalf("expected one error log, got %#v", logger.errors)
	}
}

func TestStartMongoSync_ModeOff(t *testing.T) {
	startMongoSync(context.Background(), config.MongoConfig{Sync: config.MongoSyncConfig{Mode: config.MongoSyncModeOff}}, nil, nil)
}

func TestMongoBackends_EmptyURI(t *testing.T) {
	cfg := config.MongoConfig{}
	if _, err := newMongoStorageBackend(cfg); err == nil {
		t.Fatal("expected mongo storage backend to reject empty URI")
	}
	if _, err := newMongoGlobalStoreBackend(cfg); err == nil {
		t.Fatal("expected mongo global store backend to reject empty URI")
	}
	if _, err := openWatcherDB(config.MongoConfig{URI: "://bad-uri", Database: "gv"}); err == nil {
		t.Fatal("expected openWatcherDB to reject malformed URI")
	}
}

func TestRunServe_GracefulShutdown(t *testing.T) {
	viper.Reset()
	defer viper.Reset()
	oldDev, oldPort, oldTLS, oldHeadless := devMode, portFlag, tlsFlag, headlessFlag
	defer func() {
		devMode, portFlag, tlsFlag, headlessFlag = oldDev, oldPort, oldTLS, oldHeadless
		signal.Reset(syscall.SIGINT, syscall.SIGTERM)
	}()

	setDefaults()
	viper.Set("server.host", "127.0.0.1")
	viper.Set("server.port", 0)
	viper.Set("server.headless", true)
	viper.Set("storage.type", config.StorageTypeMemory)
	viper.Set("storage.path", t.TempDir())
	viper.Set("tracing.maxTraces", 10)
	viper.Set("logging.level", config.LogLevelInfo)
	viper.Set("logging.format", config.LogFormatJSON)
	viper.Set("session.storeType", config.SessionStoreMemory)
	viper.Set("session.headerName", "X-Virtual-Session-Id")
	viper.Set("session.inactivityTimeout", "1m")
	viper.Set("session.maxSessions", 10)
	viper.Set("scripting.defaultTimeoutMs", 50)
	viper.Set("proxy.timeoutSeconds", 1)
	viper.Set("proxy.insecureSkipVerify", false)

	go func() {
		time.Sleep(200 * time.Millisecond)
		p, err := os.FindProcess(os.Getpid())
		if err == nil {
			_ = p.Signal(syscall.SIGTERM)
		}
	}()

	if err := runServe(nil, nil); err != nil {
		t.Fatalf("runServe: %v", err)
	}
}

func TestLogServerEndpoints(t *testing.T) {
	logger := &testLogger{}
	logServerEndpoints(logger, "127.0.0.1:8080", false, false)
	if len(logger.infos) != 4 {
		t.Fatalf("expected 4 info logs for non-headless HTTP, got %d", len(logger.infos))
	}

	logger = &testLogger{}
	logServerEndpoints(logger, "127.0.0.1:8443", true, true)
	if len(logger.infos) != 2 {
		t.Fatalf("expected 2 info logs for headless TLS, got %d", len(logger.infos))
	}
}

func TestStartHTTPServerAndTLSServer(t *testing.T) {
	t.Run("http", func(t *testing.T) {
		server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })}
		startHTTPServer(server, "127.0.0.1:0", true)
		time.Sleep(50 * time.Millisecond)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			t.Fatalf("Shutdown: %v", err)
		}
	})

	t.Run("tls", func(t *testing.T) {
		viper.Reset()
		defer viper.Reset()
		baseDir := t.TempDir()
		viper.Set("server.tls.autoGenerate", true)
		viper.Set("server.tls.storePath", filepath.Join(baseDir, "certs"))
		viper.Set("storage.path", baseDir)

		server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })}
		cleanup, err := startTLSServer(server, "127.0.0.1:0", true)
		if err != nil {
			t.Fatalf("startTLSServer: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := cleanup(ctx); err != nil {
			t.Fatalf("cleanup: %v", err)
		}
		if err := server.Shutdown(ctx); err != nil {
			t.Fatalf("Shutdown: %v", err)
		}
	})
}

func TestExecuteAndMain_Subprocesses(t *testing.T) {
	if os.Getenv("GO_VIRTUAL_HELPER_EXECUTE") == "1" {
		rootCmd.SetArgs([]string{"definitely-invalid-command"})
		Execute()
		return
	}
	if os.Getenv("GO_VIRTUAL_HELPER_MAIN") == "1" {
		os.Args = []string{os.Args[0], "definitely-invalid-command"}
		rootCmd.SetArgs(os.Args[1:])
		main()
		return
	}

	for _, tc := range []struct {
		name string
		env  string
	}{
		{name: "execute", env: "GO_VIRTUAL_HELPER_EXECUTE"},
		{name: "main", env: "GO_VIRTUAL_HELPER_MAIN"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestExecuteAndMain_Subprocesses$")
			cmd.Env = append(os.Environ(), tc.env+"=1")
			err := cmd.Run()
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				t.Fatalf("expected subprocess exit error, got %v", err)
			}
			if exitErr.ExitCode() != 1 {
				t.Fatalf("expected exit code 1, got %d", exitErr.ExitCode())
			}
		})
	}
}

func TestInitConfig_UsesWorkingDirectoryFallback(t *testing.T) {
	viper.Reset()
	defer viper.Reset()
	cfgFile = ""
	oldwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(oldwd) })
	t.Chdir(t.TempDir())
	initConfig()
	if got := viper.GetString("server.host"); got == "" {
		t.Fatal("expected defaults to be applied when no config file exists")
	}
}

func TestHelperPlaceholderForSignalImport(_ *testing.T) {
	_ = syscall.SIGTERM
}
