package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/rahulvramesh/justup/pkg/sshproxy"
)

func main() {
	// Parse flags
	addr := flag.String("addr", ":2222", "SSH listen address")
	hostKeyPath := flag.String("host-key", "/etc/justup/ssh_host_ed25519_key", "Path to SSH host key")
	dbPath := flag.String("db", "/var/lib/justup/justup.db", "Path to SQLite database")
	flag.Parse()

	// Create server config
	config := &sshproxy.Config{
		ListenAddr:  *addr,
		HostKeyPath: *hostKeyPath,
		DatabasePath: *dbPath,
	}

	// Create and start server
	server, err := sshproxy.NewServer(config)
	if err != nil {
		log.Fatalf("Failed to create SSH proxy server: %v", err)
	}

	// Handle shutdown gracefully
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Printf("Received signal %v, shutting down...", sig)
		cancel()
	}()

	log.Printf("Starting SSH proxy on %s", *addr)
	if err := server.ListenAndServe(ctx); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
