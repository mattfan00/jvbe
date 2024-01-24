package main

import (
	"flag"
	"fmt"
	"github/mattfan00/jvbe/config"
	"github/mattfan00/jvbe/event"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	configPath := flag.String("c", "./config.yaml", "path to config file")
	conf, err := config.ReadFile(*configPath)
	if err != nil {
		panic(err)
	}

	db, err := sqlx.Connect("sqlite3", conf.DbConn)
	if err != nil {
		panic(err)
	}

	eventStore := event.NewStore(db)
	eventService := event.NewService(eventStore)

	r := chi.NewRouter()

	publicFileServer := http.FileServer(http.Dir("./ui/public"))
	r.Handle("/public/*", http.StripPrefix("/public/", publicFileServer))

	eventService.Routes(r)

	http.ListenAndServe(fmt.Sprintf(":%d", conf.Port), r)
}
