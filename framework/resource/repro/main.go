package main

import (
	"fmt"

	"github.com/hashicorp/waypoint-plugin-sdk/framework/resource"
)

func main() {
	// init is a function so that we can reinitialize an empty manager
	// for this test to test loading state

	for i := 0; i < 1000; i++ {
		err := resource.Bad()
		if err != nil {
			fmt.Printf("Failed on iteration %d: %v", i, err)
			return
		}
	}
	fmt.Println("Didn't fail")
	return
}
