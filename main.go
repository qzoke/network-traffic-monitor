package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/shirou/gopsutil/net"
	_ "modernc.org/sqlite"
)

// DataUsage represents a single day's network usage data
type DataUsage struct {
	Date          string `json:"date"`
	Sent          uint64 `json:"sent"`
	Received      uint64 `json:"received"`
	SentHuman     string `json:"sentHuman"`
	ReceivedHuman string `json:"receivedHuman"`
}

var db *sql.DB
var lastSent, lastReceived uint64
var lastCheckTime time.Time

// main is the entry point of the application. It initializes the database connection,
// sets up HTTP routes, and starts the web server.
func main() {
	var err error
	// Open SQLite database
	db, err = sql.Open("sqlite", "./network_traffic.db")
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	createTable()

	// Initialize last usage values
	lastSent, lastReceived, _, _ = getNetworkUsage()
	lastCheckTime = time.Now()

	// Set up HTTP routes
	http.HandleFunc("/", handleHome)
	http.HandleFunc("/data", handleData)
	http.HandleFunc("/update", handleUpdate)
	http.HandleFunc("/current", handleCurrent)
	http.HandleFunc("/summary", handleSummary)

	fmt.Println("Server is running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// createTable creates the data_usage table if it doesn't exist in the database.
// This table stores daily network usage data.
func createTable() {
	query := `
	CREATE TABLE IF NOT EXISTS data_usage (
		date TEXT PRIMARY KEY,
		sent INTEGER,
		received INTEGER
	);`

	_, err := db.Exec(query)
	if err != nil {
		log.Fatal("Failed to create table:", err)
	}
}

// getNetworkUsage retrieves current network usage and calculates speeds.
// It returns total sent and received bytes, and current download and upload speeds in bytes per second.
func getNetworkUsage() (uint64, uint64, float64, float64) {
	stats, err := net.IOCounters(false)
	if err != nil {
		log.Fatal("Failed to get IO counters:", err)
	}

	var sent, received uint64
	for _, stat := range stats {
		sent += stat.BytesSent
		received += stat.BytesRecv
	}

	now := time.Now()
	duration := now.Sub(lastCheckTime).Seconds()
	var downloadSpeed, uploadSpeed float64
	if duration > 0 {
		downloadSpeed = float64(received-lastReceived) / duration
		uploadSpeed = float64(sent-lastSent) / duration
	}

	lastSent, lastReceived = sent, received
	lastCheckTime = now

	return sent, received, downloadSpeed, uploadSpeed
}

// saveDataUsage saves the current day's network usage to the database.
// It upserts (inserts or updates) the data for the current date.
func saveDataUsage(sent, received uint64) {
	date := time.Now().Format("2006-01-02")
	query := `
	INSERT OR REPLACE INTO data_usage (date, sent, received)
	VALUES (?, ?, ?);`

	_, err := db.Exec(query, date, sent, received)
	if err != nil {
		log.Printf("Failed to save data usage: %v", err)
	}
}

// getDataUsage retrieves the last 30 days of network usage data from the database.
// It returns a slice of DataUsage structs, sorted by date in descending order.
func getDataUsage() []DataUsage {
	query := `
	SELECT date, sent, received
	FROM data_usage
	WHERE date >= date('now', '-30 days')
	ORDER BY date DESC;`

	rows, err := db.Query(query)
	if err != nil {
		log.Printf("Failed to query data usage: %v", err)
		return nil
	}
	defer rows.Close()

	var usages []DataUsage
	for rows.Next() {
		var usage DataUsage
		err := rows.Scan(&usage.Date, &usage.Sent, &usage.Received)
		if err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}
		usage.SentHuman = humanize.Bytes(usage.Sent)
		usage.ReceivedHuman = humanize.Bytes(usage.Received)
		usages = append(usages, usage)
	}

	return usages
}

// handleHome serves the main HTML page of the application.
func handleHome(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, getHTMLContent())
}

// handleData serves the last 30 days of usage data as JSON.
func handleData(w http.ResponseWriter, r *http.Request) {
	usages := getDataUsage()
	json.NewEncoder(w).Encode(usages)
}

// handleUpdate saves the current usage data and returns a success message.
func handleUpdate(w http.ResponseWriter, r *http.Request) {
	sent, received, _, _ := getNetworkUsage()
	saveDataUsage(sent, received)
	fmt.Fprintf(w, "Network usage updated successfully!")
}

// handleCurrent serves the current network usage and speeds as JSON.
func handleCurrent(w http.ResponseWriter, r *http.Request) {
	sent, received, downloadSpeed, uploadSpeed := getNetworkUsage()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"sent":          humanize.Bytes(sent),
		"received":      humanize.Bytes(received),
		"downloadSpeed": downloadSpeed,
		"uploadSpeed":   uploadSpeed,
	})
}

// handleSummary serves the total sent and received data for the last 30 days.
func handleSummary(w http.ResponseWriter, r *http.Request) {
	usages := getDataUsage()
	var totalSent, totalReceived uint64
	for _, usage := range usages {
		totalSent += usage.Sent
		totalReceived += usage.Received
	}
	json.NewEncoder(w).Encode(map[string]string{
		"totalSent":     humanize.Bytes(totalSent),
		"totalReceived": humanize.Bytes(totalReceived),
	})
}

// gethtmlContent frpm template.html contains the HTML and JavaScript for the web interface
func getHTMLContent() string {
	content, err := os.ReadFile("template.html")
	if err != nil {
		log.Fatal("Error reading template file:", err)
	}
	return string(content)
}
