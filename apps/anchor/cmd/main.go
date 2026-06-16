package main

import (
	"flag"

	"anchor/cmd/app"

	"github.com/nanostack-dev/nanostack-framework/pkg/health"
)

func main() {
	localTunnel := flag.Bool("local-tunnel", false, "launch local cloudflared tunnel")
	healthcheck := flag.Bool(
		"healthcheck",
		false,
		"probe the local HTTP /health endpoint and exit (0 healthy, 1 unhealthy)",
	)
	flag.Parse()

	if *healthcheck {
		health.ProbeMain()
		return
	}

	app.StartAnchorWithOptions(app.StartOptions{LocalTunnel: *localTunnel})
}
