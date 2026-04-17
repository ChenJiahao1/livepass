package main

import (
	"flag"
	"fmt"

	"livepass/services/order-rpc/internal/config"
	"livepass/services/order-rpc/internal/svc"
	ordermcp "livepass/services/order-rpc/mcp"
)

var configFile = flag.String("f", "services/order-rpc/etc/order-mcp.yaml", "the config file")

func main() {
	flag.Parse()

	c, err := config.LoadMCP(*configFile)
	if err != nil {
		panic(err)
	}
	server := ordermcp.NewServer(c.McpConf, svc.NewMCPServiceContext(c))
	defer server.Stop()

	fmt.Printf("Starting order MCP server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
