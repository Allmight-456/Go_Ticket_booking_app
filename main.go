package main

import "fmt"

//First initialise the root directory : go mod init <module-name>
// To run: go run main.go
// To build: go build


func main() {
	var conferenceName string = "Go Conference"
	const conferenceTickets uint = 50
	var remainingTickets uint = 50

	//different print methods for formatted and unformatted strings
	fmt.Println("Welcome to the", conferenceName, " ticketing application")
	fmt.Printf("We have total of %v tickets and %v are still available\n", conferenceTickets, remainingTickets)
	fmt.Println("Get your tickets to ", conferenceName, " here") 

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

	remainingTickets = remainingTickets - userTickets
	// %v is a placeholder for the value of the variable
	fmt.Printf("Thank You,User %v %v with email %v book %v tickets\n", firstName, lastName, email, userTickets)
	fmt.Printf("%v tickets remaining for %v\n", remainingTickets, conferenceName)
	// %T is a placeholder for the type of the variable

}
