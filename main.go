package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	config, err := newConfig(os.Getenv("HR_DATABASE"), os.Getenv("HR_LOGFILE"))
	if err != nil {
		log.Printf("invalid hranoprov configuration: %v", err)
		os.Exit(1)
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "hranoprov-mcp", Version: "0.1.0"}, nil)
	registerTools(server, config)
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Printf("server failed: %v", err)
		os.Exit(1)
	}
}

type config struct {
	databasePath string
	logfilePath  string
}

func newConfig(databasePath, logfilePath string) (*config, error) {
	if databasePath == "" {
		return nil, fmt.Errorf("HR_DATABASE is required")
	}
	return &config{databasePath: databasePath, logfilePath: logfilePath}, nil
}
