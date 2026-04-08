package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	content, err := os.ReadFile("/mnt/storage/Project/paravizor/internal/tui/components/home/home.go")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	s := string(content)
	
	s = strings.Replace(s, "Height(h - 2).", "Height(h).", 1)
	
	os.WriteFile("/mnt/storage/Project/paravizor/internal/tui/components/home/home.go", []byte(s), 0644)
}
