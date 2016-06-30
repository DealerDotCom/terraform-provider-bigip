package util

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"io"

	"github.com/grafana/grafana/pkg/log"
)

const saltLength = 8

func Decrypt(payload []byte, secret string) []byte {
	salt := payload[:saltLength]
	key := encryptionKeyToBytes(secret, string(salt))

	block, err := aes.NewCipher(key)
	if err != nil {
		log.Fatal(4, err.Error())
	}

	// The IV needs to be unique, but not secure. Therefore it's common to
	// include it at the beginning of the ciphertext.
	if len(payload) < aes.BlockSize {
		log.Fatal(4, "payload too short")
	}
	iv := payload[saltLength : saltLength+aes.BlockSize]
	payload = payload[saltLength+aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)

	// XORKeyStream can work in-place if the two arguments are the same.
	stream.XORKeyStream(payload, payload)
	return payload
}

func Encrypt(payload []byte, secret string) []byte {
	salt := GetRandomString(saltLength)

	key := encryptionKeyToBytes(secret, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		log.Fatal(4, err.Error())
	}

	// The IV needs to be unique, but not secure. Therefore it's common to
	// include it at the beginning of the ciphertext.
	ciphertext := make([]byte, saltLength+aes.BlockSize+len(payload))
	copy(ciphertext[:saltLength], []byte(salt))
	iv := ciphertext[saltLength : saltLength+aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		log.Fatal(4, err.Error())
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[saltLength+aes.BlockSize:], payload)

	return ciphertext
}

// Key needs to be 32bytes
func encryptionKeyToBytes(secret, salt string) []byte {
	return PBKDF2([]byte(secret), []byte(salt), 10000, 32, sha256.New)
}
