package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	_ "github.com/go-sql-driver/mysql"
)

type Database struct {
	DB *sql.DB
}

type CryptoResponse struct {
	Value float64 `json:"value"`
}

type CryptoResponseWithTimestamp struct {
	Value     float64 `json:"value"`
	Timestamp string  `json:"timestamp"`
}

type HistoricalRateResponse struct {
	ExchangeRate []CryptoResponseWithTimestamp `json:"exchange_rate"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func (d *Database) CheckCryptoCurrency(crypto string) (bool, error) {
	var exists bool
	err := d.DB.QueryRow("SELECT EXISTS (SELECT 1 FROM Cryptocurrencies WHERE symbol = ?)", crypto).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (d *Database) CheckFiatCurrency(fiat string) (bool, error) {
	var exists bool
	err := d.DB.QueryRow("SELECT EXISTS (SELECT 1 FROM FiatCurrencies WHERE symbol = ?)", fiat).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func NewDatabase() (*Database, error) {
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbHost := os.Getenv("DB_HOST")
	dbDatabase := os.Getenv("DB_DATABASE")

	connString := dbUser + ":" + dbPassword + "@tcp(" + dbHost + ")/" + dbDatabase + "?parseTime=true"
	db, err := sql.Open("mysql", connString)
	if err != nil {
		return nil, err
	}

	err = db.Ping()
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
	`

	row := d.DB.QueryRow(query, crypto, fiat)

	var rate float64
	err := row.Scan(&rate)
	if err != nil {
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
		SELECT c.symbol, f.symbol, er.rate
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

		if rates[crypto] == nil {
			rates[crypto] = make(map[string]float64)
		}
		rates[crypto][fiat] = rate
	}

	return rates, nil
}

func (d *Database) GetHistoricalExchangeRates(crypto, fiat string) ([]CryptoResponseWithTimestamp, error) {
	query := `
		SELECT er.rate, er.timestamp
		FROM ExchangeRates er
		JOIN Cryptocurrencies c ON c.cryptocurrency_id = er.cryptocurrency_id
		JOIN FiatCurrencies f ON f.fiat_currency_id = er.fiat_currency_id
		WHERE c.symbol = ? AND f.symbol = ? AND er.timestamp >= NOW() - INTERVAL 24 HOUR
	`

	rows, err := d.DB.Query(query, crypto, fiat)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rates := make([]CryptoResponseWithTimestamp, 0)
	for rows.Next() {
		var rate float64
		var timestamp string
		if err := rows.Scan(&rate, &timestamp); err != nil {
			return nil, err
		}
		response := CryptoResponseWithTimestamp{
			Value:     rate,
			Timestamp: timestamp,
		}
		rates = append(rates, response)
	}

	return rates, nil
}

