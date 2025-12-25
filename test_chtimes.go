package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	testFile := "/Volumes/admin/Pictures/test_chtimes.txt"
	
	// Create a test file
	f, err := os.Create(testFile)
	if err != nil {
		fmt.Printf("Error creating file: %v\n", err)
		return
	}
	f.WriteString("test")
	f.Close()
	
	// Set a specific modtime (2023-12-20 21:39:06 with nanoseconds)
	targetTime := time.Date(2023, 12, 20, 21, 39, 6, 116054300, time.Local)
	fmt.Printf("Setting modtime to: %v\n", targetTime.Format(time.RFC3339Nano))
	
	err = os.Chtimes(testFile, targetTime, targetTime)
	if err != nil {
		fmt.Printf("Error setting time: %v\n", err)
		os.Remove(testFile)
		return
	}
	
	// Read it back
	info, err := os.Stat(testFile)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		os.Remove(testFile)
		return
	}
	
	actualTime := info.ModTime()
	fmt.Printf("Actual modtime:     %v\n", actualTime.Format(time.RFC3339Nano))
	
	if targetTime.Equal(actualTime) {
		fmt.Println("✓ Timestamps match exactly!")
	} else {
		fmt.Printf("✗ Timestamps differ by: %v\n", targetTime.Sub(actualTime))
	}
	
	// Clean up
	os.Remove(testFile)
}

