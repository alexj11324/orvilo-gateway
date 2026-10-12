package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	agentgateway "github.com/alexj11324/orvilo-gateway/agent-gateway-go"
	devicegateway "github.com/alexj11324/orvilo-gateway/device-gateway-go"
)

type service struct {
	name     string
	addr     string
	listen   func() error
	shutdown func(context.Context) error
}

type serviceResult struct {
	name string
	err  error
}

type gatewayConfigs struct {
	agent  agentgateway.Config
	device devicegateway.Config
}

func main() {
	os.Exit(run())
}

func run() int {
	configs, err := configsFromEnv()
	if err != nil {
		slog.Error("invalid configuration", "error", err)
		return 1
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stop)

	return runServices(newServices(configs), stop, configs.agent.ShutdownTimeout)
}

func configsFromEnv() (gatewayConfigs, error) {
	// The unified binary ignores PORT: each service gets its own variable so
	// the two listeners cannot collide in a single process.
	agentCfg := agentgateway.ConfigFromEnv()
	agentCfg.Port = envOrDefault("AGENT_PORT", "8787")
	if err := agentCfg.Validate(); err != nil {
		return gatewayConfigs{}, errors.New("invalid agent configuration: " + err.Error())
	}

	deviceCfg := devicegateway.ConfigFromEnv()
	deviceCfg.Port = envOrDefault("DEVICE_PORT", "8788")
	if err := deviceCfg.Validate(); err != nil {
		return gatewayConfigs{}, errors.New("invalid device configuration: " + err.Error())
	}

	return gatewayConfigs{agent: agentCfg, device: deviceCfg}, nil
}

func newServices(configs gatewayConfigs) []service {
	return []service{
		newService(
			"agent gateway",
			configs.agent.Port,
			agentgateway.NewServer(configs.agent).Routes(),
			configs.agent.ReadTimeout,
			configs.agent.WriteTimeout,
		),
		newService(
			"device gateway",
			configs.device.Port,
			devicegateway.NewServer(configs.device).Routes(),
			configs.device.ReadTimeout,
			configs.device.WriteTimeout,
		),
	}
}

func newService(name, port string, handler http.Handler, readTimeout, writeTimeout time.Duration) service {
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	}
	return service{
		name:     name,
		addr:     server.Addr,
		listen:   server.ListenAndServe,
		shutdown: server.Shutdown,
	}
}

func runServices(services []service, stop <-chan os.Signal, shutdownTimeout time.Duration) int {
	errCh := make(chan serviceResult, len(services))
	for _, svc := range services {
		go func() {
			slog.Info(svc.name+" listening", "addr", svc.addr)
			errCh <- serviceResult{name: svc.name, err: svc.listen()}
		}()
	}

	exitCode := 0
	select {
	case <-stop:
	case result := <-errCh:
		if result.err != nil && !errors.Is(result.err, http.ErrServerClosed) {
			slog.Error(result.name+" failed", "error", result.err)
			exitCode = 1
		}
	}

	if shutdownServices(services, shutdownTimeout) {
		exitCode = 1
	}
	return exitCode
}

func shutdownServices(services []service, timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	results := make(chan serviceResult, len(services))
	for _, svc := range services {
		go func() {
			results <- serviceResult{name: svc.name, err: svc.shutdown(ctx)}
		}()
	}

	failed := false
	for range services {
		select {
		case result := <-results:
			if result.err != nil {
				slog.Error(result.name+" shutdown failed", "error", result.err)
				failed = true
			}
		case <-ctx.Done():
			slog.Error("gateway shutdown timed out", "error", ctx.Err())
			return true
		}
	}
	return failed
}

func envOrDefault(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
