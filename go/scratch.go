package main

import (
	"fmt"
	"strings"
)

func main() {
	content := "---" + "\n" + "description: foo\n" + "---" + "\n" + "system 1\n" + "---" + "\nsystem 2"
	parts := strings.SplitN(content, "---", 3)
	fmt.Printf("len: %d\n", len(parts))
	for i, p := range parts {
		fmt.Printf("part %d: %q\n", i, p)
	}
}
