package main

import (
	"log"
	"net/http"
	"go-guacamole/handlers"
)

func main() {
	http.HandleFunc("/hello/", handlers.HelloHandler)

	log.Println("Server starting on :8443...")
	if err := http.ListenAndServeTLS(":8443", "certs/cert.pem", "certs/key.pem", nil); err != nil {
		log.Fatal(err)
	}
}
