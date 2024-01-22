package main

import (
	"crypto/rand"
	"crypto/rsa"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/dehydr8/kasa-go/exporter"
	"github.com/dehydr8/kasa-go/protocol"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"

	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type MetricsServer struct {
	key           *rsa.PrivateKey
	credentials   *protocol.Credentials
	registryCache *lru.Cache[string, *prometheus.Registry]

	logger log.Logger
}

func NewMetricsServer(key *rsa.PrivateKey, credentials *protocol.Credentials, logger log.Logger) *MetricsServer {
	cache, err := lru.New[string, *prometheus.Registry](10)

	if err != nil {
		panic(err)
	}

	return &MetricsServer{
		key:           key,
		credentials:   credentials,
		registryCache: cache,
		logger:        logger,
	}
}

func (s *MetricsServer) getOrCreate(key string, create func() (*prometheus.Registry, error)) (*prometheus.Registry, error) {
	if value, ok := s.registryCache.Get(key); ok {
		return value, nil
	}

	value, err := create()

	if err != nil {
		return nil, err
	}

	s.registryCache.Add(key, value)

	return value, nil
}

func main() {
	lvl := flag.String("log", "info", "debug, info, warn, error")
	address := flag.String("address", "", "address to listen on")
	port := flag.Int("port", 9500, "port to listen on")
	username := flag.String("username", "", "username for kasa login")
	password := flag.String("password", "", "password for kasa login")

	flag.Parse()

	if *username == "" || *password == "" {
		panic("username and password must be specified")
	}

	var logger log.Logger
	logger = log.NewLogfmtLogger(os.Stderr)
	logger = level.NewFilter(logger, level.Allow(level.ParseDefault(*lvl, level.InfoValue())))
	logger = log.With(logger, "ts", log.TimestampFormat(
		func() time.Time { return time.Now() },
		time.DateTime,
	), "caller", log.DefaultCaller)

	credentials := protocol.Credentials{
		Username: *username,
		Password: *password,
	}

	level.Debug(logger).Log("msg", "Generating RSA key")

	key, err := rsa.GenerateKey(rand.Reader, 1024)

	if err != nil {
		panic(err)
	}

	server := NewMetricsServer(key, &credentials, logger)

	http.HandleFunc("/scrape", server.ScrapeHandler)

	level.Info(logger).Log("msg", "Starting metrics server", "address", fmt.Sprintf("%s:%d", *address, *port))

	http.ListenAndServe(fmt.Sprintf("%s:%d", *address, *port), nil)
}

func (s *MetricsServer) ScrapeHandler(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("target")
	if target == "" {
		http.Error(w, "'target' parameter must be specified", 400)
		return
	}

	registry, err := s.getOrCreate(target, func() (*prometheus.Registry, error) {

		level.Debug(s.logger).Log("msg", "Creating new registry for target", "target", target)

		transport, err := protocol.NewAesTransport(s.key, &protocol.DeviceConfig{
			Address:     target,
			Credentials: s.credentials,
		}, s.logger)

		if err != nil {
			return nil, err
		}

		exporter, err := exporter.NewPlugExporter(target, transport, s.logger)

		if err != nil {
			return nil, err
		}

		registry := prometheus.NewRegistry()
		registry.MustRegister(exporter)

		return registry, nil
	})

	if err != nil {
		level.Error(s.logger).Log("msg", "Error creating registry", "err", err)
		http.Error(w, err.Error(), 500)
		return
	}

	promhttp.HandlerFor(registry, promhttp.HandlerOpts{}).ServeHTTP(w, r)
}
