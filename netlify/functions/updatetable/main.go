package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	_ "github.com/go-sql-driver/mysql"
)

// Database represents the MySQL database connection
type Database struct {
	DB *sql.DB
}

// CryptoResponse represents the response from the API
type CryptoResponse struct {
	Symbol string             `json:"symbol"`
	Rates  map[string]float64 `json:"rates"`
}

// ExchangeRate represents the exchange rate data to be inserted into the database
type ExchangeRate struct {
	CryptoID int     `json:"cryptocurrency_id"`
	FiatID   int     `json:"fiat_currency_id"`
	Rate     float64 `json:"rate"`
}

// APIResponse represents the response from the API
type APIResponse map[string]map[string]float64

func main() {
	lambda.Start(HandleRequest)
}

func HandleRequest(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	db, err := NewDatabase()
	if err != nil {
		log.Println("Error connecting to the database:", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, err
	}
	defer db.DB.Close()

	apiURL := "https://min-api.cryptocompare.com/data/pricemulti?fsyms=BTC,ETH,USDT,BNB,USDC,XRP,ADA,DOGE,LTC,SOL,TRX,DOT,MATIC,BCH,TON,WBTC,DAI,AVAX,SHIB,BUSD&tsyms=CNY,USD,EUR,JPY,GBP,KRW,INR,CAD,HKD,BRL,AUD,TWD,CHF,RUB,MXN,THB,SAR,AED,SGD,VND"
	response, err := http.Get(apiURL)
	if err != nil {
		log.Println("API call failed:", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusOK {
		var apiResp APIResponse
		err := json.NewDecoder(response.Body).Decode(&apiResp)
		if err != nil {
			log.Println("Error decoding API response:", err)
			return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, err
		}

		var exchangeRates []ExchangeRate
		cryptoMappings, err := db.GetCryptoMappings()
		if err != nil {
			log.Println("Error fetching crypto mappings:", err)
			return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, err
		}

		fiatMappings, err := db.GetFiatMappings()
		if err != nil {
			log.Println("Error fetching fiat mappings:", err)
			return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, err
		}

		for cryptoSymbol, rates := range apiResp {
			cryptoID := cryptoMappings[cryptoSymbol]
			if cryptoID != 0 {
				for fiatSymbol, rate := range rates {
					fiatID := fiatMappings[fiatSymbol]
					if fiatID != 0 {
						exchangeRates = append(exchangeRates, ExchangeRate{
							CryptoID: cryptoID,
							FiatID:   fiatID,
							Rate:     rate,
						})
					}
				}
			}
		}

		err = db.InsertExchangeRates(exchangeRates)
		if err != nil {
			log.Println("Error inserting exchange rates:", err)
			return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, err
		}

		return events.APIGatewayProxyResponse{StatusCode: http.StatusOK}, nil
	}

	log.Println("API call failed with status code:", response.StatusCode)
	return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, fmt.Errorf("API call failed with status code: %d", response.StatusCode)
}

// NewDatabase creates a new Database instance with a connection pool.
func NewDatabase() (*Database, error) {
	dbHost := os.Getenv("DB_HOST")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_DATABASE")

	connString := fmt.Sprintf("%s:%s@tcp(%s)/%s", dbUser, dbPassword, dbHost, dbName)
	db, err := sql.Open("mysql", connString)
	if err != nil {
		return nil, err
	}

	// Set a maximum connection pool size appropriate for your application's needs.
	db.SetMaxOpenConns(10)

	err = db.Ping()
	if err != nil {
		return nil, err
	}

	return &Database{DB: db}, nil
}

// GetCryptoMappings fetches the symbol-ID mappings for cryptocurrencies from the database.
func (d *Database) GetCryptoMappings() (map[string]int, error) {
	query := "SELECT symbol, cryptocurrency_id FROM Cryptocurrencies"
	rows, err := d.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	mappings := make(map[string]int)
	for rows.Next() {
		var symbol string
		var id int
		if err := rows.Scan(&symbol, &id); err != nil {
			return nil, err
		}
		mappings[symbol] = id
	}

	return mappings, nil
}

// GetFiatMappings fetches the symbol-ID mappings for fiat currencies from the database.
func (d *Database) GetFiatMappings() (map[string]int, error) {
	query := "SELECT symbol, fiat_currency_id FROM FiatCurrencies"
	rows, err := d.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	mappings := make(map[string]int)
	for rows.Next() {
		var symbol string
		var id int
		if err := rows.Scan(&symbol, &id); err != nil {
			return nil, err
		}
		mappings[symbol] = id
	}

	return mappings, nil
}

// InsertExchangeRates inserts the exchange rates into the database.
func (d *Database) InsertExchangeRates(rates []ExchangeRate) error {
	if len(rates) == 0 {
		return nil
	}

	query := "INSERT INTO ExchangeRates (cryptocurrency_id, fiat_currency_id, rate) VALUES (?, ?, ?)"
	stmt, err := d.DB.Prepare(query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, rate := range rates {
		_, err := stmt.Exec(rate.CryptoID, rate.FiatID, rate.Rate)
		if err != nil {
			return err
		}
	}

	return nil
}
