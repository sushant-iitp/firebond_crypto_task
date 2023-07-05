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

func handleTooManyInvalidParameters() events.APIGatewayProxyResponse {
	errorMessage := "Too many parameters. Please try again with valid parameters.\n\nValid URL formats:\n1. https://main--euphonious-brioche-40b22d.netlify.app/.netlify/functions/rate\n2. https://main--euphonious-brioche-40b22d.netlify.app/.netlify/functions/rate/{crypto}\n3. https://main--euphonious-brioche-40b22d.netlify.app/.netlify/functions/rate/{crypto}/{fiat}\n4. https://main--euphonious-brioche-40b22d.netlify.app/.netlify/functions/rate/history/{crypto}/{fiat}\n\nValid cryptocurrencies: BTC, ETH, USDT, BNB, USDC, XRP, ADA, DOGE, LTC, SOL\nValid fiat currencies: CNY, USD, EUR, JPY, GBP, KRW, INR, CAD, HKD, BRL"

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusBadRequest,
		Headers:    map[string]string{"Content-Type": "text/plain"},
		Body:       errorMessage,
	}
}

func handleInvalidParameters() events.APIGatewayProxyResponse {
	errorMessage := "Invalid parameters. Please try again with valid parameters.\n\nValid URL formats:\n1. https://main--euphonious-brioche-40b22d.netlify.app/.netlify/functions/rate\n2. https://main--euphonious-brioche-40b22d.netlify.app/.netlify/functions/rate/{crypto}\n3. https://main--euphonious-brioche-40b22d.netlify.app/.netlify/functions/rate/{crypto}/{fiat}\n4. https://main--euphonious-brioche-40b22d.netlify.app/.netlify/functions/rate/history/{crypto}/{fiat}\n\nValid cryptocurrencies: BTC, ETH, USDT, BNB, USDC, XRP, ADA, DOGE, LTC, SOL\nValid fiat currencies: CNY, USD, EUR, JPY, GBP, KRW, INR, CAD, HKD, BRL"

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusBadRequest,
		Headers:    map[string]string{"Content-Type": "text/plain"},
		Body:       errorMessage,
	}
}

func handleInvalidCryptoCurrency() events.APIGatewayProxyResponse {
	errorMessage := "Crypto currency does not exist or is not servicable. \nPlease try again with valid parameters.\n\nValid URL formats:\n1. https://main--euphonious-brioche-40b22d.netlify.app/.netlify/functions/rate\n2. https://main--euphonious-brioche-40b22d.netlify.app/.netlify/functions/rate/{crypto}\n3. https://main--euphonious-brioche-40b22d.netlify.app/.netlify/functions/rate/{crypto}/{fiat}\n4. https://main--euphonious-brioche-40b22d.netlify.app/.netlify/functions/rate/history/{crypto}/{fiat}\n\nValid cryptocurrencies: BTC, ETH, USDT, BNB, USDC, XRP, ADA, DOGE, LTC, SOL\nValid fiat currencies: CNY, USD, EUR, JPY, GBP, KRW, INR, CAD, HKD, BRL"

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusNotFound,
		Headers:    map[string]string{"Content-Type": "text/plain"},
		Body:       errorMessage,
	}
}

func handleInvalidFiatCurrency() events.APIGatewayProxyResponse {
	errorMessage := "Fiat currency does not exist or is not servicable. \nPlease try again with valid parameters.\n\nValid URL formats:\n1. https://main--euphonious-brioche-40b22d.netlify.app/.netlify/functions/rate\n2. https://main--euphonious-brioche-40b22d.netlify.app/.netlify/functions/rate/{crypto}\n3. https://main--euphonious-brioche-40b22d.netlify.app/.netlify/functions/rate/{crypto}/{fiat}\n4. https://main--euphonious-brioche-40b22d.netlify.app/.netlify/functions/rate/history/{crypto}/{fiat}\n\nValid cryptocurrencies: BTC, ETH, USDT, BNB, USDC, XRP, ADA, DOGE, LTC, SOL\nValid fiat currencies: CNY, USD, EUR, JPY, GBP, KRW, INR, CAD, HKD, BRL"

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusNotFound,
		Headers:    map[string]string{"Content-Type": "text/plain"},
		Body:       errorMessage,
	}
}

func handleExchangeRateNotFound() events.APIGatewayProxyResponse {
	errorMessage := "Exchange rates not found."

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusNotFound,
		Headers:    map[string]string{"Content-Type": "text/plain"},
		Body:       errorMessage,
	}

}

func handleGetExchangeRate(splitPath []string) (events.APIGatewayProxyResponse, error) {
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
		return handleInvalidCryptoCurrency(), nil
	}
	fiatExists, err := db.CheckFiatCurrency(fiat)
	if err != nil {
		log.Println("Error checking if fiat currency exists:", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, err
	}
	if !fiatExists {
		return handleInvalidFiatCurrency(), nil
	}

	rate, err := db.GetExchangeRate(crypto, fiat)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return handleExchangeRateNotFound(), nil
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
}

func handleGetExchangeRatesForCrypto(splitPath []string) (events.APIGatewayProxyResponse, error) {
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
		return handleInvalidCryptoCurrency(), nil
	}

	rates, err := db.GetExchangeRatesForCrypto(crypto)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return handleExchangeRateNotFound(), nil
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
}

func handleGetHistoricalExchangeRates(splitPath []string) (events.APIGatewayProxyResponse, error) {
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
		return handleInvalidCryptoCurrency(), nil
	}
	fiatExists, err := db.CheckFiatCurrency(fiat)
	if err != nil {
		log.Println("Error checking if fiat currency exists:", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, err
	}
	if !fiatExists {
		return handleInvalidFiatCurrency(), nil
	}

	rates, err := db.GetHistoricalExchangeRates(crypto, fiat)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return handleExchangeRateNotFound(), nil
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
}

func handleGetAllExchangeRates() (events.APIGatewayProxyResponse, error) {
	db, err := NewDatabase()
	if err != nil {
		log.Println("Error connecting to the database:", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, err
	}
	defer db.Close()

	rates, err := db.GetAllExchangeRates()
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return handleExchangeRateNotFound(), nil
		}
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

func HandleRequest(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	splitPath := strings.Split(request.Path, "/")
	numParams := len(splitPath)

	if numParams > 7 {
		return handleTooManyInvalidParameters(), nil
	} else if numParams == 6 {
		return handleGetExchangeRate(splitPath)
	} else if numParams == 5 && splitPath[4] != "" {
		return handleGetExchangeRatesForCrypto(splitPath)
	} else if numParams == 7 && splitPath[4] == "history" && splitPath[5] != "" && splitPath[6] != "" {
		return handleGetHistoricalExchangeRates(splitPath)
	} else if numParams == 4 {
		return handleGetAllExchangeRates()
	} else {
		return handleInvalidParameters(), nil
	}
}

func main() {
	lambda.Start(HandleRequest)
}