package main

import (
	"fmt"
	"log"
	"os"

	"forum/server/cloud"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Get Azure connection string
	connectionString := os.Getenv("AZURE_STORAGE_CONNECTION_STRING")
	if connectionString == "" {
		log.Fatal("AZURE_STORAGE_CONNECTION_STRING not found in .env")
	}

	// Create Azure Storage client
	fmt.Println("Connecting to Azure Storage...")
	azureStorage, err := cloud.NewAzureStorage(connectionString)
	if err != nil {
		log.Fatal("Failed to create Azure Storage client:", err)
	}

	fmt.Println("✓ Connected successfully!")
	fmt.Println("  Account Name:", azureStorage.GetAccountName())

	//Test generating SAS token
	fmt.Println("\nTesting SAS token generation...")
	quarantineContainer := os.Getenv("AZURE_QUARANTINE_CONTAINER")
	if quarantineContainer == "" {
		quarantineContainer = "quarantine-container"
	}

	testBlobName := fmt.Sprintf("test_%d.jpg", 123456789)
	sasURL, err := azureStorage.GenerateSASToken(quarantineContainer, testBlobName, 5)
	if err != nil {
		log.Fatal("Failed to generate SAS token:", err)
	}

	fmt.Println("✓ SAS token generated!")
	fmt.Printf("  Blob: %s\n", testBlobName)
	fmt.Printf("  URL (first 100 chars): %s...\n", sasURL[:100])
	fmt.Println("  Expires in: 5 minutes")

	// Test get public URL
	productionContainer := os.Getenv("AZURE_PRODUCTION_CONTAINER")
	if productionContainer == "" {
		productionContainer = "post-images"
	}

	publicURL := azureStorage.GetPublicURL(productionContainer, "sample.jpg")
	fmt.Println("\n✓ Public URL generation works!")
	fmt.Printf("  URL: %s\n", publicURL)

	fmt.Println("\n🎉 All tests passed! Azure Storage client is ready to use.")
}
