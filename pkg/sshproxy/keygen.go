package sshproxy

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
)

// generateHostKey generates a new ED25519 host key and saves it to the given path
func generateHostKey(path string) (ssh.Signer, error) {
	// Generate ED25519 key pair
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	// Encode private key to PEM format
	// Note: OpenSSH uses a custom format for ED25519 keys
	pemBlock := &pem.Block{
		Type:  "OPENSSH PRIVATE KEY",
		Bytes: marshalED25519PrivateKey(privateKey),
	}

	pemBytes := pem.EncodeToMemory(pemBlock)

	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// Write private key to file
	if err := os.WriteFile(path, pemBytes, 0600); err != nil {
		return nil, fmt.Errorf("failed to write key file: %w", err)
	}

	// Parse the key we just generated
	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create signer: %w", err)
	}

	return signer, nil
}

// marshalED25519PrivateKey marshals an ED25519 private key in OpenSSH format
func marshalED25519PrivateKey(key ed25519.PrivateKey) []byte {
	// OpenSSH private key format (simplified)
	// This is a simplified version - for production use a proper library

	pubKey := key.Public().(ed25519.PublicKey)

	// Build the key blob
	// Format: openssh-key-v1 format
	magic := []byte("openssh-key-v1\x00")

	// Cipher and KDF (none for unencrypted)
	cipher := "none"
	kdf := "none"
	kdfOptions := []byte{}
	numKeys := uint32(1)

	// Public key section
	pubKeyType := "ssh-ed25519"

	// Build public key blob
	pubKeyBlob := marshalString(pubKeyType)
	pubKeyBlob = append(pubKeyBlob, marshalBytes(pubKey)...)

	// Private key section (with padding)
	// Random check integers (same value for verification)
	checkInt := make([]byte, 4)
	rand.Read(checkInt)

	privSection := append(checkInt, checkInt...)
	privSection = append(privSection, marshalString(pubKeyType)...)
	privSection = append(privSection, marshalBytes(pubKey)...)
	privSection = append(privSection, marshalBytes(key)...) // Full 64-byte private key
	privSection = append(privSection, marshalString("")...) // Comment

	// Add padding
	for i := 1; len(privSection)%8 != 0; i++ {
		privSection = append(privSection, byte(i))
	}

	// Build final blob
	blob := magic
	blob = append(blob, marshalString(cipher)...)
	blob = append(blob, marshalString(kdf)...)
	blob = append(blob, marshalBytes(kdfOptions)...)
	blob = append(blob, marshalUint32(numKeys)...)
	blob = append(blob, marshalBytes(pubKeyBlob)...)
	blob = append(blob, marshalBytes(privSection)...)

	return blob
}

func marshalString(s string) []byte {
	return marshalBytes([]byte(s))
}

func marshalBytes(b []byte) []byte {
	result := make([]byte, 4+len(b))
	result[0] = byte(len(b) >> 24)
	result[1] = byte(len(b) >> 16)
	result[2] = byte(len(b) >> 8)
	result[3] = byte(len(b))
	copy(result[4:], b)
	return result
}

func marshalUint32(n uint32) []byte {
	return []byte{
		byte(n >> 24),
		byte(n >> 16),
		byte(n >> 8),
		byte(n),
	}
}
