package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var serverPort = 8080

var logDirectory = "chatlogs"

// get a timestamp with year, month, day, hour, and minute
var timestamp = time.Now().Format("2006-01-02-15-04")
var logFile = fmt.Sprintf("%s-chat.log", timestamp)

// join the path of the logs folder and the logfile
var logPath = fmt.Sprintf("%s/%s", logDirectory, logFile)

// create the logs folder if it doesn't exist
func init() {
	if _, err := os.Stat(logDirectory); os.IsNotExist(err) {
		os.Mkdir(logDirectory, 0755)
	}
}

type message struct {
	Username *string `json:"username,omitempty"`
	Message  string  `json:"message,omitempty"`
}

type client struct {
	conn     *websocket.Conn
	username string
}

var clients []*client

func main() {
	r := gin.Default()

	r.Static("/static", "./static")

	r.GET("/", func(c *gin.Context) {
		http.ServeFile(c.Writer, c.Request, "index.html")
	})

	r.GET("/ws", func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Println(err)
			return
		}

		defer conn.Close()

		log.Println("New client connected")

		var username string

		// Prompt client to enter username
		err = conn.WriteJSON(message{Message: "Please enter your username"})
		if err != nil {
			log.Println(err)
			return
		}

		// Wait for username from client
		err = conn.ReadJSON(&message{Username: &username})
		if err != nil {
			log.Println(err)
			return
		}

		// Add client to list of connected clients
		client := &client{conn, username}
		clients = append(clients, client)

		// Log list of current clients
		log.Println("Current clients:")
		for _, c := range clients {
			log.Println(c.username)
		}

		// Send welcome message to the user
		welcomeMsg := message{Message: fmt.Sprintf("Welcome, %s!", username)}
		err = conn.WriteJSON(welcomeMsg)
		if err != nil {
			log.Println(err)
			return
		}

		// Broadcast message to all connected clients that a new user has joined
		newUserMsg := message{Message: fmt.Sprintf("%s has joined the chat", username)}
		for _, c := range clients {
			err = c.conn.WriteJSON(newUserMsg)
			if err != nil {
				log.Println(err)
				break
			}
		}

		logMessage(username, newUserMsg.Message)

		// Log and broadcast first message
		var firstMsg message
		err = conn.ReadJSON(&firstMsg)
		if err != nil {
			log.Println(err)
			return
		}

		if firstMsg.Message != "" {
			// Broadcast message to all connected clients including the sender
			for _, c := range clients {
				err = c.conn.WriteJSON(firstMsg)
				if err != nil {
					log.Println(err)
					break
				}
			}

			logMessage(username, firstMsg.Message)
		}

		for {
			// Read message from client
			var msg message
			err = conn.ReadJSON(&msg)
			if err != nil {
				log.Println(err)
				break
			}

			if msg.Message != "" {
				// Broadcast message to all connected clients including the sender
				for _, c := range clients {
					err = c.conn.WriteJSON(msg)
					if err != nil {
						log.Println(err)
						break
					}
				}

				logMessage(username, msg.Message)
			}

		}

		// Remove client from list of connected clients and send broadcast that user has left the chat
		for i, c := range clients {
			if c.conn == conn {
				clients = append(clients[:i], clients[i+1:]...)

				// Broadcast message to all connected clients including the sender
				userLeftMsg := message{Message: fmt.Sprintf("%s has left the chat", username)}

				for _, c := range clients {
					err = c.conn.WriteJSON(userLeftMsg)
					if err != nil {
						log.Println(err)
						break
					}
				}

				logMessage(username, userLeftMsg.Message)
			}
		}
	})

	// run the server
	go func() {
		err := r.Run(":" + strconv.Itoa(serverPort))
		if err != nil {
			log.Fatal(err)
		}
	}()

	// print the port being used
	log.Printf("Server started on port %d\n", serverPort)

	// Wait for SIGINT or SIGTERM signal
	waitForSignal()
}

func waitForSignal() {
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)
	<-signalCh
	log.Println("Shutdown signal received, exiting...")
}

func logMessage(username string, message string) {
	log.Printf("Received message from %s: %s\n", username, message)
	f, errFile := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if errFile != nil {
		log.Println(errFile)
		return
	}

	// get a timestamp
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	// write the message to the log file
	_, err := f.WriteString(fmt.Sprintf("%s - %s: %s\n", timestamp, username, message))
	if err != nil {
		log.Println(err)
	}
	f.Close()
}
