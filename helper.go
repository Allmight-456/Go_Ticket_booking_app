//files in the same package do not need to be imported
package main

import (
	"fmt"
)

// To export a function make 1st letter Capitalised
func Fetch_firstNames() []string {
	firstNames := []string{}
	for _, booking := range bookings {     //booking is now a list value that contains a map of firstName,lastName,email,numberOfTickets to it's values
		firstNames = append(firstNames, booking["firstName"])
	}

	fmt.Printf("All of first names of the bookings are %v\n", firstNames)
	return firstNames
}
