package auth

import (
	"testing"
	"time"

	"github.com/fsamin/phoebus/internal/model"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestHashPasswordAndCheck(t *testing.T) {
	hash, err := HashPassword("mypassword")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !CheckPassword(hash, "mypassword") {
		t.Fatal("CheckPassword should return true for correct password")
	}
}

func TestCheckPasswordWrong(t *testing.T) {
	hash, _ := HashPassword("correct")
	if CheckPassword(hash, "wrong") {
		t.Fatal("CheckPassword should return false for wrong password")
	}
}

func TestGenerateAndValidateToken(t *testing.T) {
	user := &model.User{
		ID:       uuid.New(),
		Username: "testuser",
		Role:     model.RoleAdmin,
	}
	secret := "test-secret-key-for-jwt"

	tokenStr, err := GenerateToken(user, secret)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	claims, err := ValidateToken(tokenStr, secret)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}

	if claims.UserID != user.ID.String() {
		t.Errorf("UserID = %q, want %q", claims.UserID, user.ID.String())
	}
	if claims.Username != "testuser" {
		t.Errorf("Username = %q, want %q", claims.Username, "testuser")
	}
	if claims.Role != model.RoleAdmin {
		t.Errorf("Role = %q, want %q", claims.Role, model.RoleAdmin)
	}
}

func TestValidateTokenExpired(t *testing.T) {
	user := &model.User{
		ID:       uuid.New(),
		Username: "expired",
		Role:     model.RoleLearner,
	}
	secret := "test-secret"

	// Create a token that expired 1 hour ago
	claims := &Claims{
		UserID:   user.ID.String(),
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			Subject:   user.ID.String(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString([]byte(secret))

	_, err := ValidateToken(tokenStr, secret)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestValidateTokenWrongSecret(t *testing.T) {
	user := &model.User{
		ID:       uuid.New(),
		Username: "user",
		Role:     model.RoleLearner,
	}

	tokenStr, _ := GenerateToken(user, "secret-a")
	_, err := ValidateToken(tokenStr, "secret-b")
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestValidateTokenMalformed(t *testing.T) {
	_, err := ValidateToken("not.a.valid.token", "secret")
	if err == nil {
		t.Fatal("expected error for malformed token")
	}
}
