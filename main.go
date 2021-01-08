package main

import (
	"database/sql"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	_ "github.com/lib/pq"
)

var DBConnection *sql.DB

func main() {
	var err error
	DBConnection, err = sql.Open("postgres", "host=localhost port=5432 user=forums_user password=forums_user dbname=forums sslmode=disable")
	defer func() {
		_ = DBConnection.Close()
	}()
	if err != nil {
		panic(err)
	}

	if err := DBConnection.Ping(); err != nil {
		panic(err)
	}

	e := echo.New() //TODO: возможно, echo не нужен

	e.POST("/api/forum/create", ForumCreate)

	e.POST("/api/forum/:slug_/create", ThreadCreate)

	e.GET("/api/forum/:slug/details", ForumGetOne)

	e.GET("/api/forum/:slug/threads", ForumGetThreads)

	e.GET("/api/forum/:slug/users", ForumGetUsers)

	e.GET("/api/post/:id/details", PostGetOne)

	e.POST("/api/post/:id/details", PostUpdate)

	e.POST("/api/service/clear", ServiceClear)

	e.GET("/api/service/status", ServiceStatus)

	e.POST("/api/thread/:slug_or_id/create", PostsCreate)

	e.GET("/api/thread/:slug_or_id/details", ThreadGetOne)

	e.POST("/api/thread/:slug_or_id/details", ThreadUpdate)

	e.GET("/api/thread/:slug_or_id/posts", ThreadGetPosts)

	e.POST("/api/thread/:slug_or_id/vote", ThreadVote)

	e.POST("/api/user/:nickname/create", UserCreate)

	e.GET("/api/user/:nickname/profile", UserGetOne)

	e.POST("/api/user/:nickname/profile", UserUpdate)

	e.Use(middleware.Logger())
	_ = e.Start("0.0.0.0:5000")
}
