package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB

// Cryptocurrency represents the Cryptocurrencies table
type Cryptocurrency struct {
	ID     int    `json:"cryptocurrency_id"`
	Symbol string `json:"symbol"`
}

// FiatCurrency represents the FiatCurrencies table
type FiatCurrency struct {
	ID     int    `json:"fiat_currency_id"`
	Symbol string `json:"symbol"`
}

// ExchangeRate represents the ExchangeRates table
type ExchangeRate struct {
	ID               int     `json:"exchange_rate_id"`
	CryptocurrencyID int     `json:"cryptocurrency_id"`
	FiatCurrencyID   int     `json:"fiat_currency_id"`
	Rate             float64 `json:"rate"`
	Timestamp        string  `json:"timestamp"`
}

// Response represents the response data
type Response struct {
	Crypto  string             `json:"cryptocurrency,omitempty"`
	Value   float64            `json:"value,omitempty"`
	Fiat    string             `json:"fiat,omitempty"`
	Ticker  map[string]float64 `json:"ticker,omitempty"`
	History []ExchangeRate     `json:"history,omitempty"`
	Error   string             `json:"error,omitempty"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error"`
}

func main() {
	lambda.Start(HandleRequest)
}

func HandleRequest(ctx events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	path := ctx.Path
	splitPath := strings.Split(path, "/")

	if len(splitPath) >= 4 && splitPath[3] == "singlerate" {
		switch len(splitPath) {
		case 6:
			// /rate/{crypto}/{fiat}
			crypto := splitPath[4]
			fiat := splitPath[5]
			response := processCryptoFiatRequest(crypto, fiat)
			return createResponse(response), nil
		case 5:
			// /rate/{crypto}
			crypto := splitPath[4]
			response := processCryptoRequest(crypto)
			return createResponse(response), nil
		case 4:
			// /rate
			response := processRatesRequest()
			return createResponse(response), nil
		case 7:
			// /rate/history/{crypto}/{fiat}
			if splitPath[4] == "history" {
				crypto := splitPath[5]
				fiat := splitPath[6]
				response := processHistoricalRequest(crypto, fiat)
				return createResponse(response), nil
			}
		}
	}

	return createErrorResponse("Invalid request: Please modify your request"), nil
}

func createErrorResponse(message string) events.APIGatewayProxyResponse {
	errorResponse := ErrorResponse{Error: message}
	responseBody, _ := json.Marshal(errorResponse)
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusBadRequest,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       string(responseBody),
	}
}

func createResponse(data interface{}) events.APIGatewayProxyResponse {
	responseBody, _ := json.Marshal(data)
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       string(responseBody),
	}
}

func processCryptoFiatRequest(crypto, fiat string) Response {
	// Check if the given cryptocurrency and fiat currency exist
	cryptoID, err := getCryptocurrencyID(crypto)
	if err != nil {
		return Response{Error: "Invalid Cryptocurrency/CryptoCurrency Not Supported"}
	}

	fiatID, err := getFiatCurrencyID(fiat)
	if err != nil {
		return Response{Error: "Invalid Fiat Currency/FiatCurrency Not Supported"}
	}

	// Fetch the exchange rate from the database
	rate, err := getExchangeRate(cryptoID, fiatID)
	if err != nil {
		return Response{Error: "Invalid Cryptocurrency - FiatCurrency Pair / Pair not serviceable"}
	}

	response := Response{
		Crypto: crypto,
		Value:  rate,
		Fiat:   fiat,
	}

	return response
}

func processCryptoRequest(crypto string) Response {
	// Check if the given cryptocurrency exists
	cryptoID, err := getCryptocurrencyID(crypto)
	if err != nil {
		return Response{Error: "Invalid Cryptocurrency"}
	}

	// Fetch the exchange rates for the given cryptocurrency
	rates, err := getExchangeRatesForCrypto(cryptoID)
	if err != nil {
		return Response{Error: "Invalid Cryptocurrency"}
	}

	response := Response{
		Crypto: crypto,
		Ticker: rates,
	}

	return response
}

func processRatesRequest() Response {
	// Fetch all exchange rates
	rates, err := getAllExchangeRates()
	if err != nil {
		return Response{Error: "Failed to fetch exchange rates"}
	}

	response := Response{
		Ticker: rates,
	}

	return response
}

