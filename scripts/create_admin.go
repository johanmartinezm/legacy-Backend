package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v4/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	dsn := "postgres://dba:123@localhost:5432/applegacy?sslmode=disable"
	dbPool, err := pgxpool.Connect(context.Background(), dsn)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	defer dbPool.Close()

	email := "smatiz@hotmail.com"
	password := "123456"

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal(err)
	}

	query := `
		INSERT INTO core.admin_users (email, password_hash, first_name, last_name, role)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (email) DO UPDATE 
		SET password_hash = EXCLUDED.password_hash;
	`

	_, err = dbPool.Exec(context.Background(), query, email, string(hashedPassword), "S", "Matiz", "admin")
	if err != nil {
		log.Fatalf("Failed to insert/update admin: %v\n", err)
	}

	fmt.Printf("Admin user %s has been created/updated with password %s\n", email, password)
}
