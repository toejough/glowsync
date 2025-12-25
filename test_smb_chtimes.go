package main

import (
	"fmt"
	"io"
	"os"
	"time"
)

func main() {
	srcFile := "/Volumes/photo/lr-exports/shared/reorganized/2022/2022-05/2022-05-16.AT8A1752.jpg"
	dstFile := "/Volumes/admin/Pictures/test_copy.jpg"
	
	// Get source modtime
	srcInfo, err := os.Stat(srcFile)
	if err != nil {
		fmt.Printf("Error reading source: %v\n", err)
		return
	}
	srcModTime := srcInfo.ModTime()
	fmt.Printf("Source modtime: %v\n", srcModTime.Format(time.RFC3339Nano))
	
	// Copy file
	src, _ := os.Open(srcFile)
	dst, _ := os.Create(dstFile)
	io.Copy(dst, src)
	src.Close()
	dst.Close()
	
	// Check modtime before Chtimes
	info1, _ := os.Stat(dstFile)
	fmt.Printf("Dest modtime before Chtimes: %v\n", info1.ModTime().Format(time.RFC3339Nano))
	
	// Set modtime
	fmt.Printf("Calling Chtimes with: %v\n", srcModTime.Format(time.RFC3339Nano))
	err = os.Chtimes(dstFile, srcModTime, srcModTime)
	if err != nil {
		fmt.Printf("Chtimes error: %v\n", err)
	} else {
		fmt.Printf("Chtimes returned success\n")
	}
	
	// Check modtime after Chtimes
	info2, _ := os.Stat(dstFile)
	fmt.Printf("Dest modtime after Chtimes: %v\n", info2.ModTime().Format(time.RFC3339Nano))
	
	if srcModTime.Equal(info2.ModTime()) {
		fmt.Println("✓ Modtimes match!")
	} else {
		fmt.Printf("✗ Modtimes differ by: %v\n", srcModTime.Sub(info2.ModTime()))
	}
	
	// Clean up
	os.Remove(dstFile)
}

