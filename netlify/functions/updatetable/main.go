package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	_ "github.com/go-sql-driver/mysql"
)

const (
	apiBaseURL = "https://min-api.cryptocompare.com/data/pricemulti"
)

var (
	dbHost     = os.Getenv("DB_HOST")
	dbUser     = os.Getenv("DB_USER")
	dbPassword = os.Getenv("DB_PASSWORD")
	dbName     = os.Getenv("DB_NAME")
)

// SymbolIDMapping represents the mapping of symbols to IDs
type SymbolIDMapping map[string]int

// ExchangeRate represents the exchange rate data
type ExchangeRate struct {
	CryptocurrencyID int     `json:"cryptocurrency_id"`
	FiatCurrencyID   int     `json:"fiat_currency_id"`
	Rate             float64 `json:"rate"`
	Timestamp        string  `json:"timestamp"`
}

// Function to fetch symbol-ID mappings from the database
func fetchSymbolIDMapping(table, symbolColumn, idColumn string) (SymbolIDMapping, error) {
	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s)/%s", dbUser, dbPassword, dbHost, dbName))
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(fmt.Sprintf("SELECT %s, %s FROM %s", symbolColumn, idColumn, table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	mapping := make(SymbolIDMapping)
	for rows.Next() {
		var symbol string
		var id int
		err := rows.Scan(&symbol, &id)
		if err != nil {
			return nil, err
		}
		mapping[symbol] = id
	}

	return mapping, nil
}

// Function to insert exchange rates into the database in bulk
func insertExchangeRates(rates []ExchangeRate) error {
	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s)/%s", dbUser, dbPassword, dbHost, dbName))
	if err != nil {
		return err
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare("INSERT INTO ExchangeRates (cryptocurrency_id, fiat_currency_id, rate, timestamp) VALUES (?, ?, ?, ?)")
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, rate := range rates {
		_, err := stmt.Exec(rate.CryptocurrencyID, rate.FiatCurrencyID, rate.Rate, rate.Timestamp)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		return err
	}

	return nil
}

// Handler function for the Lambda event
func lambdaHandler() error {
	// Fetch symbol-ID mappings for cryptocurrencies and fiat currencies
	cryptoSymbolIDMapping, err := fetchSymbolIDMapping("Cryptocurrencies", "symbol", "cryptocurrency_id")
	if err != nil {
		return err
	}

	fiatSymbolIDMapping, err := fetchSymbolIDMapping("FiatCurrencies", "symbol", "fiat_currency_id")
	if err != nil {
		return err
	}

	// Construct API call URL with symbols
	cryptoSymbols := strings.Join(keys(cryptoSymbolIDMapping), ",")
	fiatSymbols := strings.Join(keys(fiatSymbolIDMapping), ",")
	apiURL := fmt.Sprintf("%s?fsyms=%s&tsyms=%s", apiBaseURL, cryptoSymbols, fiatSymbols)

	// Make API call to fetch exchange rates
	response, err := http.Get(apiURL)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusOK {
		var data map[string]map[string]float64
		err := json.NewDecoder(response.Body).Decode(&data)
		if err != nil {
			return err
		}

		// Prepare exchange rates for bulk insertion
		var exchangeRates []ExchangeRate

		for cryptoSymbol, rates := range data {
			cryptoID := cryptoSymbolIDMapping[cryptoSymbol]

			for fiatSymbol, rate := range rates {
				fiatID := fiatSymbolIDMapping[fiatSymbol]
				exchangeRates = append(exchangeRates, ExchangeRate{
					CryptocurrencyID: cryptoID,
					FiatCurrencyID:   fiatID,
					Rate:             rate,
					Timestamp:        time.Now().Format("2006-01-02 15:04:05"),
				})
			}
		}

		// Bulk insert the exchange rates into the database
		err = insertExchangeRates(exchangeRates)
		if err != nil {
			return err
		}

		return nil
	}

	return fmt.Errorf("API call failed with status code: %d", response.StatusCode)
}

// Helper function to get keys from a map
func keys(m SymbolIDMapping) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func main() {
	lambda.Start(lambdaHandler)
}
