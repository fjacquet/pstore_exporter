// PowerStore Exporter collects metrics from Dell PowerStore arrays and exposes them
// via a Prometheus /metrics endpoint and an optional OTLP metric push.
//
// Usage:
//
//	pstore_exporter --config config.yaml [--debug] [--once] [--trace]
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fjacquet/pstore_exporter/internal/config"
	"github.com/fjacquet/pstore_exporter/internal/logging"
	"github.com/fjacquet/pstore_exporter/internal/models"
	"github.com/fjacquet/pstore_exporter/internal/powerstore"
	"github.com/fjacquet/pstore_exporter/internal/telemetry"
	"github.com/fjacquet/pstore_exporter/internal/utils"
	"github.com/fsnotify/fsnotify"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/otel/trace"
)

const (
	programName       = "pstore_exporter"
	shutdownTimeout   = 15 * time.Second
	readHeaderTimeout = 5 * time.Second
)

// version is the build version, injected via -ldflags "-X main.version=...".
var version = "dev"

var (
	configFile string
	debug      bool
	once       bool
	// traceAPI backs --trace ("trace" the name would shadow the otel trace
	// import). It is read by buildClients on startup AND on every config
	// reload, so the tracing transport survives client-pool rebuilds.
	traceAPI bool
)

// Server owns the HTTP server, the snapshot store, the collection loop, the per-array
// clients, and the dual export paths.
type Server struct {
	cfg        *models.SafeConfig
	configPath string

	httpSrv  *http.Server
	registry *prometheus.Registry
	store    *powerstore.SnapshotStore

	telemetry      *telemetry.Manager
	tracerProvider trace.TracerProvider

	mu            sync.Mutex // guards clients/collector/collectCancel across reloads
	clients       []powerstore.Client
	collector     *powerstore.Collector
	collectCancel context.CancelFunc
	otlp          *powerstore.OTLPExporter

	configWatcher *fsnotify.Watcher
	serverErrChan chan error
}

// NewServer creates a server bound to the given thread-safe config.
func NewServer(safeCfg *models.SafeConfig, configPath string) *Server {
	return &Server{
		cfg:           safeCfg,
		configPath:    configPath,
		registry:      prometheus.NewRegistry(),
		store:         powerstore.NewSnapshotStore(),
		serverErrChan: make(chan error, 1),
	}
}

