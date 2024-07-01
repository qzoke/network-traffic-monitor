package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/shirou/gopsutil/net"
	_ "modernc.org/sqlite"
)

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

func main() {
	var err error
	db, err = sql.Open("sqlite", "./network_traffic.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	createTable()

	lastSent, lastReceived, _, _ = getNetworkUsage()
	lastCheckTime = time.Now()

	http.HandleFunc("/", handleHome)
	http.HandleFunc("/data", handleData)
	http.HandleFunc("/update", handleUpdate)
	http.HandleFunc("/current", handleCurrent)

	fmt.Println("Server is running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func createTable() {
	query := `
	CREATE TABLE IF NOT EXISTS data_usage (
		date TEXT PRIMARY KEY,
		sent INTEGER,
		received INTEGER
	);`

	_, err := db.Exec(query)
	if err != nil {
		log.Fatal(err)
	}
}

func getNetworkUsage() (uint64, uint64, float64, float64) {
	stats, err := net.IOCounters(false)
	if err != nil {
		log.Fatal(err)
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

func saveDataUsage(sent, received uint64) {
	date := time.Now().Format("2006-01-02")
	query := `
	INSERT OR REPLACE INTO data_usage (date, sent, received)
	VALUES (?, ?, ?);`

	_, err := db.Exec(query, date, sent, received)
	if err != nil {
		log.Fatal(err)
	}
}

func getDataUsage() []DataUsage {
	query := `
	SELECT date, sent, received
	FROM data_usage
	WHERE date >= date('now', '-30 days')
	ORDER BY date DESC;`

	rows, err := db.Query(query)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	var usages []DataUsage
	for rows.Next() {
		var usage DataUsage
		err := rows.Scan(&usage.Date, &usage.Sent, &usage.Received)
		if err != nil {
			log.Fatal(err)
		}
		usage.SentHuman = humanize.Bytes(usage.Sent)
		usage.ReceivedHuman = humanize.Bytes(usage.Received)
		usages = append(usages, usage)
	}

	return usages
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, htmlContent)
}

func handleData(w http.ResponseWriter, r *http.Request) {
	usages := getDataUsage()
	json.NewEncoder(w).Encode(usages)
}

func handleUpdate(w http.ResponseWriter, r *http.Request) {
	sent, received, _, _ := getNetworkUsage()
	saveDataUsage(sent, received)
	fmt.Fprintf(w, "Network usage updated successfully!")
}

func handleCurrent(w http.ResponseWriter, r *http.Request) {
	sent, received, downloadSpeed, uploadSpeed := getNetworkUsage()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"sent":          humanize.Bytes(sent),
		"received":      humanize.Bytes(received),
		"downloadSpeed": humanize.Bytes(uint64(downloadSpeed)) + "/s",
		"uploadSpeed":   humanize.Bytes(uint64(uploadSpeed)) + "/s",
	})
}

const htmlContent = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Network Traffic Monitor</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
    <style>
        body {
            font-family: Arial, sans-serif;
            line-height: 1.6;
            color: #ffffff;
            max-width: 800px;
            margin: 0 auto;
            padding: 20px;
            background-color: #1e2a3a;
        }
        h1 {
            color: #3498db;
            text-align: center;
        }
        button {
            background-color: #3498db;
            color: white;
            border: none;
            padding: 10px 20px;
            text-align: center;
            text-decoration: none;
            display: inline-block;
            font-size: 16px;
            margin: 4px 2px;
            cursor: pointer;
            border-radius: 4px;
        }
        button:hover {
            background-color: #2980b9;
        }
        #currentUsage {
            background-color: #2c3e50;
            border-radius: 4px;
            padding: 10px;
            margin-top: 20px;
            text-align: center;
            font-size: 18px;
        }
        #chartContainer {
            margin-top: 20px;
            background-color: #2c3e50;
            padding: 20px;
            border-radius: 4px;
        }
        table {
            border-collapse: collapse;
            width: 100%;
            margin-top: 20px;
            background-color: #2c3e50;
        }
        th, td {
            border: 1px solid #34495e;
            padding: 12px;
            text-align: left;
        }
        th {
            background-color: #34495e;
        }
        tr:nth-child(even) {
            background-color: #2c3e50;
        }
    </style>
</head>
<body>
    <h1>Network Traffic Monitor</h1>
    <button onclick="updateUsage()">Update Network Usage</button>
    <div id="currentUsage">
        Current Usage - Sent: <span id="currentSent">0 B</span>, Received: <span id="currentReceived">0 B</span><br>
        Download Speed: <span id="downloadSpeed">0 B/s</span>, Upload Speed: <span id="uploadSpeed">0 B/s</span>
    </div>
    <div id="chartContainer">
        <canvas id="networkChart"></canvas>
    </div>
    <table id="usageTable">
        <tr>
            <th>Date</th>
            <th>Sent</th>
            <th>Received</th>
        </tr>
    </table>

    <script>
        let chart;
        const dataPoints = [];

        function initChart() {
            const ctx = document.getElementById('networkChart').getContext('2d');
            chart = new Chart(ctx, {
                type: 'line',
                data: {
                    labels: [],
                    datasets: [{
                        label: 'Download Speed',
                        borderColor: '#3498db',
                        data: [],
                        fill: false
                    }]
                },
                options: {
                    responsive: true,
                    scales: {
                        y: {
                            beginAtZero: true,
                            title: {
                                display: true,
                                text: 'Speed (B/s)'
                            }
                        }
                    }
                }
            });
        }

        function updateChart() {
            fetch('/current')
                .then(response => response.json())
                .then(data => {
                    const downloadSpeed = parseFloat(data.downloadSpeed.replace(/[^0-9.]/g, ''));
                    const time = new Date().toLocaleTimeString();
                    
                    if (chart.data.labels.length > 20) {
                        chart.data.labels.shift();
                        chart.data.datasets[0].data.shift();
                    }
                    
                    chart.data.labels.push(time);
                    chart.data.datasets[0].data.push(downloadSpeed);
                    chart.update();
                    
                    document.getElementById('currentSent').textContent = data.sent;
                    document.getElementById('currentReceived').textContent = data.received;
                    document.getElementById('downloadSpeed').textContent = data.downloadSpeed;
                    document.getElementById('uploadSpeed').textContent = data.uploadSpeed;
                });
        }

        function updateTable() {
            fetch('/data')
                .then(response => response.json())
                .then(data => {
                    const table = document.getElementById('usageTable');
                    table.innerHTML = '<tr><th>Date</th><th>Sent</th><th>Received</th></tr>';
                    data.forEach(item => {
                        const row = table.insertRow();
                        row.insertCell(0).textContent = item.date;
                        row.insertCell(1).textContent = item.sentHuman;
                        row.insertCell(2).textContent = item.receivedHuman;
                    });
                });
        }

        function updateUsage() {
            fetch('/update')
                .then(response => response.text())
                .then(message => {
                    alert(message);
                    updateTable();
                });
        }

        initChart();
        updateTable();
        setInterval(updateChart, 1000); // Update chart and current usage every second
        setInterval(updateTable, 60000); // Update table every minute
    </script>
</body>
</html>
`
