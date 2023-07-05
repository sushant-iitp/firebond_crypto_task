package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

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

//Insert your DB credentials(User,Password,Host & database) here.
func NewDatabase() (*Database, error) {
	dbUser := ""
	dbPassword := ""
	dbHost := ""
	dbDatabase := ""

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
	    AND er.timestamp = (
		SELECT MAX(timestamp)
		FROM ExchangeRates
		WHERE cryptocurrency_id = er.cryptocurrency_id
		AND fiat_currency_id = er.fiat_currency_id)
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
		AND er.timestamp = (
		SELECT MAX(timestamp)
		FROM ExchangeRates
		WHERE cryptocurrency_id = er.cryptocurrency_id
	)
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
	FROM (
		SELECT cryptocurrency_id, fiat_currency_id, MAX(timestamp) AS max_timestamp
		FROM ExchangeRates
		GROUP BY cryptocurrency_id, fiat_currency_id
	) AS latest
	JOIN ExchangeRates er ON er.cryptocurrency_id = latest.cryptocurrency_id
		AND er.fiat_currency_id = latest.fiat_currency_id
		AND er.timestamp = latest.max_timestamp
	JOIN Cryptocurrencies c ON c.cryptocurrency_id = er.cryptocurrency_id
	JOIN FiatCurrencies f ON f.fiat_currency_id = er.fiat_currency_id;
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

func handleTooManyInvalidParameters(w http.ResponseWriter, r *http.Request) {
	errorMessage := "Too many parameters. Please try again with valid parameters.\n\nValid URL formats:\n1. http://localhost:8080/rates\n2. http://localhost:8080/rates/{crypto}\n3. http://localhost:8080/rates/{crypto}/{fiat}\n4. http://localhost:8080/rates/history/{crypto}/{fiat} "
	w.WriteHeader(http.StatusBadRequest)
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(errorMessage))
}

func handleInvalidParameters(w http.ResponseWriter, r *http.Request) {
	errorMessage := "Invalid parameters. Please try again with valid parameters.\n\nValid URL formats:\n1. http://localhost:8080/rates\n2. http://localhost:8080/rates/{crypto}\n3. http://localhost:8080/rates/{crypto}/{fiat}\n4. http://localhost:8080/rates/history/{crypto}/{fiat} "

	w.WriteHeader(http.StatusBadRequest)
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(errorMessage))
}

func handleInvalidCryptoCurrency(w http.ResponseWriter, r *http.Request) {
	errorMessage := "Crypto currency does not exist or is not servicable. \nPlease try again with valid parameters.\n\nValid URL formats:\n1. http://localhost:8080/rates\n2. http://localhost:8080/rates/{crypto}\n3. http://localhost:8080/rates/{crypto}/{fiat}\n4. http://localhost:8080/rates/history/{crypto}/{fiat} "

	w.WriteHeader(http.StatusNotFound)
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(errorMessage))
}

func handleInvalidFiatCurrency(w http.ResponseWriter, r *http.Request) {
	errorMessage := "Fiat currency does not exist or is not servicable. \nPlease try again with valid parameters.\n\nValid URL formats:\n1. http://localhost:8080/rates\n2. http://localhost:8080/rates/{crypto}\n3. http://localhost:8080/rates/{crypto}/{fiat}\n4. http://localhost:8080/rates/history/{crypto}/{fiat} "

	w.WriteHeader(http.StatusNotFound)
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(errorMessage))
}

func handleExchangeRateNotFound(w http.ResponseWriter, r *http.Request) {
	errorMessage := "Exchange rates not found."

	w.WriteHeader(http.StatusNotFound)
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(errorMessage))
}

