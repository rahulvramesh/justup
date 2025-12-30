package sshproxy

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"

	"github.com/rahulvramesh/justup/pkg/database"
	"github.com/rahulvramesh/justup/pkg/kubernetes"
	"golang.org/x/crypto/ssh"
)

// Config holds the SSH proxy server configuration
type Config struct {
	ListenAddr   string
	HostKeyPath  string
	DatabasePath string
}

// Server is the SSH proxy server
type Server struct {
	config     *Config
	sshConfig  *ssh.ServerConfig
	db         *database.DB
	k8sClient  *kubernetes.Client
	hostSigner ssh.Signer
}

// NewServer creates a new SSH proxy server
func NewServer(config *Config) (*Server, error) {
	// Load or generate host key
	hostKey, err := loadOrGenerateHostKey(config.HostKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load host key: %w", err)
	}

	// Open database
	db, err := database.Open(config.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Create Kubernetes client
	k8sClient, err := kubernetes.NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	server := &Server{
		config:     config,
		db:         db,
		k8sClient:  k8sClient,
		hostSigner: hostKey,
	}

	// Configure SSH server
	server.sshConfig = &ssh.ServerConfig{
		PublicKeyCallback: server.publicKeyCallback,
		ServerVersion:     "SSH-2.0-JustupProxy",
	}
	server.sshConfig.AddHostKey(hostKey)

	return server, nil
}

// ListenAndServe starts the SSH proxy server
func (s *Server) ListenAndServe(ctx context.Context) error {
	listener, err := net.Listen("tcp", s.config.ListenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	defer listener.Close()

	// Close listener on context cancellation
	go func() {
		<-ctx.Done()
		listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				log.Printf("Failed to accept connection: %v", err)
				continue
			}
		}

		go s.handleConnection(ctx, conn)
	}
}

