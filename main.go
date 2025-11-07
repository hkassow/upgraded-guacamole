package main

import (
	"log"
	"net/http"
	"go-guacamole/handlers"
)

func main() {
	http.HandleFunc("/hello/", handlers.HelloHandler)
	http.HandleFunc("/recipes", handlers.RecipesHandler) 

	// ~~~ frontend ~~~
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
            http.ServeFile(w, r, "index.html")
    	})

    	// static assets
    	fs := http.FileServer(http.Dir("static"))
    	http.Handle("/static/", http.StripPrefix("/static/", fs))


	log.Println("Server starting on :8443...")
	if err := http.ListenAndServeTLS(":8443", "certs/cert.pem", "certs/key.pem", nil); err != nil {
		log.Fatal(err)
	}
}
