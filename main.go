package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func adsf() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}
}

type Counter struct {
	Count int
	sync.Mutex
}

var counter Counter
var db *sql.DB

func main() {
	// Initialize the counter and the database connection
	counter = Counter{}
	db = initDB()

	http.HandleFunc("/", helloHandler)
	http.HandleFunc("/socket", socketHandler)
	http.HandleFunc("/count", getCountHandler)
	http.HandleFunc("/increment", incrementCountHandler)

	log.Println("Starting server on port 8080...")
	log.Fatal(http.ListenAndServe(os.Getenv("PORT"), nil))
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Hello, World!")
}

func socketHandler(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Failed to upgrade to WebSocket:", err)
		return
	}

	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			log.Println("WebSocket read error:", err)
			break
		}

		switch messageType {
		case websocket.TextMessage:
			message := string(p)
			fmt.Println("Received message:", message)
			// Add your own logic to process the message

		case websocket.BinaryMessage:
			// Handle binary message
			// Add your own logic to process the message

		case websocket.CloseMessage:
			err = conn.Close()
			if err != nil {
				log.Println("WebSocket close error:", err)
			}
			break
		}
	}
}

func getCountHandler(w http.ResponseWriter, r *http.Request) {
	counter.Lock()
	defer counter.Unlock()

	count := counter.Count

	// Return the count as a JSON response
	json.NewEncoder(w).Encode(map[string]int{"count": count})
}

func incrementCountHandler(w http.ResponseWriter, r *http.Request) {
	counter.Lock()
	defer counter.Unlock()

	counter.Count++
	log.Println("Count incremented:", counter.Count)

	// Update the count in the database
	err := updateCountInDB(counter.Count)
	if err != nil {
		log.Println("Failed to update count in the database:", err)
	}

	// Return the updated count as a JSON response
	json.NewEncoder(w).Encode(map[string]int{"count": counter.Count})
}

func initDB() *sql.DB {
	db, err := sql.Open("postgres", "postgres:postgres//postgres:postgres@localhost:5432/clickers?sslmode=disable")
	if err != nil {
		log.Fatal("Failed to connect to the database:", err)
	}

	// Create the count table if it doesn't exist
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS count (
		id SERIAL PRIMARY KEY,
		value INT NOT NULL
	)`)
	if err != nil {
		log.Fatal("Failed to create count table:", err)
	}

	// Retrieve the count from the database
	err = db.QueryRow("SELECT value FROM count ORDER BY id DESC LIMIT 1").Scan(&counter.Count)
	if err != nil && err != sql.ErrNoRows {
		log.Fatal("Failed to retrieve count from the database:", err)
	}
	return db
}

func updateCountInDB(count int) error {
	_, err := db.Exec("INSERT INTO count (value) VALUES ($1)", count)
	if err != nil {
		return err
	}
	return nil
}
