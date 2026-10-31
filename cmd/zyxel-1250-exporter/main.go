package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/xfoucron/zyxel-1250-exporter/internal/collector"
	"github.com/xfoucron/zyxel-1250-exporter/internal/zyxel"
)

const maxConcurrentScrapes = 2

var version = "dev"

type options struct {
	switchURL   string
	listen      string
	timeout     time.Duration
	insecure    bool
	showVersion bool
}

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(os.Args[1:], log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(args []string, log *slog.Logger) error {
	var opts options
	fs := flag.NewFlagSet("zyxel-1250-exporter", flag.ContinueOnError)
	fs.StringVar(&opts.switchURL, "zyxel.url", envOr("ZYXEL_URL", "http://192.168.1.3"), "base URL of the switch web interface")
	fs.BoolVar(&opts.insecure, "zyxel.insecure-skip-verify", os.Getenv("ZYXEL_INSECURE_SKIP_VERIFY") == "true", "do not verify the TLS certificate of the switch (it is self-signed)")
	fs.StringVar(&opts.listen, "listen", envOr("ZYXEL_LISTEN", ":9092"), "address to listen on")
	fs.DurationVar(&opts.timeout, "scrape-timeout", 10*time.Second, "timeout of a single scrape")
	fs.BoolVar(&opts.showVersion, "version", false, "print the version and exit")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if opts.showVersion {
		fmt.Println(version)
		return nil
	}

	password := os.Getenv("ZYXEL_PASSWORD")
	if password == "" {
		return errors.New("ZYXEL_PASSWORD is not set")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return serve(ctx, opts, password, log)
}

func serve(ctx context.Context, opts options, password string, log *slog.Logger) error {
	httpClient := &http.Client{Timeout: opts.timeout}
	if opts.insecure {
		httpClient.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} //nolint:gosec // opt-in, the switch only has a self-signed certificate.
	}
	client, err := zyxel.New(opts.switchURL, password, httpClient)
	if err != nil {
		return err
	}

	registry := prometheus.NewRegistry()
	buildInfo := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "zyxel_exporter_build_info",
		Help:        "Build information of the exporter.",
		ConstLabels: prometheus.Labels{"version": version},
	})
	buildInfo.Set(1)
	registry.MustRegister(
		buildInfo,
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		collector.New(client, opts.timeout, log),
	)

	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{Registry: registry, MaxRequestsInFlight: maxConcurrentScrapes}))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	srv := &http.Server{
		Addr:              opts.listen,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		if err := client.Logout(shutdownCtx); err != nil {
			log.Warn("logout failed", "err", err)
		}
	}()

	log.Info("listening", "addr", opts.listen, "version", version, "switch", opts.switchURL)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	<-done
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
