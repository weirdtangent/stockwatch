package main

import (
	"crypto/aes"
	"crypto/cipher"
	crand "crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"math/rand"

	"github.com/aws/aws-sdk-go/aws/session"

	"github.com/weirdtangent/myaws"
)

// random string of bytes, use in nonce values, for example
//   https://stackoverflow.com/questions/22892120/how-to-generate-a-random-string-of-a-fixed-length-in-go
const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

func RandStringMask(n int) string {
	b := make([]byte, n)
	for i := 0; i < n; {
		if idx := int(rand.Int63() & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i++
		}
	}
	return string(b)
}

func encryptURL(awssess *session.Session, text []byte) ([]byte, error) {
	secret, err := myaws.AWSGetSecretKV(awssess, "stockwatch_misc", "stockwatch_next_url_key")
	if err != nil {
		return nil, err
	}
	key := []byte(*secret)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	b := base64.StdEncoding.EncodeToString(text)
	ciphertext := make([]byte, aes.BlockSize+len(b))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(crand.Reader, iv); err != nil {
		return nil, err
	}
	cfb := cipher.NewCFBEncrypter(block, iv)
	cfb.XORKeyStream(ciphertext[aes.BlockSize:], []byte(b))
	cipherstring := ([]byte(base64.URLEncoding.EncodeToString(ciphertext)))
	return cipherstring, nil
}

func decryptURL(awssess *session.Session, cipherstring []byte) ([]byte, error) {
	secret, err := myaws.AWSGetSecretKV(awssess, "stockwatch_misc", "stockwatch_next_url_key")
	if err != nil {
		return nil, err
	}
	key := []byte(*secret)
	textstr, err := base64.URLEncoding.DecodeString(string(cipherstring))
	if err != nil {
		return nil, err
	}
	text := ([]byte(textstr))
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(text) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}
	iv := text[:aes.BlockSize]
	text = text[aes.BlockSize:]
	cfb := cipher.NewCFBDecrypter(block, iv)
	cfb.XORKeyStream(text, text)
	data, err := base64.StdEncoding.DecodeString(string(text))
	if err != nil {
		return nil, err
	}
	return data, nil
}
