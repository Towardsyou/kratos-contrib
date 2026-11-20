package grafana

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
)

func TestLogEndpoint(t *testing.T) {
	var exports atomic.Int32
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/otlp/v1/logs" {
			t.Errorf("unexpected OTLP request: %s %s", r.Method, r.URL.Path)
		}
		if user, password, ok := r.BasicAuth(); !ok || user != "instance" || password != "test-key" {
			t.Error("missing configured Grafana authorization")
		}
		exports.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer collector.Close()
	t.Setenv("OTEL_EXPORTER_OTLP_LOGS_ENDPOINT", collector.URL)
	endpoint := collector.Listener.Addr().String()

	for _, tc := range []struct {
		name string
		cfg  LoggerConfig
		want int32
	}{
		{name: "empty"},
		{name: "configured", cfg: LoggerConfig{Endpoint: endpoint, Insecure: true, InstanceID: "instance", APIKey: "test-key"}, want: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			provider, shutdown, err := InitLoggerProvider(ctx, "test", "1", tc.cfg)
			if err != nil {
				t.Fatal(err)
			}
			var stdout bytes.Buffer
			logger := NewLogger(log.NewStdLogger(&stdout), provider)
			if err := logger.Log(log.LevelInfo, "msg", "upload example"); err != nil {
				t.Fatal(err)
			}
			if err := shutdown(ctx); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(stdout.String(), "upload example") {
				t.Errorf("stdout missing message: %q", stdout.String())
			}
			if got := exports.Load(); got != tc.want {
				t.Errorf("OTLP requests = %d, want %d", got, tc.want)
			}
		})
	}
}
