package main

import (
	"fmt"
	"strings"
)

//First initialise the root directory : go mod init <module-name>
// To run: go run main.go
// To build: go build

func main() {
	var conferenceName string = "Go Conference"
	const conferenceTickets uint = 50
	var remainingTickets uint = 50
	// var bookings [50]string          //array of size 50
	var bookings []string //slices => array of size unknown
	// bookings := []string{}           //other syntax

	//different print methods for formatted and unformatted strings
	fmt.Println("Welcome to the", conferenceName, " ticketing application")
	fmt.Printf("We have total of %v tickets and %v are still available\n", conferenceTickets, remainingTickets)
	fmt.Println("Get your tickets to ", conferenceName, " here")

	for remainingTickets > 0 {
		//declaring variables
		var userTickets uint
		var firstName string
		var lastName string
		var email string

		//assigning values to variables
		fmt.Println("Enter your first name: ")
		fmt.Scan(&firstName)

		fmt.Println("Enter your last name: ")
		fmt.Scan(&lastName)

		fmt.Println("Enter your email: ")
		fmt.Scan(&email)

		fmt.Println("Enter number of tickets: ")
		fmt.Scan(&userTickets)

		isValidName := len(firstName) >= 2 && len(lastName) >= 2
		isValidEmail := strings.Contains(email, "@")
		isValidTicket := userTickets > 0 && userTickets <= remainingTickets

		// Validate user input
		if !isValidTicket || !isValidEmail || !isValidName {
			
			if !isValidName {
				fmt.Println("First or Last name too short")
			}
			if !isValidEmail{
				fmt.Println("Enter a Valid Email Address")
			}
			if !isValidTicket{
				fmt.Printf("We have %v tickets remaining. Please enter a valid number of tickets.Try Again\n", remainingTickets)
			}
		} else {
			remainingTickets -= userTickets

			bookings = append(bookings, firstName+" "+lastName)

			fmt.Printf("Thank You,User %v %v with email %v book %v tickets\n", firstName, lastName, email, userTickets)
			fmt.Printf("%v tickets remaining for %v\n", remainingTickets, conferenceName)

			firstNames := []string{}
			for _, value := range bookings {
				var name = strings.Fields(value)
				firstNames = append(firstNames, name[0])
			}

			fmt.Printf("All of first names of the bookings are %v\n", firstNames)
		}
	}
}
