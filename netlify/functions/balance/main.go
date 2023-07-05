package main

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"math/big"
	"net/http"
	"os"
	"regexp"
	"strings"

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

// Helper function to convert balance from Wei to Ether
func weiToEther(balance *big.Int) float64 {
	etherValue := new(big.Float)
	etherValue.SetString(balance.String())
	etherValue = etherValue.Quo(etherValue, big.NewFloat(math.Pow10(18)))
	ether, _ := etherValue.Float64()
	return ether
}

func GetBalanceHandler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Get the Ethereum address from the path parameter
	address := strings.TrimPrefix(request.Path, "/.netlify/functions/balance/")
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
	infuraURL := os.Getenv("INFURA_URL")
	if infuraURL == "" {
		log.Fatal("INFURA_URL environment variable is not set")
	}
	client, err := ethclient.Dial(infuraURL)
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
		Address string  `json:"address"`
		Balance float64 `json:"balance"`
	}{
		Address: address,
		Balance: weiToEther(balance),
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
