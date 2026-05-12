package jwt_test

import (
	"testing"
	"time"

	"microservie/user-auth/internal/jwt"
)

func TestIssueAndVerify_roundTrip(t *testing.T) {
	mgr := jwt.New([]byte("test-secret-32-bytes-long-padding"), time.Hour)

	token, err := mgr.Issue("user-1")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	uid, err := mgr.Verify(token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if uid != "user-1" {
		t.Fatalf("want user-1, got %s", uid)
	}
}

func TestVerify_rejectsTamperedToken(t *testing.T) {
	mgr := jwt.New([]byte("test-secret-32-bytes-long-padding"), time.Hour)
	token, _ := mgr.Issue("user-1")
	_, err := mgr.Verify(token + "x")
	if err == nil {
		t.Fatal("want error on tampered token")
	}
}

func TestVerify_rejectsExpiredToken(t *testing.T) {
	mgr := jwt.New([]byte("test-secret-32-bytes-long-padding"), -1*time.Second)
	token, _ := mgr.Issue("user-1")
	_, err := mgr.Verify(token)
	if err == nil {
		t.Fatal("want error on expired token")
	}
}
