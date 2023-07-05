# CryptoData Service

CryptoData is a service built using Go and deployed on Netlify as a serverless function with a MySQL database. It provides exchange rate data for cryptocurrencies and fiat currencies through various API endpoints. The service includes the following five API endpoints:

1. `/rates/{crypto}/{fiat}`: Fetches the exchange rate of a given cryptocurrency to a given fiat currency.
2. `/rates/{crypto}`: Fetches the exchange rate of all supported fiat currencies for a given cryptocurrency.
3. `/rates`: Fetches the exchange rate of all supported cryptocurrencies with all supported fiat currencies.
4. `/rates/history/{crypto}/{fiat}`: Fetches the exchange rate data of the past 24 hours for a given cryptocurrency to a given fiat currency.
5. `/balance/{address}`: Fetches the current balance of a specific Ethereum address.

## Accessing the Service

The CryptoData service can be accessed using the following URLs:

1. `https://main--euphonious-brioche-40b22d.netlify.app/.netlify/functions/rates/{crypto}/{fiat}`
2. `https://main--euphonious-brioche-40b22d.netlify.app/.netlify/functions/rates/{crypto}`
3. `https://main--euphonious-brioche-40b22d.netlify.app/.netlify/functions/rates/`
4. `https://main--euphonious-brioche-40b22d.netlify.app/.netlify/functions/rates/history/{crypto}/{fiat}`
5. `https://main--euphonious-brioche-40b22d.netlify.app/.netlify/functions/balance/{address}`

Example URL: `https://main--euphonious-brioche-40b22d.netlify.app/.netlify/functions/rates/BTC/USD`

Make a GET request to the appropriate URL to retrieve the desired information.

## Data Storage and Updation

The CryptoData service uses a MySQL database to store exchange rate data. The database schema includes three tables:

- Cryptocurrencies: Stores information about cryptocurrencies.
- FiatCurrencies: Stores information about fiat currencies.
- ExchangeRates: Stores exchange rate data with timestamps.

The database schema is designed as follows:

-- Table: Cryptocurrencies
CREATE TABLE Cryptocurrencies (
  cryptocurrency_id INT AUTO_INCREMENT PRIMARY KEY,
  symbol VARCHAR(10)
);

-- Table: FiatCurrencies
CREATE TABLE FiatCurrencies (
  fiat_currency_id INT AUTO_INCREMENT PRIMARY KEY,
  symbol VARCHAR(10)
);

-- Table: ExchangeRates
CREATE TABLE ExchangeRates (
  exchange_rate_id INT AUTO_INCREMENT PRIMARY KEY,
  cryptocurrency_id INT,
  fiat_currency_id INT,
  rate DECIMAL(18, 8),
  timestamp TIMESTAMP,
  FOREIGN KEY (cryptocurrency_id) REFERENCES Cryptocurrencies(cryptocurrency_id),
  FOREIGN KEY (fiat_currency_id) REFERENCES FiatCurrencies(fiat_currency_id)
);

To keep the exchange rate data updated, a cron job is used to schedule functions that fetch data from the CryptoCompare API and store it in the ExchangeRates table every 10 minutes.

Supported Cryptocurrencies for the current service are: BTC, ETH, USDT, BNB, USDC, XRP, ADA, DOGE, LTC, SOL.

Supported Fiat Currencies for the current service are: CNY, USD, EUR, JPY, GBP, KRW, INR, CAD, HKD, BRL.

## Ethereum Balance API

This function provides an API endpoint to retrieve the current balance of a specific Ethereum address. 
The functionality is exposed through the endpoint GET /balance/{address}.
Balance is retrieved using Infura which provides Ethereum node infrastructure that allows us to interact with the Ethereum network
Limitations:
- The code uses a simple regular expression to check the validity of Ethereum addresses but may not catch all invalid addresses.
- This implementation only retrieves the current balance of an Ethereum address and does not interact with smart contracts.


## Testing

To test the APIs of the CryptoData service, you can use the POSTMAN Collection provided in the repository. Import the collection into POSTMAN and execute the requests to validate the functionality and responses of the service.

## Local Setup

The CryptoData service can also be deployed locally using the files present in the `cryptolocal` folder. Follow these steps to set up the service locally:

1. Set up MySQL and initialize your database and tables according to the schema provided above.
2. Populate the data on the schema using the Python scripts provided in the sequence: `cryptolist.py`, `fiatlist.py`, `filldata.py`.
   Make sure to fill in your local database credentials in the respective files required for the database connection.
   The `filldata.py` script will update the database every 10 minutes with new values.
   Currently, the service uses an API call that can fetch data for around 60 cryptocurrencies with around 20 fiat currencies in a single call.
   You can increase the supported currencies by modifying the API calls and the flow of `filldata.py` to fetch data from a different API accordingly.
3. After setting up the database, you can run the local service present in the `main.go` file.
   Remember to fill in your local database credentials in the `main.go` file required for the database connection.
4. The service can be accessed at the following URL: `http://localhost:8080`, with the various endpoints:
   - `http://localhost:8080/rates`
   - `http://localhost:8080/rates/{crypto}`
   - `http://localhost:8080/rates/{crypto}/{fiat}`
   - `http://localhost:8080/rates/history/{crypto}/{fiat}`
   
   Example URL: `http://localhost:8080/rates/BTC/USD`

## Unit Testing

Unit testing of functions can be executed from the `unit_test.go` file after filling in the mock database credentials in the `setup()` and `tearDown()` functions.
