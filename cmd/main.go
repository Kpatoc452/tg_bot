package main

import (
	"database/sql"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	_ "github.com/lib/pq"
)

const (
	dbHost     = "localhost"
	dbPort     = 5432
	dbUser     = "youruser"
	dbPassword = "yourpassword"
	dbName     = "yourdb"
)

func main() {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", dbHost, dbPort, dbUser, dbPassword, dbName)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	bot, err := tgbotapi.NewBotAPI("TG_BOT_TOKEN")
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true
	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		userID := update.Message.From.ID

		if !isUserExists(db, userID) {
			createUser(db, userID)
		}

		message := update.Message.Text

		cleanMessage := strings.ReplaceAll(strings.TrimSpace(message), ",", ".")

		if isValidNumber(cleanMessage) {
			amount, err := strconv.ParseFloat(cleanMessage, 64)
			if err == nil {
				if updateBalance(db, userID, amount) {
					balance := getUserBalance(db, userID)
					reply := fmt.Sprintf("Ваш текущий баланс: $%.2f", balance)
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, reply)
					bot.Send(msg)
				} else {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Недостаточно средств для списания.")
					bot.Send(msg)
				}
			}
		} else {
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Пожалуйста, введите число.")
			bot.Send(msg)
		}
	}
}

func isUserExists(db *sql.DB, userID int64) bool {
	var exists bool
	query := "SELECT EXISTS (SELECT 1 FROM users WHERE user_id=$1)"
	err := db.QueryRow(query, userID).Scan(&exists)
	if err != nil && err != sql.ErrNoRows {
		log.Fatal(err)
	}
	return exists
}

func createUser(db *sql.DB, userID int64) {
	query := "INSERT INTO users (user_id, balance) VALUES ($1, $2)"
	_, err := db.Exec(query, userID, 0.00)
	if err != nil {
		log.Fatal(err)
	}
}

func updateBalance(db *sql.DB, userID int64, amount float64) bool {
	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}
	defer tx.Rollback()

	var currentBalance float64
	err = tx.QueryRow("SELECT balance FROM users WHERE user_id = $1 FOR UPDATE", userID).Scan(&currentBalance)
	if err != nil {
		log.Fatal(err)
	}

	newBalance := currentBalance + amount
	if newBalance < 0 {
		return false
	}

	_, err = tx.Exec("UPDATE users SET balance = $1 WHERE user_id = $2", newBalance, userID)
	if err != nil {
		log.Fatal(err)
	}

	err = tx.Commit()
	if err != nil {
		log.Fatal(err)
	}
	return true
}

func getUserBalance(db *sql.DB, userID int64) float64 {
	var balance float64
	query := "SELECT balance FROM users WHERE user_id = $1"
	err := db.QueryRow(query, userID).Scan(&balance)
	if err != nil && err != sql.ErrNoRows {
		log.Fatal(err)
	}
	return balance
}

func isValidNumber(s string) bool {
	re := regexp.MustCompile(`^-?\d+(\.\d+)?$`)
	return re.MatchString(s)
}
