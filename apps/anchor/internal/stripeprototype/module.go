package stripeprototype

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"time"

	"go.uber.org/fx"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 15 * time.Second
	writeTimeout      = 180 * time.Second
	idleTimeout       = 60 * time.Second
)

type RuntimeConfig struct {
	AccountID          string `json:"account_id"`
	ProductID          string `json:"product_id"`
	ProductName        string `json:"product_name"`
	AnchorURL          string `json:"anchor_url"`
	AnchorAPIKey       string `json:"anchor_api_key"`
	FallbackTemplateID string `json:"fallback_template_id"`
	WebhookSecret      string `json:"webhook_secret"`
	StatePath          string `json:"state_path"`
	ReturnURL          string `json:"return_url"`
	ListenAddress      string `json:"listen_address"`
}

func LoadRuntimeConfig(path string) (RuntimeConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return RuntimeConfig{}, errors.New("unable to read the private prototype configuration file")
	}
	var config RuntimeConfig
	if err = json.Unmarshal(data, &config); err != nil {
		return RuntimeConfig{}, errors.New("invalid prototype configuration file")
	}
	host, _, err := net.SplitHostPort(config.ListenAddress)
	if err != nil || host != loopbackAddress {
		return RuntimeConfig{}, errors.New("billing prototype must listen on 127.0.0.1")
	}
	return config, nil
}

func Module(config RuntimeConfig, specification []byte) fx.Option {
	return fx.Module("stripe-billing-prototype",
		fx.Provide(func() (*Store, error) {
			return OpenStore(config.StatePath, config.AccountID, config.ProductID, config.FallbackTemplateID)
		}),
		fx.Provide(func() (StripeGateway, error) { return NewCLI(config.AccountID, nil) }),
		fx.Provide(func() (AnchorGateway, error) {
			return NewAnchorGateway(
				config.AnchorURL,
				config.ProductID,
				config.AnchorAPIKey,
				WithAnchorProductName(config.ProductName),
			)
		}),
		fx.Provide(func(stripe StripeGateway, anchor AnchorGateway, store *Store) (*Service, error) {
			return NewService(
				Config{
					ExpectedAccountID: config.AccountID,
					ProductID:         config.ProductID,
					ReturnURL:         config.ReturnURL,
					WebhookSecret:     config.WebhookSecret,
				},
				stripe,
				anchor,
				store,
			)
		}),
		fx.Invoke(func(lifecycle fx.Lifecycle, shutdowner fx.Shutdowner, service *Service, store *Store) error {
			handler, err := NewHTTPHandler(service, specification)
			if err != nil {
				return err
			}
			server := &http.Server{
				Addr:              config.ListenAddress,
				Handler:           handler,
				ReadHeaderTimeout: readHeaderTimeout,
				ReadTimeout:       readTimeout,
				WriteTimeout:      writeTimeout,
				IdleTimeout:       idleTimeout,
			}
			workerContext, cancel := context.WithCancel(context.Background())
			workerDone, serverDone := make(chan struct{}), make(chan struct{})
			lifecycle.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					if _, accountErr := service.stripe.Account(ctx); accountErr != nil {
						return accountErr
					}
					if _, snapshotErr := service.anchor.Snapshot(ctx); snapshotErr != nil {
						return snapshotErr
					}
					listener, listenErr := (&net.ListenConfig{}).Listen(ctx, "tcp", server.Addr)
					if listenErr != nil {
						return listenErr
					}
					go func() {
						defer close(serverDone)
						if serveErr := server.Serve(
							listener,
						); serveErr != nil &&
							!errors.Is(serveErr, http.ErrServerClosed) {
							_ = shutdowner.Shutdown(fx.ExitCode(1))
						}
					}()
					go func() {
						defer close(workerDone)
						if queueErr := service.queueLinkedOrganizations(); queueErr != nil {
							_ = shutdowner.Shutdown(fx.ExitCode(1))
							return
						}
						ticker := time.NewTicker(time.Second)
						sweep := time.NewTicker(time.Minute)
						defer sweep.Stop()
						defer ticker.Stop()
						for {
							select {
							case <-workerContext.Done():
								return
							case <-sweep.C:
								if queueErr := service.queueLinkedOrganizations(); queueErr != nil {
									_ = shutdowner.Shutdown(fx.ExitCode(1))
									return
								}
							case <-ticker.C:
								if processErr := service.ProcessPending(
									workerContext,
								); processErr != nil &&
									workerContext.Err() == nil {
									_ = shutdowner.Shutdown(fx.ExitCode(1))
									return
								}
							}
						}
					}()
					return nil
				},
				OnStop: func(ctx context.Context) error {
					cancel()
					shutdownErr := server.Shutdown(ctx)
					select {
					case <-workerDone:
					case <-ctx.Done():
						return ctx.Err()
					}
					select {
					case <-serverDone:
					case <-ctx.Done():
						return ctx.Err()
					}
					return errors.Join(shutdownErr, store.Close())
				},
			})
			return nil
		}),
	)
}
