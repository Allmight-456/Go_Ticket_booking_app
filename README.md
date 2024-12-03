# Go Ticket Booking Application

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
  - `helper.go`: Includes utility functions like `Fetch_firstNames`.
- **Validation Package**:
  - `validate.go`: Provides the `ValidateUser` function for input validation.
- **Data Structures**:
  - Uses maps to store booking details (`firstName`, `lastName`, `email`, `numberOfTickets`).
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
      Welcome to the Go Conference  ticketing application
      We have total of 50 tickets and 50 are still available
      Get your tickets to  Go Conference  here
      Enter your first name: 
      Ishan
      Enter your last name: 
      ku
      Enter your email: 
      ishu@gmail.com
      Enter number of tickets: 
      24
      List of bookings is [map[email:ishu@gmail.com firstName:Ishan lastName:ku numberOfTickets:24]]
      Thank You,User Ishan ku with email ishu@gmail.com book 24 tickets
      26 tickets remaining for Go Conference
      All of first names of the bookings are [Ishan]
      Enter your first name: 
      ram
      Enter your last name: 
      ni
      Enter your email: 
      sia@gmail.com
      Enter number of tickets: 
      49
      We have 26 tickets remaining. Please enter a valid number of tickets.Try Again
      Enter your first name: 
      sita
      Enter your last name: 
      ram
      Enter your email: 
      sitaram@gmail.com
      Enter number of tickets: 
      25
      List of bookings is [map[email:ishu@gmail.com firstName:Ishan lastName:ku numberOfTickets:24] map[email:sitaram@gmail.com firstName:sita lastName:ram numberOfTickets:25]]
      Thank You,User sita ram with email sitaram@gmail.com book 25 tickets
      1 tickets remaining for Go Conference
      All of first names of the bookings are [Ishan sita]
      Enter your first name: 
