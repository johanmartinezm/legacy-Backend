package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
)

func TestExhaustiveUserUpdate(t *testing.T) {
	dbURL := "postgres://postgres:postgres@localhost:5432/applegacy?sslmode=disable"
	pool, err := pgxpool.Connect(context.Background(), dbURL)
	if err != nil {
		t.Fatalf("Unable to connect to database: %v", err)
	}
	defer pool.Close()

	repo := NewUserRepository(pool)
	ctx := context.Background()

	// 1. Get a user
	users, err := repo.FindAll(ctx)
	if err != nil || len(users) == 0 {
		t.Fatalf("No users found or error: %v", err)
	}
	user := users[0]

	// Create a unique salt for strings
	salt := fmt.Sprintf("%d", time.Now().UnixNano())

	// 2. Modify ALL fields
	user.FirstName = "TestFirst_" + salt
	user.LastName = "TestLast_" + salt
	user.Industry = "Servicios"
	user.Phone = "12345678"
	user.Country = "Colombia"
	user.IdentificationType = "CC"
	user.IdentificationNumber = "ID_" + salt
	user.CustomerStatus = "Ya soy cliente"
	user.Generation = "Segunda"
	user.IsPublicProfile = !user.IsPublicProfile
	user.AllowMessagesFromStrangers = !user.AllowMessagesFromStrangers
	user.ShowActivity = !user.ShowActivity

	err = repo.Update(ctx, user)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// 3. Verify
	updatedUser, err := repo.FindByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}

	if updatedUser.FirstName != user.FirstName {
		t.Errorf("FirstName mismatch")
	}
	if updatedUser.Industry != user.Industry {
		t.Errorf("Industry mismatch: got %s, want %s", updatedUser.Industry, user.Industry)
	}
	if updatedUser.Country != user.Country {
		t.Errorf("Country mismatch: got %s, want %s", updatedUser.Country, user.Country)
	}
	if updatedUser.IdentificationType != user.IdentificationType {
		t.Errorf("IdentificationType mismatch: got %s, want %s", updatedUser.IdentificationType, user.IdentificationType)
	}
	if updatedUser.IdentificationNumber != user.IdentificationNumber {
		t.Errorf("IdentificationNumber mismatch: got %s, want %s", updatedUser.IdentificationNumber, user.IdentificationNumber)
	}
	if updatedUser.CustomerStatus != user.CustomerStatus {
		t.Errorf("CustomerStatus mismatch: got %s, want %s", updatedUser.CustomerStatus, user.CustomerStatus)
	}
	if updatedUser.Generation != user.Generation {
		t.Errorf("Generation mismatch")
	}
	if updatedUser.IsPublicProfile != user.IsPublicProfile {
		t.Errorf("IsPublicProfile mismatch")
	}

	fmt.Printf("✓ Exhaustive update test passed for user %s\n", user.ID)
	fmt.Printf("  Country: %s, ID Type: %s, Industry: %s, Customer Status: %s\n",
		updatedUser.Country, updatedUser.IdentificationType, updatedUser.Industry, updatedUser.CustomerStatus)
}
