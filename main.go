package main

import (
	"fmt"
	"go_booking-app/validate"
	"sync"
)

type UserData struct {       //new data type for entries having diffrent type of value
	firstName string
	lastName string
	email string
	numberOfTickets uint
}

//wait Group to wait for the goroutines to finish
var waitGroup = sync.WaitGroup{}

//First initialise the root directory : go mod init <module-name>
// To run: go run main.go
//To run for multiple packages: go run .
// To build: go build

//SHARED VARIABLES FOR ALL FUNCTIONS , PACKAGE LEVEL VARIABLES
var conferenceName string = "Go Conference"
const conferenceTickets uint = 50
var remainingTickets uint = 50
// var bookings [50]string          //array of size 50
// var bookings []string              //slices => array of size unknown
// bookings := []string{}
// var bookings = make([]map[string]string,0)     //list map containing key value pairs
var bookings = make([]UserData,0)


func main() {

	greetUsers()

	for remainingTickets > 0 {
		firstName, lastName, email, userTickets := getUserInput()

		isValidName, isValidEmail, isValidTicket := validate.ValidateUserInput(firstName, lastName, email, userTickets,remainingTickets)

		// Validate user input
		if !isValidTicket || !isValidEmail || !isValidName {

			if !isValidName {
				fmt.Println("First or Last name too short")
			}
			if !isValidEmail {
				fmt.Println("Enter a Valid Email Address")
			}
			if !isValidTicket {
				fmt.Printf("We have %v tickets remaining. Please enter a valid number of tickets.Try Again\n", remainingTickets)
			}
		} else {

			bookTickets(userTickets, firstName, lastName, email)

			waitGroup.Add(1)          //increases the counter for main thread to wait
			//send ticket via email as a goroutine ,concurrency
			go sendTicket(userTickets, firstName, lastName, email)
			Fetch_firstNames()
		}

	}
	waitGroup.Wait()
}

func greetUsers() {
	//different print methods for formatted and unformatted strings
	fmt.Println("Welcome to the", conferenceName, " ticketing application")
	fmt.Printf("We have total of %v tickets and %v are still available\n", conferenceTickets, remainingTickets)
	fmt.Println("Get your tickets to ", conferenceName, " here")

}


func getUserInput() (string, string, string, uint) {
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

	return firstName, lastName, email, userTickets
}

func bookTickets(userTickets uint, firstName string, lastName string, email string) {
	remainingTickets -= userTickets

	// //create a map for bookings to contain list of key-value pairs
	// var userData = make(map[string]string)
	// userData["firstName"]= firstName
	// userData["lastName"] = lastName
	// userData["email"] = email
	// userData["numberOfTickets"] = fmt.Sprintf("%v", userTickets)   //convert uint to string

	userData := UserData{
		firstName:firstName,
		lastName:lastName, 
		email:email, 
		numberOfTickets:userTickets,
	}
	bookings = append(bookings, userData)

	fmt.Printf("List of bookings is %v\n", bookings)
	fmt.Printf("Thank You,User %v %v with email %v book %v tickets\n", firstName, lastName, email, userTickets)
	fmt.Printf("%v tickets remaining for %v\n", remainingTickets, conferenceName)
}