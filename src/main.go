package main

import (
	"crypto/rand"
	"fmt"
	"time"
)

func main() {
	bc := NewBlockchain()

	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		fmt.Println("Error generating key:", err)
		return
	}

	bc.AddMessage("Danya", "Asur", "Hello Asur!", key)
	time.Sleep(time.Second)
	bc.AddMessage("Asur", "Danya", "Hi Danya, how are you?", key)

	fmt.Println("Blockchain contents (encrypted):")
	for _, block := range bc.Chain {
		fmt.Printf("Index: %d\n", block.Index)
		fmt.Printf("Hash: %s\n", block.Hash)
		fmt.Printf("PrevHash: %s\n", block.PrevHash)
		fmt.Printf("Messages: %v\n\n", block.Messages)
	}

	fmt.Println("Decrypted messages for Danya:")
	for _, msg := range bc.ReadMessages("Danya", key) {
		fmt.Println(msg)
	}

	fmt.Println("\nDecrypted messages for Asur:")
	for _, msg := range bc.ReadMessages("Asur", key) {
		fmt.Println(msg)
	}

	fmt.Println("\nBlockchain valid?", bc.VerifyChain())

	startNetwork()
	select {}
}
