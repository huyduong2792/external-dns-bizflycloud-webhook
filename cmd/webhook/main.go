package main

import (
	"fmt"

	"github.com/bizflycloud/external-dns-bizflycloud-webhook/cmd/webhook/init/configuration"
	"github.com/bizflycloud/external-dns-bizflycloud-webhook/cmd/webhook/init/dnsprovider"
	"github.com/bizflycloud/external-dns-bizflycloud-webhook/cmd/webhook/init/logging"
	"github.com/bizflycloud/external-dns-bizflycloud-webhook/cmd/webhook/init/server"
	"github.com/bizflycloud/external-dns-bizflycloud-webhook/pkg/webhook"
	log "github.com/sirupsen/logrus"
)

const banner = `
  ____ ___ __________ _  __   __   ____ _     ___  _   _ ____  
 | __ )_ _|__  /  ___| | \ \ / /  / ___| |   / _ \| | | |  _ \ 
 |  _ \| |  / /| |_  | |  \ V /  | |   | |  | | | | | | | | | |
 | |_) | | / /_|  _| | |___| |   | |___| |__| |_| | |_| | |_| |
 |____/___/____|_|   |_____|_|    \____|_____\___/ \___/|____/ 

 external-dns-bizflycloud-webhook
 version: %s (%s)

`

var (
	Version = "local"
	Gitsha  = "?"
)

func main() {
	fmt.Printf(banner, Version, Gitsha)
	logging.Init()
	config := configuration.Init()
	provider, err := dnsprovider.Init(config)
	if err != nil {
		log.Fatalf("Failed to initialize DNS provider: %v", err)
	}
	srv := server.Init(config, webhook.New(provider))
	server.ShutdownGracefully(srv)
}
