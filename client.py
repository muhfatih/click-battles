import websocket

def on_message(ws, message):
    print("Received message:", message)

def on_error(ws, error):
    print("WebSocket error:", error)

def on_close(ws):
    print("WebSocket connection closed")

def on_open(ws):
    def send_message():
        while True:
            message = input("Enter message (or 'q' to quit): ")
            if message == "q":
                break
            ws.send(message)

    print("WebSocket connection opened")
    send_message()

if __name__ == "__main__":
    ws = websocket.WebSocketApp("ws://localhost:8080/socket",
                                on_open=on_open,
                                on_message=on_message,
                                on_error=on_error,
                                on_close=on_close)
    ws.run_forever()
