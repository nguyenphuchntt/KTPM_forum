package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"

	"github.com/joho/godotenv"
)

// Test script để test upload endpoint
func main() {
	// Load .env
	godotenv.Load()

	baseURL := "http://localhost:8080"

	fmt.Println("=== Testing Upload Backend ===\n")

	// Test 1: Without authentication (should fail)
	fmt.Println("Test 1: Request SAS token without authentication")
	testRequestSASWithoutAuth(baseURL)

	// Test 2: With authentication (need to login first)
	fmt.Println("\nTest 2: Register, Login and request SAS token")
	testRegister(baseURL)
	sessionCookie := testLogin(baseURL)
	if sessionCookie != "" {
		testRequestSASWithAuth(baseURL, sessionCookie)
	}
}

func testRequestSASWithoutAuth(baseURL string) {
	requestBody := map[string]interface{}{
		"filename":     "test.jpg",
		"size":         1024000, // 1MB
		"content_type": "image/jpeg",
	}

	body, _ := json.Marshal(requestBody)

	resp, err := http.Post(baseURL+"/api/upload/request-url", "application/json", bytes.NewBuffer(body))
	if err != nil {
		log.Printf("  ✗ Request failed: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		fmt.Println("  ✓ Correctly rejected (401 Unauthorized)")
	} else {
		fmt.Printf("  ✗ Expected 401, got %d\n", resp.StatusCode)
	}
}

func testRegister(baseURL string) {
	formData := url.Values{
		"email":                 {"testuser@example.com"},
		"username":              {"TestUser"},
		"password":              {"TestPass123"},
		"password-confirmation": {"TestPass123"},
	}

	resp, err := http.PostForm(baseURL+"/signup", formData)
	if err != nil {
		log.Printf("  ✗ Register failed: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 || resp.StatusCode == 302 {
		fmt.Println("  ✓ Registration successful")
	} else {
		// Ignore 400/500 if user already exists
		fmt.Printf("  ! Registration response: %d (User might already exist)\n", resp.StatusCode)
	}
}

func testLogin(baseURL string) string {
	// Create client with cookie jar
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar: jar,
	}

	// Try to login (adjust credentials as needed)
	formData := url.Values{
		"username": {"TestUser"},
		"password": {"TestPass123"},
	}

	resp, err := client.PostForm(baseURL+"/signin", formData)
	if err != nil {
		log.Printf("  ✗ Login failed: %v", err)
		return ""
	}
	defer resp.Body.Close()

	// Check cookies in jar
	u, _ := url.Parse(baseURL)
	for _, cookie := range jar.Cookies(u) {
		if cookie.Name == "session_id" {
			fmt.Printf("  ✓ Login successful, session: %s...\n", cookie.Value[:10])
			return cookie.Value
		}
	}

	fmt.Printf("  ✗ No session cookie received. Status: %d\n", resp.StatusCode)
	return ""
}

func testRequestSASWithAuth(baseURL, sessionCookie string) {
	// Test valid request
	fmt.Println("\n  Test 2a: Valid image (1MB .jpg)")
	testSASRequest(baseURL, sessionCookie, "test.jpg", 1024000, "image/jpeg")

	// Test invalid size
	fmt.Println("\n  Test 2b: Invalid size (10MB, should fail)")
	testSASRequest(baseURL, sessionCookie, "large.jpg", 10*1024*1024, "image/jpeg")

	// Test invalid extension
	fmt.Println("\n  Test 2c: Invalid extension (.pdf, should fail)")
	testSASRequest(baseURL, sessionCookie, "doc.pdf", 1024000, "application/pdf")
}

func testSASRequest(baseURL, sessionCookie, filename string, size int64, contentType string) {
	requestBody := map[string]interface{}{
		"filename":     filename,
		"size":         size,
		"content_type": contentType,
	}

	body, _ := json.Marshal(requestBody)

	req, _ := http.NewRequest("POST", baseURL+"/api/upload/request-url", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "session_id", Value: sessionCookie})

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("    ✗ Request failed: %v", err)
		return
	}
	defer resp.Body.Close()

	responseBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 200 {
		var result map[string]interface{}
		json.Unmarshal(responseBody, &result)

		fmt.Println("    ✓ SAS token generated successfully!")
		if uploadURL, ok := result["upload_url"].(string); ok {
			fmt.Printf("      Upload URL (first 80 chars): %s...\n", uploadURL[:min(80, len(uploadURL))])
		}
		if publicURL, ok := result["public_url"].(string); ok {
			fmt.Printf("      Public URL: %s\n", publicURL)
		}
		if expiresIn, ok := result["expires_in"].(float64); ok {
			fmt.Printf("      Expires in: %.0f seconds\n", expiresIn)
		}
	} else {
		fmt.Printf("    ✗ Request failed: %d\n", resp.StatusCode)
		fmt.Printf("      Response: %s\n", string(responseBody))
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
