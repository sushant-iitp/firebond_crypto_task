package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	_ "github.com/go-sql-driver/mysql"
)

// Database connection
var db *sql.DB

// ApiResponse represents the JSON response format
type ApiResponse struct {
	Rate  float64   `json:"rate,omitempty"`
	Time  time.Time `json:"time,omitempty"`
	Error string    `json:"error,omitempty"`
}

func init() {
	// Initialize database connection
	dbHost := os.Getenv("DB_HOST")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_DATABASE")

	connectionString := fmt.Sprintf("%s:%s@tcp(%s)/%s", dbUser, dbPassword, dbHost, dbName)
	var err error
	db, err = sql.Open("mysql", connectionString)
	if err != nil {
		log.Fatal("Error connecting to the database: ", err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatal("Error pinging the database: ", err)
	}

	log.Println("Connected to the database")
}

func main() {
	lambda.Start(HandleRequest)
}

func HandleRequest(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	path := request.Path
	splitPath := strings.Split(path, "/")

	if len(splitPath) >= 4 && splitPath[3] == "singlerate" {
		if len(splitPath) == 7 && splitPath[4] == "history" {
			// Endpoint 4: /rate/history/{cryptocurrency}/{fiat}
			crypto := splitPath[5]
			fiat := splitPath[6]
			response := processHistoricalRequest(crypto, fiat)
			return createResponse(response), nil
		} else if len(splitPath) == 6 {
			// Endpoint 1: /rate/{cryptocurrency}/{fiat}
			crypto := splitPath[4]
			fiat := splitPath[5]
			response := processCryptoFiatRequest(crypto, fiat)
			return createResponse(response), nil
		} else if len(splitPath) == 5 {
			// Endpoint 2: /rate/{cryptocurrency}
			crypto := splitPath[4]
			response := processCryptoRequest(crypto)
			return createResponse(response), nil
		}
	} else if len(splitPath) == 4 && splitPath[3] == "rate" {
		// Endpoint 3: /rate
		response := processRatesRequest()
		return createResponse(response), nil
	}

	// Return 404 Not Found for unknown routes
	return events.APIGatewayProxyResponse{StatusCode: 404}, nil
}

func processCryptoFiatRequest(crypto, fiat string) ApiResponse {
	response := ApiResponse{}
	cryptoID, err := getCurrencyID("Cryptocurrencies", "symbol", crypto)
	if err != nil {
		response.Error = "Invalid Cryptocurrency/CryptoCurrency Not Supported"
		return response
	}

	fiatID, err := getCurrencyID("FiatCurrencies", "symbol", fiat)
	if err != nil {
		response.Error = "Invalid Fiat Currency/FiatCurrency Not Supported"
		return response
	}

	query := "SELECT rate FROM ExchangeRates WHERE cryptocurrency_id = ? AND fiat_currency_id = ? ORDER BY timestamp DESC LIMIT 1"
	err = db.QueryRow(query, cryptoID, fiatID).Scan(&response.Rate)
	if err != nil {
		response.Error = "Error fetching exchange rate"
	}

	return response
}

func processCryptoRequest(crypto string) ApiResponse {
	response := ApiResponse{}
	cryptoID, err := getCurrencyID("Cryptocurrencies", "symbol", crypto)
	if err != nil {
		response.Error = "Invalid Cryptocurrency"
		return response
	}

	query := "SELECT fiat_currency_id, rate FROM ExchangeRates WHERE cryptocurrency_id = ? ORDER BY timestamp DESC"
	rows, err := db.Query(query, cryptoID)
	if err != nil {
		response.Error = "Error fetching exchange rates"
		return response
	}
	defer rows.Close()

	for rows.Next() {
		var fiatID int
		var rate float64
		err := rows.Scan(&fiatID, &rate)
		if err != nil {
			response.Error = "Error scanning database rows"
			return response
		}

		fiatSymbol, err := getCurrencySymbol("FiatCurrencies", fiatID)
		if err != nil {
			response.Error = "Error fetching fiat currency symbol"
			return response
		}

		response.Rate = rate
		response.Time = time.Now()
		// Print the rate for each fiat currency
		fmt.Printf("%s: %f\n", fiatSymbol, rate)
	}

	return response
}

func processRatesRequest() ApiResponse {
	response := ApiResponse{}

	query := "SELECT cryptocurrency_id, fiat_currency_id, rate FROM ExchangeRates"
	rows, err := db.Query(query)
	if err != nil {
		response.Error = "Error fetching exchange rates"
		return response
	}
	defer rows.Close()

	for rows.Next() {
		var cryptoID, fiatID int
		var rate float64
		err := rows.Scan(&cryptoID, &fiatID, &rate)
		if err != nil {
			response.Error = "Error scanning database rows"
			return response
		}

		cryptoSymbol, err := getCurrencySymbol("Cryptocurrencies", cryptoID)
		if err != nil {
			response.Error = "Error fetching cryptocurrency symbol"
			return response
		}

		fiatSymbol, err := getCurrencySymbol("FiatCurrencies", fiatID)
		if err != nil {
			response.Error = "Error fetching fiat currency symbol"
			return response
		}

		fmt.Printf("%s-%s: %f\n", cryptoSymbol, fiatSymbol, rate)
	}

	return response
}

func processHistoricalRequest(crypto, fiat string) ApiResponse {
	response := ApiResponse{}

	if crypto != "history" {
		response.Error = "Invalid request: Please modify your request"
		return response
	}

	cryptoID, err := getCurrencyID("Cryptocurrencies", "symbol", fiat)
	if err != nil {
		response.Error = "Invalid Cryptocurrency"
		return response
	}

	fiatID, err := getCurrencyID("FiatCurrencies", "symbol", fiat)
	if err != nil {
		response.Error = "Invalid Fiat Currency"
		return response
	}

	query := "SELECT rate, timestamp FROM ExchangeRates WHERE cryptocurrency_id = ? AND fiat_currency_id = ? AND timestamp >= DATE_SUB(NOW(), INTERVAL 24 HOUR)"
	rows, err := db.Query(query, cryptoID, fiatID)
	if err != nil {
		response.Error = "Error fetching historical exchange rates"
		return response
	}
	defer rows.Close()

	for rows.Next() {
		var rate float64
		var timestamp time.Time
		err := rows.Scan(&rate, &timestamp)
		if err != nil {
			response.Error = "Error scanning database rows"
			return response
		}

		response.Rate = rate
		response.Time = timestamp
		// Print the rate and timestamp for each record
		fmt.Printf("Rate: %f, Time: %s\n", rate, timestamp)
	}

	return response
}

func getCurrencyID(table, column, symbol string) (int, error) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE symbol = ?", column, table)
	var id int
	err := db.QueryRow(query, symbol).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func getCurrencySymbol(table string, id int) (string, error) {
	query := fmt.Sprintf("SELECT symbol FROM %s WHERE %s_id = ?", table, strings.ToLower(table))
	var symbol string
	err := db.QueryRow(query, id).Scan(&symbol)
	if err != nil {
		return "", err
	}
	return symbol, nil
}

func createResponse(response ApiResponse) events.APIGatewayProxyResponse {
	body, err := json.Marshal(response)
	if err != nil {
		log.Println("Error marshaling JSON response:", err)
		return events.APIGatewayProxyResponse{StatusCode: 500}
	}

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       string(body),
		Headers:    map[string]string{"Content-Type": "application/json"},
	}
}
