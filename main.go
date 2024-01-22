package main

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net/http"
	"os"

	"github.com/dehydr8/kasa-go/device"
	"github.com/dehydr8/kasa-go/exporter"
	"github.com/dehydr8/kasa-go/logger"
	"github.com/dehydr8/kasa-go/model"
	"github.com/dehydr8/kasa-go/util"
	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type MetricsServer struct {
	key           *rsa.PrivateKey
	credentials   *model.Credentials
	registryCache *lru.Cache[string, *prometheus.Registry]
}

func NewMetricsServer(key *rsa.PrivateKey, credentials *model.Credentials, cacheMax int) *MetricsServer {
	cache, err := lru.New[string, *prometheus.Registry](cacheMax)

	if err != nil {
		panic(err)
	}

	return &MetricsServer{
		key:           key,
		credentials:   credentials,
		registryCache: cache,
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
	fs := ff.NewFlagSet(fmt.Sprintf("kasa-exporter (rev %s)", util.Revision))

	var (
		lvl            = fs.StringEnum('l', "log", "log level: debug, info, warn, error", "info", "debug", "warn", "error")
		address        = fs.String('a', "address", "", "address to listen on")
		port           = fs.Int('p', "port", 9500, "port to listen on")
		username       = fs.StringLong("username", "", "username for kasa login")
		password       = fs.StringLong("password", "", "password for kasa login")
		hashedPassword = fs.StringLong("hashed_password", "", "hashed (sha1) password for kasa login")
		maxRegistries  = fs.IntLong("max_registries", 16, "maximum number of registries to cache")
	)

	if err := ff.Parse(fs, os.Args[1:],
		ff.WithEnvVarPrefix("KASA_EXPORTER"),
		ff.WithConfigFileFlag("config"),
		ff.WithConfigFileParser(ff.PlainParser),
	); err != nil {
		fmt.Printf("%s\n", ffhelp.Flags(fs))
		fmt.Printf("err=%v\n", err)
		os.Exit(1)
	}

	if *username == "" {
		fmt.Printf("%s\n", ffhelp.Flags(fs))
		fmt.Printf("err=%v\n", "username must be specified")
		os.Exit(1)
	}

	if *password == "" && *hashedPassword == "" {
		fmt.Printf("%s\n", ffhelp.Flags(fs))
		fmt.Printf("err=%v\n", "password or hashed_password must be specified")
		os.Exit(1)
	}

	logger.SetupLogging(*lvl)

	credentials := model.Credentials{
		Username:       *username,
		Password:       *password,
		HashedPassword: *hashedPassword,
	}

	logger.Debug("msg", "Generating RSA key")

	key, err := rsa.GenerateKey(rand.Reader, 1024)

	if err != nil {
		panic(err)
	}

	server := NewMetricsServer(key, &credentials, *maxRegistries)

	http.HandleFunc("/scrape", server.ScrapeHandler)

	logger.Info("msg", "Starting metrics server", "address", fmt.Sprintf("%s:%d", *address, *port))

	http.ListenAndServe(fmt.Sprintf("%s:%d", *address, *port), nil)
}

func (s *MetricsServer) ScrapeHandler(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("target")
	if target == "" {
		http.Error(w, "'target' parameter must be specified", 400)
		return
	}

	registry, err := s.getOrCreate(target, func() (*prometheus.Registry, error) {

		logger.Debug("msg", "Creating new registry for target", "target", target)

		device, err := device.NewDevice(s.key, &model.DeviceConfig{
			Address:     target,
			Credentials: s.credentials,
		})

		if err != nil {
			return nil, err
		}

		exporter, err := exporter.NewPlugExporter(device)

		if err != nil {
			return nil, err
		}

		registry := prometheus.NewRegistry()
		registry.MustRegister(exporter)

		return registry, nil
	})

	if err != nil {
		logger.Error("msg", "Error creating registry", "err", err)
		http.Error(w, err.Error(), 500)
		return
	}

	promhttp.HandlerFor(registry, promhttp.HandlerOpts{}).ServeHTTP(w, r)
}
