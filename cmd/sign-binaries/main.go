// Command sign-binaries signs release binaries with the Ed25519 private key
// stored in the MINISIGN_PRIVATE_KEY GitHub secret, used in CI to produce
// the "<file>.minisig" sidecars that "jcore update self" verifies before
// installing an update.
//
// Usage: MINISIGN_PRIVATE_KEY=<base64_priv_key> go run ./cmd/sign-binaries <file1> <file2> ...
package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"os"
)

func main() {
	privKeyBase64 := os.Getenv("MINISIGN_PRIVATE_KEY")
	if privKeyBase64 == "" {
		log.Fatal("MINISIGN_PRIVATE_KEY environment variable is not set")
	}

	privKeyBytes, err := base64.StdEncoding.DecodeString(privKeyBase64)
	if err != nil {
		log.Fatalf("failed to decode private key: %v", err)
	}

	if len(privKeyBytes) != ed25519.PrivateKeySize {
		log.Fatalf("invalid private key size: expected %d, got %d", ed25519.PrivateKeySize, len(privKeyBytes))
	}

	privKey := ed25519.PrivateKey(privKeyBytes)

	if len(os.Args) < 2 {
		log.Fatal("usage: go run ./cmd/sign-binaries <file1> <file2> ...")
	}

	for _, path := range os.Args[1:] {
		if err := signFile(path, privKey); err != nil {
			log.Fatalf("failed to sign %s: %v", path, err)
		}
		fmt.Printf("Successfully signed %s -> %s.minisig\n", path, path)
	}
}

func signFile(path string, privKey ed25519.PrivateKey) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	signature := ed25519.Sign(privKey, content)
	sigBase64 := base64.StdEncoding.EncodeToString(signature)

	sigPath := path + ".minisig"
	return os.WriteFile(sigPath, []byte(sigBase64), 0644)
}
