package main

import (
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

func main() {
	url := "http://localhost:8080/api/upload/request-url?filename=test.jpg&contentType=image/jpeg"
	
	// Configured limit: 5 requests per minute (burst 5?)
	// Let's try to send 10 requests quickly
	
	totalRequests := 10
	var wg sync.WaitGroup
	
	fmt.Printf("Sending %d requests to %s...\n", totalRequests, url)
	
	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			resp, err := http.Get(url)
			if err != nil {
				fmt.Printf("Request %d failed: %v\n", id, err)
				return
			}
			defer resp.Body.Close()
			
			body, _ := io.ReadAll(resp.Body)
			fmt.Printf("Request %d: Status %d\n", id, resp.StatusCode)
			if resp.StatusCode == 429 {
				fmt.Printf("Request %d BLOCKED (Rate Limit)\n", id)
			} else if resp.StatusCode == 200 {
				fmt.Printf("Request %d OK\n", id)
			} else {
				fmt.Printf("Request %d: %s\n", id, string(body))
			}
		}(i)
		time.Sleep(100 * time.Millisecond) // Small delay to ensure order roughly
	}
	
	wg.Wait()
	fmt.Println("Test complete.")
}
