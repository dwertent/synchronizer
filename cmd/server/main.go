package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/gobwas/ws"
	pulsarconnector "github.com/kubescape/messaging/pulsar/connector"
	"github.com/kubescape/synchronizer/utils"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/kubescape/go-logger"
	"github.com/kubescape/go-logger/helpers"
	"github.com/kubescape/synchronizer/adapters"
	"github.com/kubescape/synchronizer/adapters/backend/v1"
	"github.com/kubescape/synchronizer/cmd/server/authentication"

	"github.com/kubescape/synchronizer/config"
	"github.com/kubescape/synchronizer/core"
)

func main() {
	ctx := context.Background()

	// load config
	cfg, err := config.LoadConfig("/etc/config")
	if err != nil {
		logger.L().Fatal("unable to load configuration", helpers.Error(err))
	}

	// backend adapter
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// enable prometheus metrics
	if _, present := os.LookupEnv("PROMETHEUS_METRICS"); present {
		go func() {
			logger.L().Info("prometheus metrics enabled - http://localhost:2112")
			http.Handle("/metrics", promhttp.Handler())
			_ = http.ListenAndServe(":2112", nil)
		}()
	}

	var adapter adapters.Adapter
	if cfg.Backend.PulsarConfig != nil {
		logger.L().Info("initializing pulsar client")
		pulsarClient, err := pulsarconnector.NewClient(
			pulsarconnector.WithConfig(cfg.Backend.PulsarConfig),
		)
		if err != nil {
			logger.L().Fatal("failed to create pulsar client", helpers.Error(err), helpers.String("config", fmt.Sprintf("%+v", cfg.Backend.PulsarConfig)))
		}
		defer pulsarClient.Close()

		pulsarProducer, err := backend.NewPulsarMessageProducer(cfg, pulsarClient)
		if err != nil {
			logger.L().Fatal("failed to create pulsar producer", helpers.Error(err), helpers.String("config", fmt.Sprintf("%+v", cfg.Backend.PulsarConfig)))
		}

		pulsarConsumer, err := backend.NewPulsarMessageConsumer(cfg, pulsarClient)
		if err != nil {
			logger.L().Fatal("failed to create pulsar consumer", helpers.Error(err), helpers.String("config", fmt.Sprintf("%+v", cfg.Backend.PulsarConfig)))
		}

		adapter = backend.NewBackendAdapter(ctx, pulsarProducer, pulsarConsumer)
	} else {
		// mock adapter
		logger.L().Info("initializing mock adapter")
		adapter = adapters.NewMockAdapter(false)
	}

	// start pprof server
	utils.ServePprof()

	// start liveness probe
	utils.StartLivenessProbe()

	// websocket server
	_ = http.ListenAndServe(":8080",
		authentication.AuthenticationServerMiddleware(cfg.Backend.AuthenticationServer,
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, _, _, err := ws.UpgradeHTTP(r, w)
				if err != nil {
					logger.L().Error("unable to upgrade connection", helpers.Error(err))
					return
				}
				go func() {
					defer conn.Close()
					synchronizer := core.NewSynchronizerServer(r.Context(), adapter, conn)
					err = synchronizer.Start(r.Context())
					if err != nil {
						id := utils.ClientIdentifierFromContext(r.Context())
						logger.L().Error("error during sync, closing listener",
							helpers.String("account", id.Account),
							helpers.String("cluster", id.Cluster),
							helpers.Error(err))
						err := synchronizer.Stop(r.Context())
						if err != nil {
							logger.L().Error("error during sync stop", helpers.Error(err))
						}
						return
					}
				}()
			})))
}
