package main

import (
	"bufio"
	"fmt"
	"obwo/libraries/util"
	"os"
	"slices"
	"strings"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("ERROR %v\n", err)
		}
	}()
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		if scanner.Text() == "" {
			continue
		}
		results := util.SQL(scanner.Text())
		keys := []string{}
		if len(results) > 0 {
			for k, _ := range results[0] {
				keys = append(keys, k)
			}
		}
		slices.Sort(keys)
		fmt.Println(strings.Join(keys, "\t"))
		for _, r := range results {
			for _, k := range keys {
				fmt.Print(r[k], "\t")
			}
		}
		fmt.Println()
	}
}
