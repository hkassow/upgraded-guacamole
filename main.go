package main

import (
	"log"
	"net/http"

	"go-guacamole/handlers"
	"go-guacamole/db"
)

func main() {
	// ~~~ backend ~~~
	http.HandleFunc("/hello/", handlers.HelloHandler)
	http.HandleFunc("/recipes", handlers.RecipesHandler) 

	// ~~~ frontend ~~~
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
            http.ServeFile(w, r, "index.html")
    	})

    	// ~~~ static assets ~~~
    	fs := http.FileServer(http.Dir("static"))
    	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// ~~~ db ~~~
	db.Connect()
	defer db.Close()

	db.RunMigrations(db.Pool)

	log.Println("Server starting on :8443...")
	if err := http.ListenAndServeTLS(":8443", "certs/cert.pem", "certs/key.pem", nil); err != nil {
		log.Fatal(err)
	}
}
