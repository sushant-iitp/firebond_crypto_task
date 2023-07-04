package main

import (
	"database/sql"
	"fmt"
	"github.com/stretchr/testify/assert"
	"log"
	"os"
	"testing"
	"time"
)

var testDB *sql.DB
var db *Database

func TestMain(m *testing.M) {
	setup()
	code := m.Run()
	tearDown()
	os.Exit(code)
}

func setup() {
	// You need to set up a test database and connect to it
	testDB, err := sql.Open("mysql", "sushant:1234567890@tcp(localhost:3306)/testDB?parseTime=true")
	if err != nil {
		log.Fatalf("Error connecting to the test database: %v", err)
	}

	err = testDB.Ping()
	if err != nil {
		log.Fatalf("Error pinging the test database: %v", err)
	}

	// Create the test tables (Cryptocurrencies, FiatCurrencies, and ExchangeRates)
	_, err = testDB.Exec(`
    CREATE TABLE IF NOT EXISTS Cryptocurrencies (
        cryptocurrency_id INT PRIMARY KEY AUTO_INCREMENT,
        symbol VARCHAR(10) NOT NULL UNIQUE
    );
`)
	if err != nil {
		log.Fatalf("Error creating Cryptocurrencies table: %s", err)
	}

	_, err = testDB.Exec(`
    CREATE TABLE IF NOT EXISTS FiatCurrencies (
        fiat_currency_id INT PRIMARY KEY AUTO_INCREMENT,
        symbol VARCHAR(10) NOT NULL UNIQUE
    );
`)
	if err != nil {
		log.Fatalf("Error creating FiatCurrencies table: %s", err)
	}

	_, err = testDB.Exec(`
    CREATE TABLE IF NOT EXISTS ExchangeRates (
        exchange_rate_id INT PRIMARY KEY AUTO_INCREMENT,
        cryptocurrency_id INT,
        fiat_currency_id INT,
        rate DECIMAL(8, 2) NOT NULL,
        timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        FOREIGN KEY (cryptocurrency_id) REFERENCES Cryptocurrencies(cryptocurrency_id),
        FOREIGN KEY (fiat_currency_id) REFERENCES FiatCurrencies(fiat_currency_id)
    );
`)
	if err != nil {
		log.Fatalf("Error creating ExchangeRates table: %s", err)
	}

	db = &Database{DB: testDB}

}

func tearDown() {
	if testDB != nil {
		testDB.Close()
	}
}

func TestCheckCryptoCurrency(t *testing.T) {
	// Add some test data to the Cryptocurrencies table
	_, err := testDB.Exec("INSERT INTO Cryptocurrencies (symbol) VALUES ('BTC'), ('ETH'), ('USDT'), ('BNB')")
	if err != nil {
		t.Fatalf("Error adding test data: %v", err)
	}

	// Test cases
	testCases := []struct {
		crypto   string
		expected bool
	}{
		{"BTC", true},
		{"ETH", true},
		{"LTC", false},
	}

	for _, tc := range testCases {
		exists, err := db.CheckCryptoCurrency(tc.crypto)
		assert.NoError(t, err, "Error checking crypto currency")
		assert.Equal(t, tc.expected, exists, fmt.Sprintf("Unexpected result for %s", tc.crypto))
	}
}

func TestCheckFiatCurrency(t *testing.T) {
	// Add some test data to the FiatCurrencies table
	_, err := testDB.Exec("INSERT INTO FiatCurrencies (symbol) VALUES ('USD'), ('EUR'), ('JPY'), ('GBP')")
	if err != nil {
		t.Fatalf("Error adding test data: %v", err)
	}

	// Test cases
	testCases := []struct {
		fiat     string
		expected bool
	}{
		{"USD", true},
		{"EUR", true},
		{"CNY", false},
	}

	for _, tc := range testCases {
		exists, err := db.CheckFiatCurrency(tc.fiat)
		assert.NoError(t, err, "Error checking fiat currency")
		assert.Equal(t, tc.expected, exists, fmt.Sprintf("Unexpected result for %s", tc.fiat))
	}
}

func TestGetExchangeRate(t *testing.T) {
	// Add some test data to the ExchangeRates table
	_, err := testDB.Exec(`
		INSERT INTO Cryptocurrencies (symbol) VALUES ('BTC'), ('ETH');
		INSERT INTO FiatCurrencies (symbol) VALUES ('USD'), ('EUR');
		INSERT INTO ExchangeRates (cryptocurrency_id, fiat_currency_id, rate, timestamp)
		VALUES
			(1, 1, 35000.00, '2021-01-01 00:00:00'),
			(1, 2, 30000.00, '2021-01-01 00:00:00'),
			(2, 1, 2000.00, '2021-01-01 00:00:00'),
			(2, 2, 1800.00, '2021-01-01 00:00:00');
	`)
	if err != nil {
		t.Fatalf("Error adding test data: %v", err)
	}

	// Test cases
	testCases := []struct {
		crypto   string
		fiat     string
		date     time.Time
		expected float64
	}{
		{"BTC", "USD", time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC), 35000.00},
		{"BTC", "EUR", time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC), 30000.00},
		{"ETH", "USD", time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC), 2000.00},
		{"ETH", "EUR", time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC), 1800.00},
	}

	for _, tc := range testCases {
		rate, err := db.GetExchangeRate(tc.crypto, tc.fiat)
		assert.NoError(t, err, "Error getting exchange rate")
		assert.Equal(t, tc.expected, rate, fmt.Sprintf("Unexpected result for %s to %s ", tc.crypto, tc.fiat))
	}
}