func processHistoricalRequest(crypto, fiat string) Response {
	// Check if the given cryptocurrency and fiat currency exist
	cryptoID, err := getCryptocurrencyID(crypto)
	if err != nil {
		return Response{Error: "Invalid Cryptocurrency"}
	}

	fiatID, err := getFiatCurrencyID(fiat)
	if err != nil {
		return Response{Error: "Invalid Fiat Currency"}
	}

	// Fetch the historical exchange rates for the past 24 hours
	rates, err := getHistoricalExchangeRates(cryptoID, fiatID)
	if err != nil {
		return Response{Error: "Invalid Cryptocurrency - FiatCurrency Pair / Pair not serviceable"}
	}

	response := Response{
		Crypto:  crypto,
		Fiat:    fiat,
		History: rates,
	}

	return response
}

func getCryptocurrencyID(symbol string) (int, error) {
	query := "SELECT cryptocurrency_id FROM Cryptocurrencies WHERE symbol = ?"
	row := db.QueryRow(query, symbol)

	var id int
	err := row.Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to get cryptocurrency ID: %v", err)
	}

	return id, nil
}

func getFiatCurrencyID(symbol string) (int, error) {
	query := "SELECT fiat_currency_id FROM FiatCurrencies WHERE symbol = ?"
	row := db.QueryRow(query, symbol)

	var id int
	err := row.Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to get fiat currency ID: %v", err)
	}

	return id, nil
}

func getExchangeRate(cryptoID, fiatID int) (float64, error) {
	query := "SELECT rate FROM ExchangeRates WHERE cryptocurrency_id = ? AND fiat_currency_id = ? ORDER BY timestamp DESC LIMIT 1"
	row := db.QueryRow(query, cryptoID, fiatID)

	var rate float64
	err := row.Scan(&rate)
	if err != nil {
		return 0, fmt.Errorf("failed to get exchange rate: %v", err)
	}

	return rate, nil
}

func getExchangeRatesForCrypto(cryptoID int) (map[string]float64, error) {
	query := "SELECT c.symbol, er.rate FROM ExchangeRates er JOIN FiatCurrencies f ON er.fiat_currency_id = f.fiat_currency_id JOIN Cryptocurrencies c ON er.cryptocurrency_id = c.cryptocurrency_id WHERE c.cryptocurrency_id = ?"
	rows, err := db.Query(query, cryptoID)
	if err != nil {
		return nil, fmt.Errorf("failed to get exchange rates for cryptocurrency: %v", err)
	}
	defer rows.Close()

	rates := make(map[string]float64)
	for rows.Next() {
		var symbol string
		var rate float64
		err := rows.Scan(&symbol, &rate)
		if err != nil {
			return nil, fmt.Errorf("failed to scan exchange rates for cryptocurrency: %v", err)
		}
		rates[symbol] = rate
	}

	return rates, nil
}

func getAllExchangeRates() (map[string]float64, error) {
	query := "SELECT c.symbol, er.rate FROM ExchangeRates er JOIN FiatCurrencies f ON er.fiat_currency_id = f.fiat_currency_id JOIN Cryptocurrencies c ON er.cryptocurrency_id = c.cryptocurrency_id"
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get all exchange rates: %v", err)
	}
	defer rows.Close()

	rates := make(map[string]float64)
	for rows.Next() {
		var symbol string
		var rate float64
		err := rows.Scan(&symbol, &rate)
		if err != nil {
			return nil, fmt.Errorf("failed to scan all exchange rates: %v", err)
		}
		rates[symbol] = rate
	}

	return rates, nil
}

func getHistoricalExchangeRates(cryptoID, fiatID int) ([]ExchangeRate, error) {
	query := "SELECT rate, timestamp FROM ExchangeRates WHERE cryptocurrency_id = ? AND fiat_currency_id = ? AND timestamp >= DATE_SUB(NOW(), INTERVAL 24 HOUR)"
	rows, err := db.Query(query, cryptoID, fiatID)
	if err != nil {
		return nil, fmt.Errorf("failed to get historical exchange rates: %v", err)
	}
	defer rows.Close()

	var rates []ExchangeRate
	for rows.Next() {
		var rate float64
		var timestamp string
		err := rows.Scan(&rate, &timestamp)
		if err != nil {
			return nil, fmt.Errorf("failed to scan historical exchange rates: %v", err)
		}
		rates = append(rates, ExchangeRate{Rate: rate, Timestamp: timestamp})
	}

	return rates, nil
}

func init() {
	host := os.Getenv("DB_HOST")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	database := os.Getenv("DB_DATABASE")

	connectionString := fmt.Sprintf("%s:%s@tcp(%s)/%s", user, password, host, database)

	var err error
	db, err = sql.Open("mysql", connectionString)
	if err != nil {
		log.Fatal(err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}
}
