package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

func mockHandler(serviceName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Return dummy response with transaction_amount so Gateway can charge a fee
		resp := map[string]interface{}{
			"status":             "success",
			"service":            serviceName,
			"path":               r.URL.Path,
			"transaction_amount": 100000,
			"message":            fmt.Sprintf("Mock response from %s", serviceName),
		}
		json.NewEncoder(w).Encode(resp)
	}
}

func startServer(port int, serviceName string, wg *sync.WaitGroup) {
	defer wg.Done()
	
	mux := http.NewServeMux()
	mux.HandleFunc("/", mockHandler(serviceName))

	fmt.Printf("Started Mock %-15s Server on port %d\n", serviceName, port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), mux); err != nil {
		fmt.Printf("Error starting server on port %d: %v\n", port, err)
	}
}

func main() {
	ports := map[int]string{
		8101: "SmartBank",
		8102: "Marketplace",
		8103: "POS",
		8104: "SupplierHub",
		8105: "LogistiKita",
		8106: "UMKM Insight",
	}

	var wg sync.WaitGroup
	
	fmt.Println("=== Starting All Mock Downstream Servers ===")
	for port, serviceName := range ports {
		wg.Add(1)
		go startServer(port, serviceName, &wg)
	}

	fmt.Println("All mock servers are running. Press Ctrl+C to stop.")
	wg.Wait()
}