// Start initializes telemetry and starts the HTTP server immediately so /metrics
// and /health are served from the empty-seeded store without waiting on array
// client construction or the first collection. Building the clients, running the
// initial collection, wiring the OTLP path, and starting the background loop all
// happen in a background goroutine so nothing user-facing blocks ~80s on an
// unreachable array.
//
// In --once mode there is no server: clients are built, one cycle runs
// synchronously, and the caller exits. That one-shot path may block, which is fine.
func (s *Server) Start() error {
	cfg := s.cfg.Get()

	s.initTracing(cfg)

	if once {
		// One-shot: build clients and run a single synchronous cycle, no server.
		return s.startCollection(context.Background(), cfg)
	}

	if err := s.registry.Register(powerstore.NewPromCollector(s.store)); err != nil {
		return fmt.Errorf("failed to register collector: %w", err)
	}

	if err := s.registry.Register(powerstore.NewBuildInfoCollector(version, runtime.Version())); err != nil {
		return fmt.Errorf("failed to register build info collector: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle(cfg.Server.URI, promhttp.HandlerFor(s.registry, promhttp.HandlerOpts{}))
	mux.HandleFunc("/health", s.healthHandler)

	s.httpSrv = &http.Server{
		Addr:              cfg.GetServerAddress(),
		Handler:           mux,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	// Start listening first: the empty-seeded store lets /health return
	// 200 "starting" and /metrics serve (with no powerstore_up series yet).
	go func() {
		log.Infof("Starting %s on %s%s", programName, cfg.GetServerAddress(), cfg.Server.URI)
		if err := s.httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.serverErrChan <- fmt.Errorf("HTTP server error: %w", err)
		}
	}()

	// Build clients, run the initial collection, wire OTLP, and start the loop
	// in the background so client construction (which performs a login not bound
	// by the collection timeout) never delays the server coming up.
	go func() {
		if err := s.startCollection(context.Background(), cfg); err != nil {
			log.Errorf("Initial collection setup failed: %v", err)
			return
		}
		if err := s.initOTLP(cfg); err != nil {
			log.Warnf("OTLP metrics export disabled: %v", err)
		}
	}()

	return nil
}

// initTracing sets up the optional OpenTelemetry tracer provider.
func (s *Server) initTracing(cfg *models.Config) {
	if !cfg.IsOTelTracingEnabled() {
		return
	}
	mgr := telemetry.NewManager(telemetry.Config{
		Endpoint:       cfg.OpenTelemetry.Tracing.Endpoint,
		Insecure:       cfg.OpenTelemetry.Tracing.Insecure,
		SamplingRate:   cfg.OpenTelemetry.Tracing.SamplingRate,
		ServiceName:    "pstore-exporter",
		ServiceVersion: version,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := mgr.Initialize(ctx); err != nil {
		log.Warnf("Failed to initialize tracing: %v. Continuing without tracing.", err)
		return
	}
	s.telemetry = mgr
	s.tracerProvider = mgr.TracerProvider()
}

// startCollection builds the client pool and collector, runs an initial synchronous
// cycle, and starts the background loop. Caller must hold no locks.
func (s *Server) startCollection(ctx context.Context, cfg *models.Config) error {
	clients, err := buildClients(cfg)
	if err != nil {
		return err
	}
	collector := powerstore.NewCollector(clients, s.store, cfg.GetCollectionInterval(), cfg.GetCollectionTimeout(), s.tracerProvider)

	// Initial synchronous cycle so the first scrape isn't empty.
	initCtx, cancel := context.WithTimeout(ctx, cfg.GetCollectionTimeout()+5*time.Second)
	collector.CollectOnce(initCtx)
	cancel()

	if once {
		s.mu.Lock()
		s.clients = clients
		s.collector = collector
		s.mu.Unlock()
		return nil
	}

	loopCtx, loopCancel := context.WithCancel(context.Background())
	go collector.Run(loopCtx)

	s.mu.Lock()
	s.clients = clients
	s.collector = collector
	s.collectCancel = loopCancel
	s.mu.Unlock()
	return nil
}

// initOTLP sets up the OTLP metric push path if enabled. The exporter's periodic
// reader pushes automatically once instruments are registered, so there is no
// explicit start step beyond EnsureInstruments.
func (s *Server) initOTLP(cfg *models.Config) error {
	if !cfg.IsOTelMetricsEnabled() {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exp, err := powerstore.NewOTLPExporter(ctx, cfg.OpenTelemetry.Metrics, s.store, version)
	if err != nil {
		return err
	}
	if err := exp.EnsureInstruments(); err != nil {
		_ = exp.Shutdown(ctx)
		return err
	}
	s.otlp = exp

	// Metric names absent during the first cycle get no instruments at init, so
	// refresh instruments after every collection cycle (EnsureInstruments is
	// idempotent — it tracks an already-registered set).
	s.mu.Lock()
	collector := s.collector
	s.mu.Unlock()
	if collector != nil {
		collector.SetOnCycle(func() { _ = exp.EnsureInstruments() })
	}

	log.Infof("OTLP metrics push enabled to %s", cfg.OpenTelemetry.Metrics.Endpoint)
	return nil
}

// buildClients constructs one ArrayClient per configured array. An array whose client
// cannot be built is logged and skipped; if no clients build at all, startup fails.
func buildClients(cfg *models.Config) ([]powerstore.Client, error) {
	clients := make([]powerstore.Client, 0, len(cfg.Arrays))
	fleetConcurrency := cfg.GetMaxConcurrency()
	for _, a := range cfg.Arrays {
		client, err := powerstore.NewArrayClient(a, a.MaxConcurrencyOr(fleetConcurrency), traceAPI)
		if err != nil {
			log.Warnf("array %q: failed to create client, skipping: %v", a.Name, err)
			continue
		}
		clients = append(clients, client)
	}
	if len(clients) == 0 {
		return nil, errors.New("no usable array clients could be created from configuration")
	}
	return clients, nil
}

// dumpSamples prints every collected sample in Prometheus exposition style,
// sorted, so a `--once --debug` run against a live array can be diffed against
// docs/metrics.md to spot silently-absent metrics. Samples go to stdout as
// plain lines (`powerstore_...`), distinguishable from the JSON log records.
func dumpSamples(snap *powerstore.Snapshot) {
	var lines []string
	for _, as := range snap.PerArray {
		for _, s := range as.Samples {
			parts := make([]string, 0, len(s.Labels))
			for _, l := range s.Labels {
				parts = append(parts, fmt.Sprintf("%s=%q", l.Name, l.Value))
			}
			lines = append(lines, fmt.Sprintf("%s{%s} %v", s.Name, strings.Join(parts, ","), s.Value))
		}
	}
	sort.Strings(lines)
	for _, l := range lines {
		fmt.Println(l)
	}
}

// ErrorChan returns the channel that receives fatal HTTP server errors.
func (s *Server) ErrorChan() <-chan error { return s.serverErrChan }

// ReloadConfig reloads configuration; when the array set changes it rebuilds the
// client pool and swaps it into the running collector.
func (s *Server) ReloadConfig(configPath string) error {
	arraysChanged, err := s.cfg.ReloadConfig(configPath)
	if err != nil {
		return err
	}
	if !arraysChanged {
		return nil
	}

	log.Info("Array set changed; rebuilding client pool")
	newClients, err := buildClients(s.cfg.Get())
	if err != nil {
		return fmt.Errorf("failed to rebuild clients after reload: %w", err)
	}

	s.mu.Lock()
	oldClients := s.clients
	s.clients = newClients
	collector := s.collector
	s.mu.Unlock()

	if collector != nil {
		collector.SetClients(newClients)
	}
	for _, c := range oldClients {
		if err := c.Close(); err != nil {
			log.Debugf("client close during reload: %v", err)
		}
	}

	// New metric names (if any) need OTLP instruments registered.
	if s.otlp != nil {
		if err := s.otlp.EnsureInstruments(); err != nil {
			log.Warnf("Failed to register OTLP instruments after reload: %v", err)
		}
	}
	return nil
}

// stopCollection cancels the loop and closes the current client pool.
func (s *Server) stopCollection() {
	s.mu.Lock()
	cancel := s.collectCancel
	clients := s.clients
	s.collectCancel = nil
	s.clients = nil
	s.collector = nil
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	for _, c := range clients {
		if err := c.Close(); err != nil {
			log.Debugf("client close during shutdown: %v", err)
		}
	}
}

// SetConfigWatcher stores the file watcher for cleanup at shutdown.
func (s *Server) SetConfigWatcher(w *fsnotify.Watcher) { s.configWatcher = w }

// Shutdown stops the watcher, HTTP server, collection loop, exporters, and clients.
func (s *Server) Shutdown() error {
	if s.configWatcher != nil {
		if err := s.configWatcher.Close(); err != nil {
			log.Warnf("Config watcher close warning: %v", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if s.httpSrv != nil {
		log.Info("Shutting down HTTP server...")
		if err := s.httpSrv.Shutdown(ctx); err != nil {
			log.Warnf("HTTP server shutdown warning: %v", err)
		}
	}

	s.stopCollection()

	if s.otlp != nil {
		if err := s.otlp.Shutdown(ctx); err != nil {
			log.Warnf("OTLP shutdown warning: %v", err)
		}
	}

	if s.telemetry != nil {
		if err := s.telemetry.Shutdown(ctx); err != nil {
			log.Warnf("Telemetry shutdown warning: %v", err)
		}
	}

	close(s.serverErrChan)
	log.Info("Server stopped gracefully")
	return nil
}

// healthHandler reports 200 if any array was scraped successfully, 503 if all are
// down, and 200 "starting" before the first cycle populates the store.
func (s *Server) healthHandler(w http.ResponseWriter, _ *http.Request) {
	snap := s.store.Load()
	if len(snap.PerArray) == 0 {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintln(w, "OK (starting)")
		return
	}
	for _, as := range snap.PerArray {
		if as.Up {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintln(w, "OK")
			return
		}
	}
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = fmt.Fprintln(w, "UNHEALTHY: all arrays unreachable")
}

func validateConfig(configPath string) (*models.Config, error) {
	if !utils.FileExists(configPath) {
		return nil, fmt.Errorf("config file not found: %s", configPath)
	}
	var cfg models.Config
	if err := utils.ReadFile(&cfg, configPath); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	return &cfg, nil
}

func setupLogging(cfg models.Config, debugMode bool) error {
	if err := logging.PrepareLogs(cfg.Server.LogName); err != nil {
		return fmt.Errorf("failed to initialize logging: %w", err)
	}
	if debugMode {
		log.SetLevel(log.DebugLevel)
		log.Debug("Debug mode enabled")
	}
	return nil
}

func waitForShutdown(serverErr <-chan error) error {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-stop:
		log.Infof("Received signal %v, initiating graceful shutdown...", sig)
		return nil
	case err := <-serverErr:
		return err
	}
}

func main() {
	rootCmd := &cobra.Command{
		Use:     programName,
		Version: version,
		Short:   "Prometheus/OTLP exporter for Dell PowerStore",
		Long:    "PowerStore Exporter collects metrics from Dell PowerStore arrays and exposes them via Prometheus and OTLP.",
		RunE: func(_ *cobra.Command, _ []string) error {
			utils.LoadDotEnv(configFile)

			cfg, err := validateConfig(configFile)
			if err != nil {
				return err
			}

			safeCfg := models.NewSafeConfig(cfg, utils.ResolveSecrets)
			if err := setupLogging(*safeCfg.Get(), debug); err != nil {
				return err
			}

			log.Infof("Starting %s...", programName)
			log.Infof("Monitoring %d array(s)", len(safeCfg.Get().Arrays))

			server := NewServer(safeCfg, configFile)
			if err := server.Start(); err != nil {
				return err
			}

			if once {
				if debug {
					dumpSamples(server.store.Load())
				}
				log.Info("--once: single collection cycle complete, exiting")
				return server.Shutdown()
			}

			config.SetupSIGHUPHandler(configFile, server.ReloadConfig)
			if watcher, err := config.WatchConfigFile(configFile, server.ReloadConfig); err != nil {
				log.Warnf("File watcher setup failed: %v. SIGHUP reload still available.", err)
			} else {
				server.SetConfigWatcher(watcher)
			}

			if err := waitForShutdown(server.ErrorChan()); err != nil {
				log.Errorf("Server error: %v", err)
			}
			return server.Shutdown()
		},
	}

	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "Path to configuration file (required)")
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "Enable debug mode")
	rootCmd.PersistentFlags().BoolVar(&once, "once", false, "Run a single collection cycle and exit")
	rootCmd.PersistentFlags().BoolVar(&traceAPI, "trace", false,
		"Log raw bulk-API response bodies (method/URL/status + body; headers are never logged). "+
			"Typed gopowerstore calls cannot be traced (no transport seam in the SDK). Very verbose.")
	_ = rootCmd.MarkPersistentFlagRequired("config")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
