package zbrowser

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"fmt"
)

var cryptKey = []byte("2SBW3%DCEs#af!NM")

var cryptIV = []byte("7%3s5$2K74df2N32")

// Encrypt encrypts plaintext using AES-128-CBC with PKCS7 padding,
// and returns a URL-safe base64 encoded string (padding stripped).
func Encrypt(plaintext []byte) (string, error) {
	block, err := aes.NewCipher(cryptKey)
	if err != nil {
		return "", fmt.Errorf("[ZBrowser] new cipher: %w", err)
	}

	// PKCS7 padding
	padLen := aes.BlockSize - len(plaintext)%aes.BlockSize
	padded := make([]byte, len(plaintext)+padLen)
	copy(padded, plaintext)
	for i := len(plaintext); i < len(padded); i++ {
		padded[i] = byte(padLen)
	}

	ciphertext := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, cryptIV).CryptBlocks(ciphertext, padded)

	return base64.URLEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a URL-safe base64 encoded (optionally padded) ciphertext
// using AES-128-CBC and removes PKCS7 padding.
func Decrypt(encoded string) ([]byte, error) {
	// Restore standard base64 padding if stripped
	if mod := len(encoded) % 4; mod != 0 {
		encoded += string([]byte("====")[:4-mod])
	}

	ciphertext, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("[ZBrowser] base64 decode: %w", err)
	}

	if len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("[ZBrowser] invalid ciphertext length: %d", len(ciphertext))
	}

	block, err := aes.NewCipher(cryptKey)
	if err != nil {
		return nil, fmt.Errorf("[ZBrowser] new cipher: %w", err)
	}

	plaintext := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, cryptIV).CryptBlocks(plaintext, ciphertext)

	// Remove PKCS7 padding
	padLen := int(plaintext[len(plaintext)-1])
	if padLen == 0 || padLen > aes.BlockSize {
		return nil, fmt.Errorf("[ZBrowser] invalid PKCS7 padding: %d", padLen)
	}
	plaintext = plaintext[:len(plaintext)-padLen]

	return plaintext, nil
}
