package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/dotenv/cli/internal/crypto"
)

func main() {
	var (
		mode    = flag.String("mode", "test", "Mode: test, encrypt, decrypt")
		input   = flag.String("input", "", "Input text for encrypt/decrypt")
		keyStr  = flag.String("key", "", "Base64 encoded key (uses test key if empty)")
		verbose = flag.Bool("v", false, "Verbose output")
	)

	flag.Parse()

	// Initialize crypto
	crypto.Initialize()

	switch *mode {
	case "test":
		runTests(*verbose)
	case "encrypt":
		if *input == "" {
			log.Fatal("Input required for encryption")
		}
		encrypt(*input, *keyStr)
	case "decrypt":
		if *input == "" {
			log.Fatal("Input required for decryption")
		}
		decrypt(*input, *keyStr)
	default:
		log.Fatalf("Unknown mode: %s", *mode)
	}
}

func runTests(verbose bool) {
	fmt.Println("Go Encryption Test Results")
	fmt.Println("==========================")

	// Test key (non-zero for validation)
	testKey := make([]byte, 32)
	for i := range testKey {
		testKey[i] = byte(i + 1)
	}

	testCases := []string{
		"Hello, World!",
		"",
		"Unicode: 你好世界 🌍",
		"Special chars: !@#$%^&*()",
	}

	for _, plaintext := range testCases {
		fmt.Printf("Plaintext: %q\n", plaintext)

		// Encrypt
		ciphertext, err := crypto.EncryptString(plaintext, testKey)
		if err != nil {
			fmt.Printf("Encryption error: %v\n\n", err)
			continue
		}
		fmt.Printf("Ciphertext: %s\n", ciphertext)

		// Decrypt
		decrypted, err := crypto.DecryptString(ciphertext, testKey)
		if err != nil {
			fmt.Printf("Decryption error: %v\n\n", err)
			continue
		}
		fmt.Printf("Decrypted: %q\n", decrypted)

		match := decrypted == plaintext
		if match {
			fmt.Println("Match: ✓")
		} else {
			fmt.Println("Match: ✗")
		}
		fmt.Println()
	}

	// Generate fixed IV test vectors for cross-platform testing
	fmt.Println("\nFixed IV Test Vectors")
	fmt.Println("====================")

	encryptor := crypto.NewGCMEncryptor()
	fixedIV := make([]byte, 12) // 12 zeros
	fixedKey := make([]byte, 32)
	for i := range fixedKey {
		fixedKey[i] = byte(i + 1) // Non-zero for validation
	}

	for _, plaintext := range testCases {
		ciphertext, err := encryptor.EncryptWithIV([]byte(plaintext), fixedKey, fixedIV)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		fmt.Printf("Plaintext: %q\n", plaintext)
		fmt.Printf("Key: %s\n", base64.StdEncoding.EncodeToString(fixedKey))
		fmt.Printf("IV: %s\n", base64.StdEncoding.EncodeToString(fixedIV))
		fmt.Printf("Ciphertext: %s\n", ciphertext)

		if verbose {
			// Decode and show structure
			decoded, _ := base64.StdEncoding.DecodeString(ciphertext)
			fmt.Printf("  Structure: IV[%d] + Ciphertext+Tag[%d]\n", 12, len(decoded)-12)
		}
		fmt.Println()
	}
}

func encrypt(plaintext, keyStr string) {
	key := getKey(keyStr)

	ciphertext, err := crypto.EncryptString(plaintext, key)
	if err != nil {
		log.Fatalf("Encryption failed: %v", err)
	}

	fmt.Println(ciphertext)
}

func decrypt(ciphertext, keyStr string) {
	key := getKey(keyStr)

	plaintext, err := crypto.DecryptString(ciphertext, key)
	if err != nil {
		log.Fatalf("Decryption failed: %v", err)
	}

	fmt.Println(plaintext)
}

func getKey(keyStr string) []byte {
	if keyStr == "" {
		// Use test key (non-zero for validation)
		key := make([]byte, 32)
		for i := range key {
			key[i] = byte(i + 1)
		}
		return key
	}

	key, err := base64.StdEncoding.DecodeString(keyStr)
	if err != nil {
		log.Fatalf("Invalid key encoding: %v", err)
	}

	if len(key) != 32 {
		log.Fatalf("Invalid key size: expected 32 bytes, got %d", len(key))
	}

	return key
}

// Helper to test compatibility with PHP/JS output
func init() {
	// Check if we're running with test vectors from PHP/JS
	if len(os.Args) > 1 && os.Args[1] == "compat" {
		testCompatibility()
		os.Exit(0)
	}
}

func testCompatibility() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: test-go compat <ciphertext>")
		os.Exit(1)
	}

	ciphertext := os.Args[2]
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1) // Non-zero test key
	}

	plaintext, err := crypto.DecryptString(ciphertext, key)
	if err != nil {
		fmt.Printf("Decryption failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Decrypted: %q\n", plaintext)
}
