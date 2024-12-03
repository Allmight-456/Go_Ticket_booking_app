# Go Ticket Booking CLI App

A robust backend application for managing conference ticket bookings. This project is built with **Go** and incorporates advanced features such as package-level variables, modular code organization, inter-package communication, maps, slices, and input validation.

---

## Features

### Core Functionalities:
- **Greeting Users**: Displays a welcome message with ticket details.
- **Ticket Booking**: Allows users to book tickets, stores user details in a map, and maintains a list of bookings.
- **Fetching Data**: Retrieves all first names of booked users.
- **Validation**: Includes input validation for user details and ticket counts using a dedicated package.

### Code Organization:
- **Main Package**:
  - `main.go`: Contains core functions like `greetUsers`, `bookTickets`, and `getUserInput`.
  - `helper.go`: Includes utility functions like `Fetch_firstNames`,`sendTickets`.
- **Validation Package**:
  - `validate.go`: Provides the `ValidateUser` function for input validation.
- **Data Structures**:
  - Uses Struct type for custom data type storing multiple types of values in same structure
  - Uses maps to store booking details (`firstName`, `lastName`, `email`, `numberOfTickets`(changed to string)).
  - Slices are used to manage a list of all bookings.

---

## Getting Started

### Prerequisites
- **Go Language**: Ensure Go is installed. Download it [here](https://go.dev/dl/).

---

### Setup Instructions

1. **Initialize a Module**  
   Run the following in the root directory to set up the Go module:
   ```bash
   go mod init <module-name>


### Setup Instructions

1. **Initialize a Module**  
   Run the following command to initialize the Go module in the root directory:
   ```bash
   go mod init <module-name>
2. **Clone the repository**
   ```bash
   git clone https://github.com/Allmight-456/Go_Ticket_booking_app.git
   cd <project-directory>

3. **Run the Application**
   ```bash
   go run .

4. **Build the Application**
      ```bash
         go build

### Example Interaction
   ```bash
      go run .  03:59:49 AM
      Welcome to the Go Conference  ticketing application
      We have total of 50 tickets and 50 are still available
      Get your tickets to  Go Conference  here
      Enter your first name: 
      Ishan 
      Enter your last name: 
      Kumar
      Enter your email: 
      ishan@gmail.com
      Enter number of tickets: 
      42
      List of bookings is [{Ishan Kumar ishan@gmail.com 42}]
      Thank You,User Ishan Kumar with email ishan@gmail.com book 42 tickets
      8 tickets remaining for Go Conference
      All of first names of the bookings are [Ishan]
      Enter your first name: 
      ram
      Enter your last name: 
      shyam
      Enter your email: 
      shay\###############
      Sending ticket:
       42 tickets for Ishan Kumar 
      to email address ishan@gmail.com
      ###############
      ^R
      shayam@
      Enter number of tickets: 
      6
      List of bookings is [{Ishan Kumar ishan@gmail.com 42} {ram shyam shayam@ 6}]
      Thank You,User ram shyam with email shayam@ book 6 tickets
      2 tickets remaining for Go Conference
      All of first names of the bookings are [Ishan ram]
      Enter your first name: 
      shyam
      Enter your last name: 
      
      ###############
      Sending ticket:
       6 tickets for ram shyam 
      to email address shayam@
      ###############
      kuma
      Enter your email: 
      shy@
      Enter number of tickets: 
      6
      We have 2 tickets remaining. Please enter a valid number of tickets.Try Again
      Enter your first name: 
      for
      Enter your last name: 
      ham
      Enter your email: 
      farham@gmail.com
      Enter number of tickets: 
      2
      List of bookings is [{Ishan Kumar ishan@gmail.com 42} {ram shyam shayam@ 6} {for ham farham@gmail.com 2}]
      Thank You,User for ham with email farham@gmail.com book 2 tickets
      0 tickets remaining for Go Conference
      All of first names of the bookings are [Ishan ram for]
      ###############
      Sending ticket:
       2 tickets for for ham 
      to email address farham@gmail.com
      ###############
