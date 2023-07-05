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
            query = f"INSERT INTO FiatCurrencies (symbol) VALUES ('{symbol}')"
            cursor.execute(query)
        
        conn.commit()
        print("Crypto/Fiat currencies inserted successfully.")
    except Exception as e:
        print("Error occurred while inserting fiat currencies:", str(e))
    finally:
        if conn.is_connected():
            cursor.close()
            conn.close()


# Top 20 Fiat currencies
fiat_currencies = [
    {"symbol": "USD"},
{"symbol": "EUR"},
{"symbol": "JPY"},
{"symbol": "GBP"},
{"symbol": "CAD"},
{"symbol": "CNY"},
{"symbol": "HKD"},
{"symbol": "KRW"},
{"symbol": "INR"},
{"symbol": "BRL"},

]

# Call the function to insert Fiat currencies into the database
insert_fiat_currencies(fiat_currencies)
