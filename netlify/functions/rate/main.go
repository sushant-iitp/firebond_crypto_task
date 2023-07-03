package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	_ "github.com/go-sql-driver/mysql"
)

type ExchangeRate struct {
	Timestamp time.Time `json:"timestamp"`
	Rate      float64   `json:"rate"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type Database struct {
	DB *sql.DB
}

func NewDatabase() (*Database, error) {
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbHost := os.Getenv("DB_HOST")
	dbDatabase := os.Getenv("DB_DATABASE")

	db, err := sql.Open("mysql", dbUser+":"+dbPassword+"@tcp("+dbHost+")/"+dbDatabase)
	if err != nil {
		return nil, err
	}

	return &Database{DB: db}, nil
}

func (d *Database) GetExchangeRate(crypto, fiat string) ([]ExchangeRate, error) {
	query := "SELECT timestamp, rate FROM exchange_rates WHERE crypto = ? AND fiat = ?"

	rows, err := d.DB.Query(query, crypto, fiat)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rates := make([]ExchangeRate, 0)
	for rows.Next() {
		var timestamp time.Time
		var rate float64
		if err := rows.Scan(&timestamp, &rate); err != nil {
			return nil, err
		}

		rates = append(rates, ExchangeRate{Timestamp: timestamp, Rate: rate})
	}

	return rates, nil
}

func HandleRequest(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	splitPath := strings.Split(request.Path, "/")

	if len(splitPath) == 4 && splitPath[3] == "history" {
		// GET /rate/{crypto}/{fiat}/history
		crypto := splitPath[2]
		fiat := splitPath[3]

		db, err := NewDatabase()
		if err != nil {
			log.Println("Error connecting to the database:", err)
			return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, err
		}
		defer db.DB.Close()

		rates, err := db.GetExchangeRate(crypto, fiat)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				errorResponse := ErrorResponse{Error: "Exchange rate history not found"}
				responseBody, _ := json.Marshal(errorResponse)

				return events.APIGatewayProxyResponse{
					StatusCode: http.StatusNotFound,
					Headers:    map[string]string{"Content-Type": "application/json"},
					Body:       string(responseBody),
				}, nil
			}

			log.Println("Error retrieving exchange rate history:", err)
			return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, err
		}

		responseBody, _ := json.Marshal(rates)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusOK,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       string(responseBody),
		}, nil
	} else {
		// Invalid endpoint
		errorResponse := ErrorResponse{Error: "Invalid endpoint"}
		responseBody, _ := json.Marshal(errorResponse)

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusNotFound,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       string(responseBody),
		}, nil
	}
}

func main() {
	lambda.Start(HandleRequest)
}
