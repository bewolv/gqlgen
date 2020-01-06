package main

import (
	"log"
	"net/http"

	todo "github.com/bewolv/gqlgen/example/config"
	"github.com/bewolv/gqlgen/graphql/handler"
	"github.com/bewolv/gqlgen/graphql/playground"
)

func main() {
	http.Handle("/", playground.Handler("Todo", "/query"))
	http.Handle("/query", handler.NewDefaultServer(
		todo.NewExecutableSchema(todo.New()),
	))
	log.Fatal(http.ListenAndServe(":8081", nil))
}
