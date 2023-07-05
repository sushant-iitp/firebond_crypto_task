package main

import (
	"database/sql"
	"log"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
)

func setup() {
	//Insert your DB credentials(User,Password,Host & database) here.
	db, err := sql.Open("mysql", "user:password@tcp(host)/database")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS FiatCurrencies (fiat_currency_id INT PRIMARY KEY AUTO_INCREMENT, symbol VARCHAR(10) UNIQUE)")
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS Cryptocurrencies (cryptocurrency_id INT PRIMARY KEY AUTO_INCREMENT, symbol VARCHAR(10) UNIQUE)")
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS ExchangeRates (exchange_rate_id INT PRIMARY KEY AUTO_INCREMENT, cryptocurrency_id INT, fiat_currency_id INT, rate FLOAT, timestamp DATETIME)")
	if err != nil {
		log.Fatal(err)
	}
}

func tearDown() {
	//Insert your DB credentials(User,Password,Host & database) here.
	db, err := sql.Open("mysql", "user:password@tcp(host)/database")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec("DROP TABLE IF EXISTS ExchangeRates")
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec("DROP TABLE IF EXISTS Cryptocurrencies")
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec("DROP TABLE IF EXISTS FiatCurrencies")
	if err != nil {
		log.Fatal(err)
	}
}

func TestNewDatabase(t *testing.T) {
	db, err := NewDatabase()
	defer db.Close()

	assert.NoError(t, err)
	assert.NotNil(t, db.DB)

	err = db.DB.Ping()
	assert.NoError(t, err)
}

func TestInsertFiatCurrencies(t *testing.T) {
	db, err := NewDatabase()
	defer db.Close()
	assert.NoError(t, err)

	symbols := []string{"USD", "EUR", "GBP"}

	for _, symbol := range symbols {
		_, err := db.DB.Exec("INSERT INTO FiatCurrencies (symbol) VALUES (?)", symbol)
		assert.NoError(t, err)
	}

	var count int
	err = db.DB.QueryRow("SELECT COUNT(*) FROM FiatCurrencies").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, len(symbols), count)
}

func TestInsertCryptocurrencies(t *testing.T) {
	db, err := NewDatabase()
	defer db.Close()
	assert.NoError(t, err)

	symbols := []string{"BTC", "ETH", "LTC"}

	for _, symbol := range symbols {
		_, err := db.DB.Exec("INSERT INTO Cryptocurrencies (symbol) VALUES (?)", symbol)
		assert.NoError(t, err)
	}

	var count int
	err = db.DB.QueryRow("SELECT COUNT(*) FROM Cryptocurrencies").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, len(symbols), count)
}