// handleConnection handles a single SSH connection
func (s *Server) handleConnection(ctx context.Context, netConn net.Conn) {
	defer netConn.Close()

	// Perform SSH handshake
	sshConn, chans, reqs, err := ssh.NewServerConn(netConn, s.sshConfig)
	if err != nil {
		log.Printf("SSH handshake failed: %v", err)
		return
	}
	defer sshConn.Close()

	// Extract workspace name from username
	workspaceName := sshConn.User()
	log.Printf("Connection from %s for workspace '%s'", sshConn.RemoteAddr(), workspaceName)

	// Get workspace pod IP
	ws, err := s.k8sClient.GetWorkspace(ctx, workspaceName)
	if err != nil {
		log.Printf("Failed to get workspace '%s': %v", workspaceName, err)
		return
	}

	if ws.Status != "Running" {
		log.Printf("Workspace '%s' is not running (status: %s)", workspaceName, ws.Status)
		return
	}

	if ws.PodIP == "" {
		log.Printf("Workspace '%s' has no IP address", workspaceName)
		return
	}

	// Connect to the workspace pod
	targetAddr := fmt.Sprintf("%s:22", ws.PodIP)
	targetConn, err := net.Dial("tcp", targetAddr)
	if err != nil {
		log.Printf("Failed to connect to workspace '%s' at %s: %v", workspaceName, targetAddr, err)
		return
	}
	defer targetConn.Close()

	// Establish SSH connection to target
	targetSSHConn, targetChans, targetReqs, err := ssh.NewClientConn(targetConn, targetAddr, &ssh.ClientConfig{
		User: "dev",
		Auth: []ssh.AuthMethod{
			// Use the same key that was used to authenticate
			ssh.PublicKeys(s.hostSigner),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	})
	if err != nil {
		log.Printf("Failed to establish SSH connection to workspace '%s': %v", workspaceName, err)
		return
	}
	defer targetSSHConn.Close()

	// Proxy the connection
	var wg sync.WaitGroup
	wg.Add(2)

	// Proxy global requests
	go func() {
		defer wg.Done()
		proxyRequests(reqs, targetSSHConn)
	}()

	go func() {
		defer wg.Done()
		proxyRequests(targetReqs, sshConn)
	}()

	// Proxy channels
	go proxyChannels(chans, targetSSHConn)
	go proxyChannels(targetChans, sshConn)

	// Wait for connection to close
	wg.Wait()
	log.Printf("Connection closed for workspace '%s'", workspaceName)
}

// publicKeyCallback validates a public key for authentication
func (s *Server) publicKeyCallback(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
	fingerprint := ssh.FingerprintSHA256(key)
	workspaceName := conn.User()

	log.Printf("Auth attempt for workspace '%s' with key %s", workspaceName, fingerprint)

	// Look up the key in the database
	sshKey, err := s.db.GetSSHKeyByFingerprint(fingerprint)
	if err != nil {
		log.Printf("Key not found: %s", fingerprint)
		return nil, fmt.Errorf("unknown public key")
	}

	// Update last used timestamp
	s.db.UpdateSSHKeyLastUsed(sshKey.ID)

	log.Printf("Auth successful for workspace '%s' (user: %s, key: %s)", workspaceName, sshKey.UserID, sshKey.Name)

	return &ssh.Permissions{
		Extensions: map[string]string{
			"user-id":     sshKey.UserID,
			"key-id":      sshKey.ID,
			"fingerprint": fingerprint,
		},
	}, nil
}

// proxyRequests proxies SSH global requests
func proxyRequests(reqs <-chan *ssh.Request, conn ssh.Conn) {
	for req := range reqs {
		if req == nil {
			return
		}
		ok, payload, err := conn.SendRequest(req.Type, req.WantReply, req.Payload)
		if err != nil {
			return
		}
		if req.WantReply {
			req.Reply(ok, payload)
		}
	}
}

// proxyChannels proxies SSH channels
func proxyChannels(chans <-chan ssh.NewChannel, conn ssh.Conn) {
	for newChan := range chans {
		if newChan == nil {
			return
		}

		// Open a channel on the target
		targetChan, targetReqs, err := conn.OpenChannel(newChan.ChannelType(), newChan.ExtraData())
		if err != nil {
			newChan.Reject(ssh.ConnectionFailed, err.Error())
			continue
		}

		// Accept the channel from the client
		clientChan, clientReqs, err := newChan.Accept()
		if err != nil {
			targetChan.Close()
			continue
		}

		// Proxy channel data bidirectionally
		go func() {
			defer clientChan.Close()
			defer targetChan.Close()

			var wg sync.WaitGroup
			wg.Add(4)

			// Proxy data
			go func() {
				defer wg.Done()
				io.Copy(targetChan, clientChan)
				targetChan.CloseWrite()
			}()
			go func() {
				defer wg.Done()
				io.Copy(clientChan, targetChan)
				clientChan.CloseWrite()
			}()

			// Proxy channel requests
			go func() {
				defer wg.Done()
				proxyChannelRequests(clientReqs, targetChan)
			}()
			go func() {
				defer wg.Done()
				proxyChannelRequests(targetReqs, clientChan)
			}()

			wg.Wait()
		}()
	}
}

// proxyChannelRequests proxies SSH channel requests
func proxyChannelRequests(reqs <-chan *ssh.Request, ch ssh.Channel) {
	for req := range reqs {
		if req == nil {
			return
		}
		ok, err := ch.SendRequest(req.Type, req.WantReply, req.Payload)
		if err != nil {
			return
		}
		if req.WantReply {
			req.Reply(ok, nil)
		}
	}
}

// loadOrGenerateHostKey loads an existing host key or generates a new one
func loadOrGenerateHostKey(path string) (ssh.Signer, error) {
	// Try to load existing key
	keyBytes, err := os.ReadFile(path)
	if err == nil {
		signer, err := ssh.ParsePrivateKey(keyBytes)
		if err == nil {
			return signer, nil
		}
	}

	// Generate new key if loading failed
	log.Printf("Generating new host key at %s", path)
	return generateHostKey(path)
}
