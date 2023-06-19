package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
)

type Counter struct {
	Counts map[int]int
	sync.Mutex
}

type ConnectionManager struct {
	connections map[*websocket.Conn]bool
}

var counter Counter
var db *sql.DB
var manager = ConnectionManager{
	connections: make(map[*websocket.Conn]bool),
}

func main() {
	// Initialize the counter and the database connection
	counter = Counter{}
	db = initDB()
	getAllCountsFromDB()

	http.HandleFunc("/", helloHandler)
	http.HandleFunc("/socket", socketHandler)
	http.HandleFunc("/count", getCountHandler)
	// http.HandleFunc("/increment", incrementCountHandler)

	log.Println("Starting server on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func loadEnvVariables() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file:", err)
	}
}

func getAllCountsFromDB() error {
	rows, err := db.Query("SELECT id_target, count_currently FROM count")
	if err != nil {
		return err
	}
	defer rows.Close()

	counter.Lock()
	counter.Counts = make(map[int]int)

	for rows.Next() {
		var idTarget, countCurrently int
		err := rows.Scan(&idTarget, &countCurrently)
		if err != nil {
			return err
		}

		counter.Counts[idTarget] = countCurrently
	}

	counter.Unlock()

	return nil
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
	// Add the connection to the manager
	manager.AddConnection(conn)
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
			// Assuming the message format is "increment:idTarget"
			if strings.HasPrefix(message, "increment:") {
				idTargetStr := strings.TrimPrefix(message, "increment:")
				idTarget, err := strconv.Atoi(idTargetStr)
				if err != nil {
					// Handle error if the idTarget is not a valid integer
				}
				// Use the idTarget variable in your code
				incrementCount(conn, idTarget)
			}
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

// AddConnection adds a new WebSocket connection to the manager
func (m *ConnectionManager) AddConnection(conn *websocket.Conn) {
	m.connections[conn] = true
}

// RemoveConnection removes a WebSocket connection from the manager
func (m *ConnectionManager) RemoveConnection(conn *websocket.Conn) {
	delete(m.connections, conn)
}

// Broadcast sends the specified JSON payload to all WebSocket connections
func (m *ConnectionManager) Broadcast(jsonPayload []byte) {
	// jsonPayload, err := json.Marshal(payload)
	// if err != nil {
	// 	log.Println("Error marshaling JSON payload:", err)
	// 	return
	// }

	for conn := range m.connections {
		err := conn.WriteMessage(websocket.TextMessage, jsonPayload)
		if err != nil {
			log.Println("Error sending message:", err)
			continue
		}
		log.Println("Sent message to connection:", conn)
	}
}

func incrementCount(conn *websocket.Conn, idTarget int) {
	counter.Lock()
	defer counter.Unlock()

	// Check if the count map has an entry for the given idTarget
	if counter.Counts == nil {
		counter.Counts = make(map[int]int)
	}

	// Increment the count for the given idTarget
	counter.Counts[idTarget]++
	count := counter.Counts[idTarget]

	log.Println("Count incremented for idTarget", idTarget, ":", count)

	// Update the count in the database
	err := updateCountInDB(idTarget, count)
	if err != nil {
		log.Println("Failed to update count in the database:", err)
	}

	// Create a JSON payload with the current counter data
	payload := []map[string]interface{}{}

	for id, count := range counter.Counts {
		data := map[string]interface{}{
			"count": count,
			"id":    id,
		}
		payload = append(payload, data)
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		log.Println("Failed to marshal JSON payload:", err)
		return
	}
	// Call the Broadcast method of the ConnectionManager to send the payload to all connections
	manager.Broadcast(jsonPayload)
	// Emit the JSON payload to the socket connection
	// err = conn.WriteMessage(websocket.TextMessage, jsonPayload)
	// if err != nil {
	// 	log.Println("Failed to emit message to socket connection:", err)
	// 	return
	// }
}

func getCountHandler(w http.ResponseWriter, r *http.Request) {
	counter.Lock()
	defer counter.Unlock()

	// count := counter.Count
	count := 1

	// Return the count as a JSON response
	json.NewEncoder(w).Encode(map[string]int{"count": count})
}

// func incrementCountHandler(w http.ResponseWriter, r *http.Request) {
// 	counter.Lock()
// 	defer counter.Unlock()

// 	counter.Count++
// 	log.Println("Count incremented:", counter.Count)

// 	// Update the count in the database
// 	err := updateCountInDB(counter.Count)
// 	if err != nil {
// 		log.Println("Failed to update count in the database:", err)
// 	}

// 	// Return the updated count as a JSON response
// 	json.NewEncoder(w).Encode(map[string]int{"count": counter.Count})
// }

func initDB() *sql.DB {
	// Load environment variables from .env file
	loadEnvVariables()

	// Retrieve the database URI from the environment variable
	dbURI := os.Getenv("DATABASE_URI")

	// Open the database connection
	db, err := sql.Open("mysql", dbURI)
	if err != nil {
		log.Fatal("Failed to connect to the database:", err)
	}

	// Create the targets table
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS targets (
		id INT AUTO_INCREMENT PRIMARY KEY,
		link_image TEXT,
		name TEXT
	)`)
	if err != nil {
		log.Fatal("Failed to create targets table:", err)
	}

	// Create the count table
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS count (
		id INT AUTO_INCREMENT PRIMARY KEY,
		id_target INT,
		count_currently INT,
		FOREIGN KEY (id_target) REFERENCES targets (id)
	)`)
	if err != nil {
		log.Fatal("Failed to create count table:", err)
	}

	// Retrieve the count from the database
	// err = db.QueryRow("SELECT value FROM count ORDER BY id DESC LIMIT 1").Scan(&counter.Count)
	// if err != nil && err != sql.ErrNoRows {
	// 	log.Fatal("Failed to retrieve count from the database:", err)
	// }
	return db
}

func updateCountInDB(idTarget, count int) error {
	_, err := db.Exec("UPDATE count SET count_currently=? WHERE id_target=?", count, idTarget)
	if err != nil {
		return err
	}
	return nil
}