func TestInsertExchangeRates(t *testing.T) {
	db, err := NewDatabase()
	defer db.Close()
	assert.NoError(t, err)

	// Prepare the values for the insertion
	cryptocurrencyID := 1
	fiatCurrencyID := 1
	rate := 25.2
	timestamp := time.Now()

	_, err = db.DB.Exec("INSERT INTO ExchangeRates (cryptocurrency_id, fiat_currency_id, rate, timestamp) VALUES (?,?,?,?)", cryptocurrencyID, fiatCurrencyID, rate, timestamp)
	assert.NoError(t, err)

	var count int
	err = db.DB.QueryRow("SELECT COUNT(*) FROM ExchangeRates").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestCheckCryptoCurrency(t *testing.T) {
	db, err := NewDatabase()
	defer db.Close()
	assert.NoError(t, err)

	_, err = db.DB.Exec("INSERT INTO Cryptocurrencies (symbol) VALUES (?)", "BTC")

	symbol := "BTC"
	exists, err := db.CheckCryptoCurrency(symbol)
	assert.NoError(t, err)
	assert.True(t, exists)
}

func TestCheckFiatCurrency(t *testing.T) {
	db, err := NewDatabase()
	defer db.Close()
	assert.NoError(t, err)

	_, err = db.DB.Exec("INSERT INTO FiatCurrencies (symbol) VALUES (?)", "USD")
	symbol := "USD"
	exists, err := db.CheckFiatCurrency(symbol)
	assert.NoError(t, err)
	assert.True(t, exists)
}

func TestGetExchangeRate(t *testing.T) {
	db, err := NewDatabase()
	defer db.Close()
	assert.NoError(t, err)

	cryptoSymbol := "BTC"
	fiatSymbol := "USD"
	_, err = db.DB.Exec("INSERT INTO Cryptocurrencies (symbol) VALUES (?)", cryptoSymbol)
	assert.NoError(t, err)
	_, err = db.DB.Exec("INSERT INTO FiatCurrencies (symbol) VALUES (?)", fiatSymbol)
	assert.NoError(t, err)
	cryptocurrencyID := 1
	fiatCurrencyID := 1
	rate := 25.254
	timestamp := time.Now()

	_, err = db.DB.Exec("INSERT INTO ExchangeRates (cryptocurrency_id, fiat_currency_id, rate, timestamp) VALUES (?,?,?,?)", cryptocurrencyID, fiatCurrencyID, rate, timestamp)
	assert.NoError(t, err)
	getrate, err := db.GetExchangeRate(cryptoSymbol, fiatSymbol)
	assert.NoError(t, err)
	assert.NotZero(t, getrate)
}

// TestGetExchangeRatesForCrypto tests the retrieval of exchange rates for a given cryptocurrency.
func TestGetExchangeRatesForCrypto(t *testing.T) {
	db, err := NewDatabase()
	defer db.Close()
	assert.NoError(t, err)

	// Prepare the test data
	cryptoSymbol := "BTC"
	fiatSymbol1 := "USD"
	fiatSymbol2 := "INR"
	cryptocurrencyID := 1
	fiatCurrencyID1 := 1
	fiatCurrencyID2 := 2
	rate1 := 25.2
	rate2 := 30.5
	timestamp := time.Now()

	// Insert the necessary data into the database
	_, err = db.DB.Exec("INSERT INTO Cryptocurrencies (symbol) VALUES (?)", cryptoSymbol)
	assert.NoError(t, err)
	_, err = db.DB.Exec("INSERT INTO FiatCurrencies (symbol) VALUES (?)", fiatSymbol1)
	assert.NoError(t, err)
	_, err = db.DB.Exec("INSERT INTO FiatCurrencies (symbol) VALUES (?)", fiatSymbol2)
	assert.NoError(t, err)
	_, err = db.DB.Exec("INSERT INTO ExchangeRates (cryptocurrency_id, fiat_currency_id, rate, timestamp) VALUES (?,?,?,?)",
		cryptocurrencyID, fiatCurrencyID1, rate1, timestamp)
	assert.NoError(t, err)
	_, err = db.DB.Exec("INSERT INTO ExchangeRates (cryptocurrency_id, fiat_currency_id, rate, timestamp) VALUES (?,?,?,?)",
		cryptocurrencyID, fiatCurrencyID2, rate2, timestamp)
	assert.NoError(t, err)

	// Retrieve the exchange rates for the cryptocurrency
	rates, err := db.GetExchangeRatesForCrypto(cryptoSymbol)
	assert.NoError(t, err)

	// Check the expected result
	expectedRates := map[string]float64{
		fiatSymbol1: rate1,
		fiatSymbol2: rate2,
	}
	assert.Equal(t, expectedRates, rates)
}

func TestGetAllExchangeRates(t *testing.T) {
	db, err := NewDatabase()
	defer db.Close()
	assert.NoError(t, err)
	// Prepare the test data
	cryptoSymbol1 := "BTC"
	cryptoSymbol2 := "ETH"
	fiatSymbol1 := "USD"
	fiatSymbol2 := "INR"
	cryptocurrencyID1 := 1
	cryptocurrencyID2 := 2
	fiatCurrencyID1 := 1
	fiatCurrencyID2 := 2
	rate1 := 25.2
	rate2 := 30.5
	rate3 := 31.1
	rate4 := 26.4
	rate5 := 5484.6
	rate6 := 345.6
	rate7 := 86.6
	rate8 := 125.6
	timestamp := time.Now()

	// Insert the necessary data into the database
	_, err = db.DB.Exec("INSERT INTO Cryptocurrencies (symbol) VALUES (?)", cryptoSymbol1)
	assert.NoError(t, err)
	_, err = db.DB.Exec("INSERT INTO Cryptocurrencies (symbol) VALUES (?)", cryptoSymbol2)
	assert.NoError(t, err)
	_, err = db.DB.Exec("INSERT INTO FiatCurrencies (symbol) VALUES (?)", fiatSymbol1)
	assert.NoError(t, err)
	_, err = db.DB.Exec("INSERT INTO FiatCurrencies (symbol) VALUES (?)", fiatSymbol2)
	assert.NoError(t, err)
	_, err = db.DB.Exec("INSERT INTO ExchangeRates (cryptocurrency_id, fiat_currency_id, rate, timestamp) VALUES (?,?,?,?)",
		cryptocurrencyID1, fiatCurrencyID1, rate1, timestamp)
	assert.NoError(t, err)
	_, err = db.DB.Exec("INSERT INTO ExchangeRates (cryptocurrency_id, fiat_currency_id, rate, timestamp) VALUES (?,?,?,?)",
		cryptocurrencyID1, fiatCurrencyID2, rate2, timestamp)
	assert.NoError(t, err)
	_, err = db.DB.Exec("INSERT INTO ExchangeRates (cryptocurrency_id, fiat_currency_id, rate, timestamp) VALUES (?,?,?,?)",
		cryptocurrencyID2, fiatCurrencyID1, rate3, timestamp)
	assert.NoError(t, err)
	_, err = db.DB.Exec("INSERT INTO ExchangeRates (cryptocurrency_id, fiat_currency_id, rate, timestamp) VALUES (?,?,?,?)",
		cryptocurrencyID2, fiatCurrencyID2, rate4, timestamp)
	assert.NoError(t, err)
	_, err = db.DB.Exec("INSERT INTO ExchangeRates (cryptocurrency_id, fiat_currency_id, rate, timestamp) VALUES (?,?,?,?)",
		cryptocurrencyID1, fiatCurrencyID1, rate5, timestamp)
	assert.NoError(t, err)
	_, err = db.DB.Exec("INSERT INTO ExchangeRates (cryptocurrency_id, fiat_currency_id, rate, timestamp) VALUES (?,?,?,?)",
		cryptocurrencyID1, fiatCurrencyID2, rate6, timestamp)
	assert.NoError(t, err)
	_, err = db.DB.Exec("INSERT INTO ExchangeRates (cryptocurrency_id, fiat_currency_id, rate, timestamp) VALUES (?,?,?,?)",
		cryptocurrencyID2, fiatCurrencyID1, rate7, timestamp)
	assert.NoError(t, err)
	_, err = db.DB.Exec("INSERT INTO ExchangeRates (cryptocurrency_id, fiat_currency_id, rate, timestamp) VALUES (?,?,?,?)",
		cryptocurrencyID2, fiatCurrencyID2, rate8, timestamp)
	assert.NoError(t, err)

	// Retrieve all exchange rates
	rates, err := db.GetAllExchangeRates()
	assert.NoError(t, err)

	// Check the expected result
	expectedRates := map[string]map[string]float64{
		cryptoSymbol1: {
			fiatSymbol1: rate5,
			fiatSymbol2: rate6,
		},
		cryptoSymbol2: {
			fiatSymbol1: rate7,
			fiatSymbol2: rate8,
		},
	}
	assert.Equal(t, expectedRates, rates)
}

// TestGetHistoricalExchangeRates tests the retrieval of historical exchange rates.
func TestGetHistoricalExchangeRates(t *testing.T) {
	db, err := NewDatabase()
	defer db.Close()
	assert.NoError(t, err)
	// Prepare the test data
	cryptoSymbol := "BTC"
	fiatSymbol := "USD"
	rate1 := 25.2
	rate2 := 30.5
	rate3 := 530.5
	timestamp1 := time.Now().UTC().Add(-2 * time.Hour).Round(time.Second)  // 2 hours ago
	timestamp2 := time.Now().UTC().Add(-1 * time.Hour).Round(time.Second)  // 1 hour ago
	timestamp3 := time.Now().UTC().Add(-26 * time.Hour).Round(time.Second) // 26 hour ago

	// Insert the necessary data into the database
	cryptocurrencyID := 1
	fiatCurrencyID := 1

	_, err = db.DB.Exec("INSERT INTO Cryptocurrencies (symbol) VALUES (?)", cryptoSymbol)
	assert.NoError(t, err)
	_, err = db.DB.Exec("INSERT INTO FiatCurrencies (symbol) VALUES (?)", fiatSymbol)
	assert.NoError(t, err)
	_, err = db.DB.Exec("INSERT INTO ExchangeRates (cryptocurrency_id, fiat_currency_id, rate, timestamp) VALUES (?,?,?,?)",
		cryptocurrencyID, fiatCurrencyID, rate1, timestamp1)
	assert.NoError(t, err)
	_, err = db.DB.Exec("INSERT INTO ExchangeRates (cryptocurrency_id, fiat_currency_id, rate, timestamp) VALUES (?,?,?,?)",
		cryptocurrencyID, fiatCurrencyID, rate2, timestamp2)
	assert.NoError(t, err)
	_, err = db.DB.Exec("INSERT INTO ExchangeRates (cryptocurrency_id, fiat_currency_id, rate, timestamp) VALUES (?,?,?,?)",
		cryptocurrencyID, fiatCurrencyID, rate3, timestamp3)
	assert.NoError(t, err)

	// Retrieve historical exchange rates
	rates, err := db.GetHistoricalExchangeRates(cryptoSymbol, fiatSymbol)
	assert.NoError(t, err)

	// Check the expected result
	expectedRates := []CryptoResponseWithTimestamp{
		{
			Value:     rate1,
			Timestamp: timestamp1.UTC().Format("2006-01-02T15:04:05Z"),
		},
		{
			Value:     rate2,
			Timestamp: timestamp2.UTC().Format("2006-01-02T15:04:05Z"),
		},
	}
	assert.Equal(t, expectedRates, rates)
}

func TestMain(m *testing.M) {
	setup()
	code := m.Run()
	tearDown()
	os.Exit(code)
}
