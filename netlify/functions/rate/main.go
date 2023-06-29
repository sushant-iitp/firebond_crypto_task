package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

type CryptoInfo struct {
	Name string  `json:"Name"`
	Rate float64 `json:"Rate"`
}

func rateHandler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Extract the cryptocurrency and fiat parameters from the request path parameters
	pathParameters := request.PathParameters
	crypto := strings.ToUpper(pathParameters["crypto"])
	fiat := strings.ToUpper(pathParameters["fiat"])

	// Connect to the MySQL database
	db, err := sql.Open("mysql", os.Getenv("DB_USER")+":"+os.Getenv("DB_PASSWORD")+"@tcp("+os.Getenv("DB_HOST")+":"+os.Getenv("DB_PORT")+")/"+os.Getenv("DB_DATABASE"))
	if err != nil {
		return events.APIGatewayProxyResponse{}, fmt.Errorf("Error connecting to the database: %v", err)
	}
	defer db.Close()

	// Prepare the SQL query with placeholders
	query := "SELECT name, rate FROM cryptoinfo WHERE name = ? AND fiat = ? ORDER BY id DESC LIMIT 1"

	// Execute the parameterized query with user input
	row := db.QueryRow(query, crypto, fiat)

	var cryptoInfo CryptoInfo
	err = row.Scan(&cryptoInfo.Name, &cryptoInfo.Rate)
	if err != nil {
		if err == sql.ErrNoRows {
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusNotFound,
				Body:       "No data found for the specified cryptocurrency and fiat",
			}, nil
		}
		return events.APIGatewayProxyResponse{}, fmt.Errorf("Error retrieving data from the database: %v", err)
	}

	// Convert the response to JSON
	responseBody, err := json.Marshal(cryptoInfo)
	if err != nil {
		return events.APIGatewayProxyResponse{}, fmt.Errorf("Error marshaling JSON response: %v", err)
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       string(responseBody),
	}, nil
}

func main() {
	lambda.Start(rateHandler)
}
