package main

import (
	"database/sql"
	"github.com/labstack/echo/v4"
	_ "github.com/lib/pq"
	"log"
)

var DBConnection *sql.DB

func main() { //TODO: названия функций есть в документации...
	var err error
	DBConnection, err = sql.Open("postgres", "host=localhost port=5432 user=forums_user password=89269199046qwerty dbname=forums sslmode=disable")

	defer func() {
		_ = DBConnection.Close()
	}()

	if err != nil {
		log.Fatal(err)
	}

	if err := DBConnection.Ping(); err != nil {
		log.Fatal(err)
	}

	e := echo.New()

	e.POST("/api/forum/create", ForumCreate)

	e.POST("/api/forum/:slug_/create", ThreadCreate)

	e.GET("/api/forum/:slug/details", ForumGetDetails)

	e.GET("/api/forum/:slug/threads", ForumGetThreads)

	e.GET("/api/forum/:slug/users", ForumGetUsers)

	e.GET("/api/post/:id/details", PostGetDetails)

	e.POST("/api/post/:id/details", PostUpdateDetails)

	e.POST("/api/service/clear", ServiceClear)

	e.GET("/api/service/status", ServiceStatus)

	e.POST("/api/thread/:slug_or_id/create", PostCreate)

	e.GET("/api/thread/:slug_or_id/details", ThreadGetDetails)

	e.POST("/api/thread/:slug_or_id/details", ThreadUpdateDetails)

	e.GET("/api/thread/:slug_or_id/posts", ThreadGetPosts)

	e.POST("/api/thread/:slug_or_id/vote", ThreadVote)

	e.POST("/api/user/:nickname/create", UserCreate)

	e.GET("/api/user/:nickname/profile", UserGet)

	e.POST("/api/user/:nickname/profile", UserUpdate)

	log.Fatal(e.Start("localhost:5000"))
}
