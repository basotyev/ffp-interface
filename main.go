package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
)

type PredictRequest struct {
	AOI  [][][]float64 `json:"aoi"`  // GeoJSON Polygon
	Date string        `json:"date"` // YYYY-MM-DD
}

type PredictResponse struct {
	Probability float64 `json:"probability"`
	Message     string  `json:"message"`
}

var predictAPIURL string

func main() {
	predictAPIURL = os.Getenv("PREDICT_API_URL")

	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/predict", predictHandler)

	fmt.Println("Server running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
	<title>Fire Risk Prediction</title>
	<meta charset="UTF-8">
	<style>
		body { font-family: Arial; }
		#map { height: 400px; width: 100%; margin-bottom: 10px; }
	</style>
	<link rel="stylesheet" href="https://unpkg.com/leaflet@1.9.4/dist/leaflet.css"/>
	<script src="https://unpkg.com/leaflet@1.9.4/dist/leaflet.js"></script>
</head>
<body>
	<h2>Fire Risk Estimation in Kazakhstan</h2>
	<div id="map"></div>
	<label>Date: <input type="date" id="date_input"/></label>
	<button onclick="sendRequest()">Check Fire Risk</button>
	<p id="result"></p>

	<script>
		var map = L.map('map').setView([50.0, 79.0], 8);
		L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png').addTo(map);

		var drawnPolygon = null;

		map.on('click', function(e) {
			if (!drawnPolygon) {
				drawnPolygon = L.polygon([ [e.latlng] ]).addTo(map);
			} else {
				let latlngs = drawnPolygon.getLatLngs()[0];
				latlngs.push(e.latlng);
				drawnPolygon.setLatLngs([latlngs]);
			}
		});

		function sendRequest() {
			if (!drawnPolygon) {
				alert("Please draw a polygon by clicking on the map");
				return;
			}
			let coords = drawnPolygon.toGeoJSON().geometry.coordinates;
			let date = document.getElementById('date_input').value;
			fetch("/predict", {
				method: "POST",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({ aoi: coords, date: date })
			})
			.then(res => res.json())
			.then(data => {
				document.getElementById("result").innerText = "Fire probability: " + data.probability + " (" + data.message + ")";
			})
			.catch(err => alert("Error: " + err));
		}
	</script>
</body>
</html>`
	t := template.Must(template.New("index").Parse(tmpl))
	t.Execute(w, nil)
}

func predictHandler(w http.ResponseWriter, r *http.Request) {
	var req PredictRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Forward to Python prediction service
	reqJSON, _ := json.Marshal(req)
	resp, err := http.Post(predictAPIURL, "application/json", bytes.NewReader(reqJSON))
	if err != nil {
		log.Println(err)
		http.Error(w, "Prediction service error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		http.Error(w, "Prediction service error: "+err.Error(), http.StatusInternalServerError)
	}
	w.WriteHeader(resp.StatusCode)
	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}
