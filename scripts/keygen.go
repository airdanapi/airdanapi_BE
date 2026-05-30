package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func main() {
	// 1. Generate RSA Key Pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}

	pubASN1, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		panic(err)
	}

	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubASN1,
	})

	// Format pubPEM to single line for .env
	pubPEMStr := strings.ReplaceAll(string(pubPEM), "\n", "\\n")
	fmt.Println("PUBLIC KEY FOR .ENV (JWT_PUBLIC_KEY_PEM):")
	fmt.Println(pubPEMStr)
	fmt.Println()

	// 2. Generate a demo JWT Token
	claims := jwt.MapClaims{
		"iss":    "smartbank",
		"aud":    []string{"ecosystem"},
		"sub":    "admin",
		"roles":  []string{"operator"},
		"scopes": []string{"marketplace:read", "marketplace:write", "admin:read", "admin:write"},
		"jti":    "demo-token-live",
		"iat":    time.Now().Unix(),
		"nbf":    time.Now().Unix(),
		"exp":    time.Now().Add(24 * 365 * time.Hour).Unix(), // 1 year
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		panic(err)
	}

	fmt.Println("DEMO TOKEN (Replace $TOKEN in DEMO.md & demo_traffic.ps1):")
	fmt.Println(tokenString)
}
