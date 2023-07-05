import json
import requests
import time
import mysql.connector

# Database connection details
db_host = ""
db_user = ""
db_password = ""
db_name = ""

# API base URL
api_base_url = "https://min-api.cryptocompare.com/data/pricemulti"

# Function to fetch symbol-id mappings from the database
def fetch_symbol_id_mapping(table_name, symbol_column, id_column):
    # Establish database connection
    connection = mysql.connector.connect(
        host=db_host, user=db_user, password=db_password, database=db_name
    )

    try:
        with connection.cursor() as cursor:
            # Query to fetch symbol-id mappings from the specified table
            query = f"SELECT {symbol_column}, {id_column} FROM {table_name}"
            cursor.execute(query)
            results = cursor.fetchall()
            symbol_id_mapping = {row[0]: row[1] for row in results}
            return symbol_id_mapping
    finally:
        # Close database connection
        connection.close()

# Function to insert exchange rates into the database in bulk
def insert_exchange_rates(exchange_rates):
    # Establish database connection
    connection = mysql.connector.connect(
        host=db_host, user=db_user, password=db_password, database=db_name
    )

    try:
        with connection.cursor() as cursor:
            # SQL statement for bulk insertion of exchange rates
            query = "INSERT INTO ExchangeRates (cryptocurrency_id, fiat_currency_id, rate, timestamp) VALUES (%s, %s, %s, NOW())"

            # Convert exchange rates to a list of tuples
            values = [
                (cryptocurrency_id, fiat_currency_id, rate)
                for (cryptocurrency_id, fiat_currency_id), rate in exchange_rates.items()
            ]

            # Bulk insert the exchange rates
            cursor.executemany(query, values)

        # Commit the transaction
        connection.commit()
    finally:
        # Close database connection
        connection.close()

# Main function
def main():
    while True:
        try:
            # Fetch symbol-id mappings for cryptocurrencies and fiat currencies
            crypto_symbol_id_mapping = fetch_symbol_id_mapping(
                "Cryptocurrencies", "symbol", "cryptocurrency_id"
            )
            fiat_symbol_id_mapping = fetch_symbol_id_mapping(
                "FiatCurrencies", "symbol", "fiat_currency_id"
            )

            # Construct API call URL with symbols
            crypto_symbols = ",".join(crypto_symbol_id_mapping.keys())
            fiat_symbols = ",".join(fiat_symbol_id_mapping.keys())
            api_url = f"{api_base_url}?fsyms={crypto_symbols}&tsyms={fiat_symbols}"

            # Make API call to fetch exchange rates
            response = requests.get(api_url)
            if response.status_code == 200:
                data = response.json()

                # Prepare exchange rates for bulk insertion
                exchange_rates = {}

                for cryptocurrency, rates in data.items():
                    cryptocurrency_id = crypto_symbol_id_mapping.get(cryptocurrency)
                    if cryptocurrency_id is not None:
                        for fiat_currency, rate in rates.items():
                            fiat_currency_id = fiat_symbol_id_mapping.get(fiat_currency)
                            if fiat_currency_id is not None:
                                key = (cryptocurrency_id, fiat_currency_id)
                                exchange_rates[key] = rate

                # Bulk insert the exchange rates into the database
                insert_exchange_rates(exchange_rates)
            else:
                print("API call failed with status code:", response.status_code)
        except Exception as e:
            print("Error occurred:", str(e))

        # Wait for 1 minute before next execution
        time.sleep(600)


# Run the main function
if __name__ == "__main__":
    main()
