package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

type CryptoResponse map[string]float64

func GetSingleRate(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	pathParams := strings.Split(request.Path, "/")
	crypto := pathParams[len(pathParams)-2]
	fiat := pathParams[len(pathParams)-1]

	fmt.Println("Crypto:", crypto)
	fmt.Println("Fiat:", fiat)

	apiURL := fmt.Sprintf("https://min-api.cryptocompare.com/data/price?fsym=%s&tsyms=%s", crypto, fiat)

	resp, err := http.Get(apiURL)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusBadGateway}, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	var data map[string]float64
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, err
	}

	value, ok := data[fiat]
	if !ok {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusNotFound}, fmt.Errorf("Exchange rate not found for %s", fiat)
	}

	response := CryptoResponse{
		fiat: value,
	}

	body, err := json.Marshal(response)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, err
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       string(body),
	}, nil
}

func main() {
	lambda.Start(GetSingleRate)
}
