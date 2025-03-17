package main

import (
	"fmt"
	"net"
	"net/http"

	log "github.com/sirupsen/logrus"

	"external-dns-yandex-webhook/internal/config"
	"external-dns-yandex-webhook/internal/yandex/client"
	"external-dns-yandex-webhook/internal/yandex/provider"

	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/provider/webhook/api"
)

func main() {
	log.SetLevel(log.DebugLevel)

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}

	startedChan := make(chan struct{})
	httpApiStarted := false

	go func() {
		<-startedChan
		log.Debugf("Webhook server started on port: :%d", cfg.Server.WebhookPort)
		httpApiStarted = true
	}()

	log.Infof("Using folder ID: %s", cfg.FolderID)

	m := http.NewServeMux()
	m.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if !httpApiStarted {
			w.WriteHeader(http.StatusInternalServerError)
		}
	})

	go func() {
		log.Debugf("Starting health server on port: :%d", cfg.Server.HealthPort)
		s := &http.Server{
			Addr:    fmt.Sprintf("0.0.0.0:%d", cfg.Server.HealthPort),
			Handler: m,
		}

		l, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", cfg.Server.HealthPort))
		if err != nil {
			log.Fatal(err)
		}
		err = s.Serve(l)
		if err != nil {
			log.Fatalf("health listener stopped: %s", err)
		}
	}()

	client, err := client.NewYandexClient(cfg.FolderID, cfg.AuthKeyFile)
	if err != nil {
		log.Fatalf("NewYandexClient: %v", err)
	}

	epf := endpoint.NewDomainFilter([]string{})
	provider := provider.NewYandexProvider(epf, false, client)
	if err != nil {
		log.Fatalf("NewYandexProvider: %v", err)
	}
	api.StartHTTPApi(provider, startedChan, 0, 0, fmt.Sprintf(":%d", cfg.Server.WebhookPort))
}
