package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run check_modtime.go <file>")
		return
	}
	
	info, err := os.Stat(os.Args[1])
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	
	fmt.Printf("ModTime: %v\n", info.ModTime().Format(time.RFC3339Nano))
}