func handleGetExchangeRate(w http.ResponseWriter, r *http.Request, splitPath []string) {
	crypto := splitPath[2]
	fiat := splitPath[3]

	db, err := NewDatabase()
	if err != nil {
		log.Println("Error connecting to the database:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer db.Close()
	cryptoExists, err := db.CheckCryptoCurrency(crypto)
	if err != nil {
		log.Println("Error checking if crypto currency exists:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !cryptoExists {
		handleInvalidCryptoCurrency(w, r)
		return
	}
	fiatExists, err := db.CheckFiatCurrency(fiat)
	if err != nil {
		log.Println("Error checking if fiat currency exists:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !fiatExists {
		handleInvalidFiatCurrency(w, r)
		return
	}

	rate, err := db.GetExchangeRate(crypto, fiat)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			handleExchangeRateNotFound(w, r)
			return
		}
		log.Println("Error retrieving exchange rate:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	response := CryptoResponse{Value: rate}
	responseBody, _ := json.Marshal(response)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(responseBody)
}

func handleGetExchangeRatesForCrypto(w http.ResponseWriter, r *http.Request, splitPath []string) {
	crypto := splitPath[2]

	db, err := NewDatabase()
	if err != nil {
		log.Println("Error connecting to the database:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer db.Close()
	cryptoExists, err := db.CheckCryptoCurrency(crypto)
	if err != nil {
		log.Println("Error checking if crypto currency exists:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !cryptoExists {
		handleInvalidCryptoCurrency(w, r)
		return
	}

	rates, err := db.GetExchangeRatesForCrypto(crypto)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			handleExchangeRateNotFound(w, r)
			return
		}
		log.Println("Error retrieving exchange rates:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	response := make(map[string]float64)
	for fiat, rate := range rates {
		response[fiat] = rate
	}

	responseBody, _ := json.Marshal(response)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(responseBody)
}

func handleGetHistoricalExchangeRates(w http.ResponseWriter, r *http.Request, splitPath []string) {
	crypto := splitPath[3]
	fiat := splitPath[4]

	db, err := NewDatabase()
	if err != nil {
		log.Println("Error connecting to the database:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer db.Close()
	cryptoExists, err := db.CheckCryptoCurrency(crypto)
	if err != nil {
		log.Println("Error checking if crypto currency exists:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !cryptoExists {
		handleInvalidCryptoCurrency(w, r)
		return
	}
	fiatExists, err := db.CheckFiatCurrency(fiat)
	if err != nil {
		log.Println("Error checking if fiat currency exists:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !fiatExists {
		handleInvalidFiatCurrency(w, r)
		return
	}

	rates, err := db.GetHistoricalExchangeRates(crypto, fiat)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			handleExchangeRateNotFound(w, r)
			return
		}
		log.Println("Error retrieving historical exchange rates:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	response := HistoricalRateResponse{ExchangeRate: rates}
	responseBody, _ := json.Marshal(response)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(responseBody)
}

func handleGetAllExchangeRates(w http.ResponseWriter, r *http.Request) {
	db, err := NewDatabase()
	if err != nil {
		log.Println("Error connecting to the database:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer db.Close()

	rates, err := db.GetAllExchangeRates()
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			handleExchangeRateNotFound(w, r)
			return
		}
		log.Println("Error retrieving exchange rates:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	responseBody, _ := json.Marshal(rates)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(responseBody)
}

func HandleRequest(w http.ResponseWriter, r *http.Request) {
	splitPath := strings.Split(r.URL.Path, "/")
	numParams := len(splitPath)

	if numParams > 5 {
		handleTooManyInvalidParameters(w, r)
	} else if numParams == 4 && splitPath[1] == "rates" {
		handleGetExchangeRate(w, r, splitPath)
	} else if numParams == 3 && splitPath[2] != "" && splitPath[1] == "rates" {
		handleGetExchangeRatesForCrypto(w, r, splitPath)
	} else if numParams == 5 && splitPath[2] == "history" && splitPath[3] != "" && splitPath[4] != "" && splitPath[1] == "rates" {
		handleGetHistoricalExchangeRates(w, r, splitPath)
	} else if numParams == 2 && splitPath[1] == "rates" {
		handleGetAllExchangeRates(w, r)
	} else {
		handleInvalidParameters(w, r)
	}

}

func main() {
	http.HandleFunc("/", HandleRequest)

	// Set the server port
	port := ":8080"

	// Start the server
	log.Printf("Server listening on port %s", port)
	log.Fatal(http.ListenAndServe(port, nil))
}
