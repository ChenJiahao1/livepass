package main

import (
	"flag"
	"fmt"

	"livepass/services/program-rpc/internal/config"
	"livepass/services/program-rpc/internal/svc"
	programmcp "livepass/services/program-rpc/mcp"
)

var configFile = flag.String("f", "services/program-rpc/etc/program-mcp.yaml", "the config file")

func main() {
	flag.Parse()

	c, err := config.LoadMCP(*configFile)
	if err != nil {
		panic(err)
	}

	server := programmcp.NewServer(c.McpConf, svc.NewMCPServiceContext(c))
	defer server.Stop()

	fmt.Printf("Starting program MCP server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
