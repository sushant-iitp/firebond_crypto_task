package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"regexp"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

func isValidAddress(address string) bool {
	// Use a regular expression to check if the address matches the Ethereum address format
	// This is a simple check and may not catch all invalid addresses, but it covers most cases
	match, _ := regexp.MatchString("^0x[0-9a-fA-F]{40}$", address)
	return match
}

func GetBalanceHandler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Get the Ethereum address from the path parameter
	address := request.PathParameters["address"]

	// Check if the address is valid
	if !isValidAddress(address) {
		response := struct {
			Message string `json:"message"`
		}{
			Message: "Address not valid",
		}

		jsonResponse, _ := json.Marshal(response)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       string(jsonResponse),
		}, nil
	}

	// Create an Ethereum client connection
	client, err := ethclient.Dial("https://mainnet.infura.io/v3/ca40b363703a4b3a8fed56a7eedd774a")
	if err != nil {
		log.Println("Failed to connect to the Ethereum client:", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, err
	}

	// Convert the address string to an Ethereum address type
	addr := common.HexToAddress(address)

	// Get the current balance of the address
	balance, err := client.BalanceAt(context.Background(), addr, nil)
	if err != nil {
		log.Println("Failed to retrieve balance:", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, err
	}

	// Prepare the response payload
	response := struct {
		Address string `json:"address"`
		Balance string `json:"balance"`
	}{
		Address: address,
		Balance: balance.String(),
	}

	// Encode the response payload as JSON
	jsonResponse, err := json.Marshal(response)
	if err != nil {
		log.Println("Failed to encode response:", err)
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, err
	}

	// Return the response
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       string(jsonResponse),
	}, nil
}

func main() {
	lambda.Start(GetBalanceHandler)
}
