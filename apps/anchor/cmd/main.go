package main

import (
	"flag"

	"anchor/cmd/app"
)

func main() {
	localTunnel := flag.Bool("local-tunnel", false, "launch local cloudflared tunnel")
	flag.Parse()

	app.StartAnchorWithOptions(app.StartOptions{LocalTunnel: *localTunnel})
}
