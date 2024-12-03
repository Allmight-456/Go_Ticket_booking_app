//files in the same package do not need to be imported
package main

import (
	"fmt"
	"time"
)

// To export a function make 1st letter Capitalised
func Fetch_firstNames() []string {
	firstNames := []string{}
	for _, booking := range bookings {     //booking is now a struct with name UserData , access it by value.firstName
		firstNames = append(firstNames, booking.firstName)
	}

	fmt.Printf("All of first names of the bookings are %v\n", firstNames)
	return firstNames
}

func sendTicket(userTickets uint, firstName string, lastName string, email string) {
	time.Sleep(10 * time.Second)    //wait for 10 seconds
	var ticket = fmt.Sprintf("%v tickets for %v %v", userTickets, firstName, lastName)
	fmt.Println("###############")
	fmt.Printf("Sending ticket:\n %v \nto email address %v\n", ticket, email)
	fmt.Println("###############")

	waitGroup.Done()     //when the send email is executed after 10 seconds, decrement the counter , decramenting the conter of waitGroup
}
