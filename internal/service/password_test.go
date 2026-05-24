package service

import "testing"

func TestPasswordHashVerify(t *testing.T) {
	hash, err := HashPassword("admin12345")
	if err != nil {
		t.Fatalf("hash password failed: %v", err)
	}

	if !VerifyPassword("admin12345", hash) {
		t.Fatal("expected password to verify")
	}
	if VerifyPassword("wrong-password", hash) {
		t.Fatal("expected wrong password to fail")
	}
}
