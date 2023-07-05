import mysql.connector

def insert_fiat_currencies(currencies):
    try:
        conn = mysql.connector.connect(
            password = "",
    database = "",
    host = "",
    port = ,
    user = ""
        )
        cursor = conn.cursor()

        for currency in currencies:
            symbol = currency.get("symbol", "")
            query = f"INSERT INTO Cryptocurrencies (symbol) VALUES ('{symbol}')"
            cursor.execute(query)
        
        conn.commit()
        print("Crypto currencies inserted successfully.")
    except Exception as e:
        print("Error occurred while inserting fiat currencies:", str(e))
    finally:
        if conn.is_connected():
            cursor.close()
            conn.close()


# Top 20 Fiat currencies
fiat_currencies = [
    {"symbol": "BTC"},
{"symbol": "ETH"},
{"symbol": "USDT"},
{"symbol": "DOGE"},
{"symbol": "BNB"},
{"symbol": "USDC"},
{"symbol": "XRP"},
{"symbol": "ADA"},
{"symbol": "LTC"},
{"symbol": "SOL"},
]

# Call the function to insert Fiat currencies into the database
insert_fiat_currencies(fiat_currencies)
