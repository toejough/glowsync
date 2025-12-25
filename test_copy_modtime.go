package main

import (
	"fmt"
	"os"
	"time"
	
	"github.com/joe/copy-files/pkg/fileops"
)

func main() {
	srcFile := "/Volumes/photo/lr-exports/shared/reorganized/2022/2022-05/2022-05-16.AT8A1752.jpg"
	dstFile := "/Volumes/admin/Pictures/test_copy_with_stats.jpg"
	
	// Get source modtime
	srcInfo, err := os.Stat(srcFile)
	if err != nil {
		fmt.Printf("Error reading source: %v\n", err)
		return
	}
	srcModTime := srcInfo.ModTime()
	fmt.Printf("Source modtime: %v\n", srcModTime.Format(time.RFC3339Nano))
	
	// Copy using our function
	stats, err := fileops.CopyFileWithStats(srcFile, dstFile, nil, nil)
	if err != nil {
		fmt.Printf("Copy error: %v\n", err)
		os.Remove(dstFile)
		return
	}
	
	fmt.Printf("Copied %d bytes\n", stats.BytesCopied)
	
	// Check destination modtime
	dstInfo, err := os.Stat(dstFile)
	if err != nil {
		fmt.Printf("Error reading dest: %v\n", err)
		os.Remove(dstFile)
		return
	}
	dstModTime := dstInfo.ModTime()
	fmt.Printf("Dest modtime:   %v\n", dstModTime.Format(time.RFC3339Nano))
	
	if srcModTime.Equal(dstModTime) {
		fmt.Println("✓ SUCCESS: Modtimes match!")
	} else {
		fmt.Printf("✗ FAIL: Modtimes differ by: %v\n", srcModTime.Sub(dstModTime))
	}
	
	// Clean up
	os.Remove(dstFile)
}

