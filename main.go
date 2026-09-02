package main

import (
	"log"
	"net/http"
	"context"

	"go-guacamole/handlers"
	"go-guacamole/db"
	"go-guacamole/lib"
)

func main() {
	// ~~~ backend ~~~
	http.HandleFunc("/hello/", handlers.HelloHandler)
	http.HandleFunc("/recipes", handlers.RecipesHandler) 
	http.HandleFunc("/ingredients", handlers.IngredientsHandler)
	http.HandleFunc("/cooking/", handlers.CookingHandler)

	http.HandleFunc("/auth/google/login", handlers.GoogleLogin)
	http.HandleFunc("/auth/google/callback", handlers.GoogleCallback)
	http.HandleFunc("/auth/me", handlers.MeHandler)

	http.HandleFunc("/users/follow-new-user", handlers.FollowNewUser)

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

	// ~~~ workers ~~~
	log.Println("Starting Recipe Worker")
	lib.StartRecipeWorker()
	if err := lib.LoadUnparsedRecipeJobs(context.Background()); err != nil {
    	log.Println("Failed loading unparsed jobs:", err)
	}

	// ~~~ start init ~~~
	handlers.InitGoogle()

	// ~~~ server ~~~
	log.Println("Server starting on :8443...")
	if err := http.ListenAndServe(":8443", nil); err != nil {
		log.Fatal(err)
	}
}
