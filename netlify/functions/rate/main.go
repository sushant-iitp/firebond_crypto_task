package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	_ "github.com/go-sql-driver/mysql"
)

type CryptoResponse struct {
	Crypto string  `json:"cryptocurrency"`
	Value  float64 `json:"value"`
	Fiat   string  `json:"fiat"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type Database struct {
	DB *sql.DB
}

func NewDatabase() (*Database, error) {
	host := os.Getenv("DB_HOST")
	username := os.Getenv("DB_USER")
	database := os.Getenv("DB_DATABASE")
	password := os.Getenv("DB_PASSWORD")
	port := os.Getenv("DB_PORT")

	connectionString := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", username, password, host, port, database)
	db, err := sql.Open("mysql", connectionString)
	if err != nil {
		return nil, err
	}

	return &Database{DB: db}, nil
}

func (d *Database) Close() error {
	return d.DB.Close()
}

func (d *Database) GetExchangeRate(crypto, fiat string) (float64, error) {
	query := `
		SELECT rate
		FROM ExchangeRates er
		JOIN Cryptocurrencies c ON c.cryptocurrency_id = er.cryptocurrency_id
		JOIN FiatCurrencies f ON f.fiat_currency_id = er.fiat_currency_id
		WHERE c.symbol = ? AND f.symbol = ?
		ORDER BY er.timestamp DESC
		LIMIT 1
	`

	var rate float64
	err := d.DB.QueryRow(query, crypto, fiat).Scan(&rate)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("Invalid Cryptocurrency-FiatCurrency Pair / Pair not servicable")
		}
		return 0, err
	}

	return rate, nil
}

func (d *Database) GetExchangeRatesForCrypto(crypto string) (map[string]float64, error) {
	query := `
		SELECT f.symbol, er.rate
		FROM ExchangeRates er
		JOIN Cryptocurrencies c ON c.cryptocurrency_id = er.cryptocurrency_id
		JOIN FiatCurrencies f ON f.fiat_currency_id = er.fiat_currency_id
		WHERE c.symbol = ?
	`

	rows, err := d.DB.Query(query, crypto)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rates := make(map[string]float64)
	for rows.Next() {
		var fiat string
		var rate float64
		if err := rows.Scan(&fiat, &rate); err != nil {
			return nil, err
		}
		rates[fiat] = rate
	}

	return rates, nil
}

func (d *Database) GetAllExchangeRates() (map[string]map[string]float64, error) {
	query := `
		SELECT c.symbol AS crypto, f.symbol AS fiat, er.rate
		FROM ExchangeRates er
		JOIN Cryptocurrencies c ON c.cryptocurrency_id = er.cryptocurrency_id
		JOIN FiatCurrencies f ON f.fiat_currency_id = er.fiat_currency_id
	`

	rows, err := d.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rates := make(map[string]map[string]float64)
	for rows.Next() {
		var crypto, fiat string
		var rate float64
		if err := rows.Scan(&crypto, &fiat, &rate); err != nil {
			return nil, err
		}

		if _, ok := rates[crypto]; !ok {
			rates[crypto] = make(map[string]float64)
		}
		rates[crypto][fiat] = rate
	}

	return rates, nil
}

func (d *Database) GetHistoricalExchangeRates(crypto, fiat string) ([]CryptoResponse, error) {
	query := `
		SELECT c.symbol AS crypto, f.symbol AS fiat, er.rate
		FROM ExchangeRates er
		JOIN Cryptocurrencies c ON c.cryptocurrency_id = er.cryptocurrency_id
		JOIN FiatCurrencies f ON f.fiat_currency_id = er.fiat_currency_id
		WHERE c.symbol = ? AND f.symbol = ?
		AND er.timestamp > ?
	`

	startTime := time.Now().Add(-24 * time.Hour)
	rows, err := d.DB.Query(query, crypto, fiat, startTime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rates []CryptoResponse
	for rows.Next() {
		var crypto, fiat string
		var rate float64
		if err := rows.Scan(&crypto, &fiat, &rate); err != nil {
			return nil, err
		}

		response := CryptoResponse{
			Crypto: crypto,
			Value:  rate,
			Fiat:   fiat,
		}
		rates = append(rates, response)
	}

	return rates, nil
}

func (d *Database) Ping() error {
	return d.DB.Ping()
}

func HandleRequest(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	path := request.Path
	splitPath := strings.Split(path, "/")

	if len(splitPath) == 2 {
		// GET /rates/{cryptocurrency}/{fiat}
		crypto := splitPath[1]
		fiat := splitPath[2]
		db, err := NewDatabase()
		if err != nil {
			return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, err
		}
		defer db.Close()

		rate, err := db.GetExchangeRate(crypto, fiat)
		if err != nil {
			errorResponse := ErrorResponse{Error: err.Error()}
			responseBody, _ := json.Marshal(errorResponse)
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusBadRequest,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       string(responseBody),
			}, nil
		}

		response := CryptoResponse{
			Crypto: crypto,
			Value:  rate,
			Fiat:   fiat,
		}
		responseBody, _ := json.Marshal(response)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusOK,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       string(responseBody),
		}, nil
	} else if len(splitPath) == 1 {
		// GET /rates/{cryptocurrency}
		crypto := splitPath[1]

		db, err := NewDatabase()
		if err != nil {
			return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, err
		}
		defer db.Close()

		rates, err := db.GetExchangeRatesForCrypto(crypto)
		if err != nil {
			errorResponse := ErrorResponse{Error: err.Error()}
			responseBody, _ := json.Marshal(errorResponse)
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusBadRequest,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       string(responseBody),
			}, nil
		}

		responseBody, _ := json.Marshal(rates)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusOK,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       string(responseBody),
		}, nil
	} else if len(splitPath) == 0 {
		// GET /rates
		db, err := NewDatabase()
		if err != nil {
			return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, err
		}
		defer db.Close()

		rates, err := db.GetAllExchangeRates()
		if err != nil {
			errorResponse := ErrorResponse{Error: err.Error()}
			responseBody, _ := json.Marshal(errorResponse)
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusBadRequest,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       string(responseBody),
			}, nil
		}

		responseBody, _ := json.Marshal(rates)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusOK,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       string(responseBody),
		}, nil
	} else if len(splitPath) == 3 {
		// GET /rates/history/{cryptocurrency}/{fiat}
		crypto := splitPath[2]
		fiat := splitPath[3]

		db, err := NewDatabase()
		if err != nil {
			return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, err
		}
		defer db.Close()

		rates, err := db.GetHistoricalExchangeRates(crypto, fiat)
		if err != nil {
			errorResponse := ErrorResponse{Error: err.Error()}
			responseBody, _ := json.Marshal(errorResponse)
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusBadRequest,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       string(responseBody),
			}, nil
		}

		responseBody, _ := json.Marshal(rates)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusOK,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       string(responseBody),
		}, nil
	}

	return events.APIGatewayProxyResponse{StatusCode: http.StatusNotFound}, nil
}

func main() {
	lambda.Start(HandleRequest)
}