func HandleRequest(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	splitPath := strings.Split(request.Path, "/")

	if len(splitPath) > 7 {
		errorMessage := `Too many parameters. Please try again with valid parameters.
		Valid URL formats:
		1. https://main--euphonious-brioche-40b22d.netlify.app/.netlify/functions/rate
		2. https://main--euphonious-brioche-40b22d.netlify.app/.netlify/functions/rate/{crypto}
		3. https://main--euphonious-brioche-40b22d.netlify.app/.netlify/functions/rate/{crypto}/{fiat}
		4. https://main--euphonious-brioche-40b22d.netlify.app/.netlify/functions/rate/history/{crypto}/{fiat}

		Valid cryptocurrencies: BTC, ETH, USDT, BNB, USDC, XRP, ADA, DOGE, LTC, SOL
		Valid fiat currencies: CNY, USD, EUR, JPY, GBP, KRW, INR, CAD, HKD, BRL"`

		errorResponse := ErrorResponse{Error: errorMessage}
		responseBody, _ := json.Marshal(errorResponse)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       string(responseBody),
		}, nil
	} else if len(splitPath) == 6 {
		// GET /rates/{crypto}/{fiat}
		crypto := splitPath[4]
		fiat := splitPath[5]

		db, err := NewDatabase()
		if err != nil {
			log.Println("Error connecting to the database:", err)
			return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, err
		}
		defer db.Close()

		cryptoExists, err := db.CheckCryptoCurrency(crypto)
		if err != nil {
			log.Println("Error checking if crypto currency exists:", err)
			return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, err
		}
		if !cryptoExists {
			errorResponse := ErrorResponse{Error: "Crypto currency does not exist or is not servicable"}
			responseBody, _ := json.Marshal(errorResponse)
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusNotFound,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       string(responseBody),
			}, nil
		}

		fiatExists, err := db.CheckFiatCurrency(fiat)
		if err != nil {
			log.Println("Error checking if fiat currency exists:", err)
			return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, err
		}
		if !fiatExists {
			errorResponse := ErrorResponse{Error: "Fiat currency does not exist or is not servicable"}
			responseBody, _ := json.Marshal(errorResponse)
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusNotFound,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       string(responseBody),
			}, nil
		}

		rate, err := db.GetExchangeRate(crypto, fiat)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				errorResponse := ErrorResponse{Error: "Exchange rate not found"}
				responseBody, _ := json.Marshal(errorResponse)
				return events.APIGatewayProxyResponse{
					StatusCode: http.StatusNotFound,
					Headers:    map[string]string{"Content-Type": "application/json"},
					Body:       string(responseBody),
				}, nil
			}
			log.Println("Error retrieving exchange rate:", err)
			return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, err
		}

		response := CryptoResponse{Value: rate}
		responseBody, _ := json.Marshal(response)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusOK,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       string(responseBody),
		}, nil
	} else if len(splitPath) == 5 && splitPath[4] != "" {
		// GET /rates/{crypto}
		crypto := splitPath[4]

		db, err := NewDatabase()
		if err != nil {
			log.Println("Error connecting to the database:", err)
			return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, err
		}
		defer db.Close()

		cryptoExists, err := db.CheckCryptoCurrency(crypto)
		if err != nil {
			log.Println("Error checking if crypto currency exists:", err)
			return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, err
		}
		if !cryptoExists {
			errorResponse := ErrorResponse{Error: "Crypto currency does not exist or is not servicable"}
			responseBody, _ := json.Marshal(errorResponse)
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusNotFound,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       string(responseBody),
			}, nil
		}

		rates, err := db.GetExchangeRatesForCrypto(crypto)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				errorResponse := ErrorResponse{Error: "Exchange rates not found for the specified cryptocurrency"}
				responseBody, _ := json.Marshal(errorResponse)
				return events.APIGatewayProxyResponse{
					StatusCode: http.StatusNotFound,
					Headers:    map[string]string{"Content-Type": "application/json"},
					Body:       string(responseBody),
				}, nil
			}
			log.Println("Error retrieving exchange rates:", err)
			return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, err
		}

		response := make(map[string]float64)
		for fiat, rate := range rates {
			response[fiat] = rate
		}

		responseBody, _ := json.Marshal(response)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusOK,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       string(responseBody),
		}, nil
	} else if len(splitPath) == 7 && splitPath[4] == "history" && splitPath[5] != "" && splitPath[6] != "" {
		// GET /rate/history/{crypto}/{fiat}
		crypto := splitPath[5]
		fiat := splitPath[6]

		db, err := NewDatabase()
		if err != nil {
			log.Println("Error connecting to the database:", err)
			return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, err
		}
		defer db.Close()

		cryptoExists, err := db.CheckCryptoCurrency(crypto)
		if err != nil {
			log.Println("Error checking if crypto currency exists:", err)
			return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, err
		}
		if !cryptoExists {
			errorResponse := ErrorResponse{Error: "Crypto currency does not exist or is not serviceable"}
			responseBody, _ := json.Marshal(errorResponse)
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusNotFound,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       string(responseBody),
			}, nil
		}

		fiatExists, err := db.CheckFiatCurrency(fiat)
		if err != nil {
			log.Println("Error checking if fiat currency exists:", err)
			return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, err
		}
		if !fiatExists {
			errorResponse := ErrorResponse{Error: "Fiat currency does not exist or is not serviceable"}
			responseBody, _ := json.Marshal(errorResponse)
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusNotFound,
				Headers:    map[string]string{"Content-Type": "application/json"},
				Body:       string(responseBody),
			}, nil
		}

		rates, err := db.GetHistoricalExchangeRates(crypto, fiat)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				errorResponse := ErrorResponse{Error: "Historical exchange rates not found for the specified cryptocurrency and fiat currency"}
				responseBody, _ := json.Marshal(errorResponse)
				return events.APIGatewayProxyResponse{
					StatusCode: http.StatusNotFound,
					Headers:    map[string]string{"Content-Type": "application/json"},
					Body:       string(responseBody),
				}, nil
			}
			log.Println("Error retrieving historical exchange rates:", err)
			return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, err
		}

		response := HistoricalRateResponse{ExchangeRate: rates}
		responseBody, _ := json.Marshal(response)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusOK,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       string(responseBody),
		}, nil
	} else {
		// GET /rates
		db, err := NewDatabase()
		if err != nil {
			log.Println("Error connecting to the database:", err)
			return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, err
		}
		defer db.Close()

		rates, err := db.GetAllExchangeRates()
		if err != nil {
			log.Println("Error retrieving exchange rates:", err)
			return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, err
		}

		responseBody, _ := json.Marshal(rates)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusOK,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       string(responseBody),
		}, nil
	}
}

func main() {
	lambda.Start(HandleRequest)
}
