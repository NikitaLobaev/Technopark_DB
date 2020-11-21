package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/lib/pq"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Error struct {
	Message string `json:"message"`
}

type Profile struct {
	Id       int64  `json:"-"`
	Nickname string `json:"nickname"`
	About    string `json:"about"`
	Email    string `json:"email"`
	Fullname string `json:"fullname"`
}

type Forum struct {
	Slug            string `json:"slug"`
	Title           string `json:"title"`
	Profile         int64  `json:"-"`
	ProfileNickname string `json:"user"`
	Threads         int32  `json:"threads"`
	Posts           int64  `json:"posts"`
}

type Thread struct {
	Id              int32         `json:"id"`
	Profile         int64         `json:"-"`
	ProfileNickname string        `json:"author"`
	Created         time.Time     `json:"created"`
	ForumSlug       string        `json:"forum"`
	Message         string        `json:"message"`
	Slug            string        `json:"slug"`
	Title           string        `json:"title"`
	Votes           int64         `json:"votes"`
	VotesNull       sql.NullInt64 `json:"-"`
}

type Post struct {
	Id              int64     `json:"id"`
	Profile         int64     `json:"-"`
	ProfileNickname string    `json:"author"`
	Created         time.Time `json:"created"`
	ForumSlug       string    `json:"forum"`
	IsEdited        bool      `json:"isEdited"`
	Message         string    `json:"message"`
	Parent          int64     `json:"parent,omitempty"`
	Posts           []int64   `json:"-"`
	Thread          int32     `json:"thread"`
}

type PostFull struct {
	Profile *Profile `json:"author,omitempty"`
	Forum   *Forum   `json:"forum,omitempty"`
	Post    Post     `json:"post"`
	Thread  *Thread  `json:"thread,omitempty"`
}

type Vote struct {
	Profile         int64  `json:"-"`
	ProfileNickname string `json:"nickname"`
	Thread          int32  `json:"-"`
	Voice           int64  `json:"voice"`
}

type Status struct {
	Forum  int64 `json:"forum"`
	Post   int64 `json:"post"`
	Thread int64 `json:"thread"`
	User   int64 `json:"user"`
}

func ForumCreate(context echo.Context) error {
	var forum Forum
	if err := context.Bind(&forum); err != nil {
		panic(err)
	}

	if err := DBConnection.QueryRow("SELECT profiles.id, profiles.nickname FROM profiles WHERE profiles.nickname = $1;",
		forum.ProfileNickname).Scan(&forum.Profile, &forum.ProfileNickname); err == sql.ErrNoRows {
		return context.JSON(http.StatusNotFound, Error{
			Message: "Can't find user with nickname " + forum.ProfileNickname,
		})
	}

	if err := DBConnection.QueryRow("SELECT forums.slug, forums.title, profiles.nickname FROM forums JOIN profiles ON forums.profile_id = profiles.id WHERE forums.slug = $1;",
		forum.Slug).Scan(&forum.Slug, &forum.Title, &forum.Profile); err != sql.ErrNoRows {
		return context.JSON(http.StatusConflict, forum)
	}

	if _, err := DBConnection.Exec("INSERT INTO forums (slug, title, profile_id) VALUES ($1, $2, $3);",
		forum.Slug, forum.Title, forum.Profile); err != nil {
		panic(err)
	}

	return context.JSON(http.StatusCreated, forum)
}

func ThreadCreate(context echo.Context) error {
	var thread Thread
	if err := context.Bind(&thread); err != nil {
		panic(err)
	}
	thread.ForumSlug = context.Param("slug_")

	if err := DBConnection.QueryRow("SELECT profiles.id, profiles.nickname FROM profiles WHERE profiles.nickname = $1;",
		thread.ProfileNickname).Scan(&thread.Profile, &thread.ProfileNickname); err == sql.ErrNoRows {
		return context.JSON(http.StatusNotFound, Error{
			Message: "Can't find user with nickname " + thread.ProfileNickname,
		})
	}

	if err := DBConnection.QueryRow("SELECT forums.slug FROM forums WHERE forums.slug = $1;",
		thread.ForumSlug).Scan(&thread.ForumSlug); err == sql.ErrNoRows {
		return context.JSON(http.StatusNotFound, Error{
			Message: "Can't find forum with slug " + thread.ForumSlug,
		})
	}

	if thread.Slug != "" {
		if err := DBConnection.QueryRow("SELECT threads.id, profiles.nickname, threads.created, threads.forum_slug, threads.message, threads.slug, threads.title FROM threads JOIN profiles ON threads.profile_id = profiles.id WHERE threads.slug = $1;",
			thread.Slug).Scan(&thread.Id, &thread.ProfileNickname, &thread.Created, &thread.ForumSlug, &thread.Message,
			&thread.Slug, &thread.Title); err != sql.ErrNoRows {
			return context.JSON(http.StatusConflict, thread)
		}
	}

	if err := DBConnection.QueryRow("INSERT INTO threads (id, profile_id, created, forum_slug, message, slug, title) VALUES (default, $1, $2, $3, $4, $5, $6) RETURNING id;",
		thread.Profile, thread.Created, thread.ForumSlug, thread.Message, thread.Slug, thread.Title).Scan(&thread.Id); err != nil {
		panic(err)
	}

	return context.JSON(http.StatusCreated, thread)
}

func ForumGetDetails(context echo.Context) error {
	var forum Forum
	forum.Slug = context.Param("slug")
	if err := DBConnection.QueryRow("SELECT forums.slug, forums.title, profiles.nickname, (SELECT COUNT(*) FROM threads WHERE threads.forum_slug = $1), (SELECT COUNT(*) FROM posts WHERE posts.thread_id IN (SELECT threads.id FROM threads WHERE threads.forum_slug = $1)) FROM forums JOIN profiles ON forums.profile_id = profiles.id WHERE forums.slug = $1;",
		forum.Slug).Scan(&forum.Slug, &forum.Title, &forum.ProfileNickname, &forum.Threads, &forum.Posts); err == sql.ErrNoRows {
		return context.JSON(http.StatusNotFound, Error{
			Message: "Can't find forum with slug " + forum.Slug,
		})
	}

	return context.JSON(http.StatusOK, forum)
}

func ForumGetThreads(context echo.Context) error {
	limit := context.QueryParam("limit")
	if limit == "" {
		limit = "NULL"
	}

	var forum Forum
	forum.Slug = context.Param("slug")
	if err := DBConnection.QueryRow("SELECT forums.slug FROM forums WHERE forums.slug = $1;",
		forum.Slug).Scan(&forum.Slug); err == sql.ErrNoRows {
		return context.JSON(http.StatusNotFound, Error{
			Message: "Can't find forum with slug " + forum.Slug,
		})
	}

	var rows *sql.Rows
	var err error
	since := context.QueryParam("since")
	if context.QueryParam("desc") != "true" {
		if since == "" {
			since = "-infinity"
		}
		rows, err = DBConnection.Query("SELECT threads.id, profiles.nickname, threads.created, threads.message, threads.slug, threads.title FROM threads JOIN profiles ON threads.profile_id = profiles.id WHERE threads.forum_slug = $1 AND threads.created >= $2 ORDER BY threads.created LIMIT $3;",
			forum.Slug, since, limit)
	} else {
		if since == "" {
			since = "infinity"
		}
		rows, err = DBConnection.Query("SELECT threads.id, profiles.nickname, threads.created, threads.message, threads.slug, threads.title FROM threads JOIN profiles ON threads.profile_id = profiles.id WHERE threads.forum_slug = $1 AND threads.created <= $2 ORDER BY threads.created DESC LIMIT $3;",
			forum.Slug, since, limit)
	}
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			panic(err)
		}
	}()

	var threads = make([]Thread, 0)
	for rows.Next() {
		var thread Thread
		if err := rows.Scan(&thread.Id, &thread.ProfileNickname, &thread.Created, &thread.Message, &thread.Slug, &thread.Title); err != nil {
			panic(err)
		}
		thread.ForumSlug = forum.Slug
		threads = append(threads, thread)
	}

	return context.JSON(http.StatusOK, threads)
}

func ForumGetUsers(context echo.Context) error {
	limit := context.QueryParam("limit")
	if limit == "" {
		limit = "NULL"
	}

	var forum Forum
	forum.Slug = context.Param("slug")
	if err := DBConnection.QueryRow("SELECT forums.slug FROM forums WHERE forums.slug = $1;",
		forum.Slug).Scan(&forum.Slug); err == sql.ErrNoRows {
		return context.JSON(http.StatusNotFound, Error{
			Message: "Can't find forum with slug " + forum.Slug,
		})
	}

	var profiles = make([]Profile, 0)
	var rows *sql.Rows
	var err error
	since := context.QueryParam("since")
	if context.QueryParam("desc") != "true" {
		if since == "" {
			rows, err = DBConnection.Query(fmt.Sprintf("SELECT profiles.id, profiles.nickname COLLATE \"C\" AS nickname, profiles.about, profiles.email, profiles.fullname FROM threads JOIN profiles ON threads.profile_id = profiles.id WHERE threads.forum_slug = $1 UNION SELECT profiles.id, profiles.nickname COLLATE \"C\" AS nickname, profiles.about, profiles.email, profiles.fullname FROM posts JOIN profiles ON posts.profile_id = profiles.id WHERE posts.thread_id IN (SELECT threads.id FROM threads WHERE threads.forum_slug = $1) ORDER BY nickname LIMIT %s;", limit),
				forum.Slug)
		} else {
			rows, err = DBConnection.Query(fmt.Sprintf("SELECT profiles.id, profiles.nickname COLLATE \"C\" AS nickname, profiles.about, profiles.email, profiles.fullname FROM threads JOIN profiles ON threads.profile_id = profiles.id WHERE threads.forum_slug = $1 AND profiles.nickname > $2 COLLATE \"C\" UNION SELECT profiles.id, profiles.nickname COLLATE \"C\" AS nickname, profiles.about, profiles.email, profiles.fullname FROM posts JOIN profiles ON posts.profile_id = profiles.id WHERE profiles.nickname > $2 COLLATE \"C\" AND posts.thread_id IN (SELECT threads.id FROM threads WHERE threads.forum_slug = $1) ORDER BY nickname LIMIT %s;", limit),
				forum.Slug, since)
		}
	} else {
		if since == "" {
			rows, err = DBConnection.Query(fmt.Sprintf("SELECT profiles.id, profiles.nickname COLLATE \"C\" AS nickname, profiles.about, profiles.email, profiles.fullname FROM threads JOIN profiles ON threads.profile_id = profiles.id WHERE threads.forum_slug = $1 UNION SELECT profiles.id, profiles.nickname COLLATE \"C\" AS nickname, profiles.about, profiles.email, profiles.fullname FROM posts JOIN profiles ON posts.profile_id = profiles.id WHERE posts.thread_id IN (SELECT threads.id FROM threads WHERE threads.forum_slug = $1) ORDER BY nickname DESC LIMIT %s;", limit),
				forum.Slug)
		} else {
			rows, err = DBConnection.Query(fmt.Sprintf("SELECT profiles.id, profiles.nickname COLLATE \"C\" AS nickname, profiles.about, profiles.email, profiles.fullname FROM threads JOIN profiles ON threads.profile_id = profiles.id WHERE threads.forum_slug = $1 AND profiles.nickname < $2 COLLATE \"C\" UNION SELECT profiles.id, profiles.nickname COLLATE \"C\" AS nickname, profiles.about, profiles.email, profiles.fullname FROM posts JOIN profiles ON posts.profile_id = profiles.id WHERE profiles.nickname < $2 COLLATE \"C\" AND posts.thread_id IN (SELECT threads.id FROM threads WHERE threads.forum_slug = $1) ORDER BY nickname DESC LIMIT %s;", limit),
				forum.Slug, since)
		}
	}
	if err != nil {
		return context.JSON(http.StatusOK, profiles)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			panic(err)
		}
	}()

	for rows.Next() {
		var profile Profile
		if err := rows.Scan(&profile.Id, &profile.Nickname, &profile.About, &profile.Email, &profile.Fullname); err != nil {
			panic(err)
		}
		profiles = append(profiles, profile)
	}

	return context.JSON(http.StatusOK, profiles)
}

func PostGetDetails(context echo.Context) error {
	/*postFull := PostFull{
		Forum: &Forum{},
		Thread: &Thread{},
		Profile: &Profile{},
	}*/
	var postFull PostFull
	id := context.Param("id")
	postFull.Post.Id, _ = strconv.ParseInt(id, 10, 64)
	var user, forum, thread bool
	for _, related := range strings.Split(context.QueryParam("related"), ",") {
		switch related {
		case "user":
			user = true
			break
		case "forum":
			forum = true
			break
		case "thread":
			thread = true
			break
		}
	}

	if err := DBConnection.QueryRow("SELECT posts.id, profiles.nickname, posts.created, posts.is_edited, posts.message, posts.thread_id, threads.forum_slug FROM posts JOIN profiles ON posts.profile_id = profiles.id JOIN threads ON posts.thread_id = threads.id WHERE posts.id = $1;", postFull.Post.Id).
		Scan(&postFull.Post.Id, &postFull.Post.ProfileNickname, &postFull.Post.Created, &postFull.Post.IsEdited,
			&postFull.Post.Message, &postFull.Post.Thread, &postFull.Post.ForumSlug); err != nil {
		return context.JSON(http.StatusNotFound, Error{
			Message: "Can't find post with id " + id,
		})
	}

	if user {
		postFull.Profile = &Profile{}
		if err := DBConnection.QueryRow("SELECT profiles.nickname, profiles.about, profiles.email, profiles.fullname FROM posts JOIN profiles ON posts.profile_id = profiles.id WHERE posts.id = $1;", postFull.Post.Id).
			Scan(&postFull.Profile.Nickname, &postFull.Profile.About, &postFull.Profile.Email,
				&postFull.Profile.Fullname); err != nil {
			fmt.Println(err)
			//panic(err)
		}
	}

	if forum {
		postFull.Forum = &Forum{}
		if err := DBConnection.QueryRow("SELECT forums.slug, forums.title, profiles.nickname, (SELECT COUNT(*) FROM threads WHERE threads.forum_slug = forums.slug), (SELECT COUNT(*) FROM posts WHERE posts.thread_id IN (SELECT threads.id FROM threads WHERE threads.forum_slug = forums.slug)) FROM posts JOIN threads ON posts.thread_id = threads.id JOIN forums ON threads.forum_slug = forums.slug JOIN profiles ON forums.profile_id = profiles.id WHERE posts.id = $1;", postFull.Post.Id).
			Scan(&postFull.Forum.Slug, &postFull.Forum.Title, &postFull.Forum.ProfileNickname, &postFull.Forum.Threads,
				&postFull.Forum.Posts); err != nil {
			fmt.Println(err)
			//panic(err)
		}
	}

	if thread {
		postFull.Thread = &Thread{}
		if err := DBConnection.QueryRow("SELECT threads.id, profiles.nickname, threads.created, threads.forum_slug, threads.message, threads.slug, threads.title FROM posts JOIN threads ON posts.thread_id = threads.id JOIN profiles ON threads.profile_id = profiles.id WHERE posts.id = $1;", postFull.Post.Id).
			Scan(&postFull.Thread.Id, &postFull.Thread.ProfileNickname, &postFull.Thread.Created,
				&postFull.Thread.ForumSlug, &postFull.Thread.Message, &postFull.Thread.Slug, &postFull.Thread.Title); err != nil {
			fmt.Println(err)
			//panic(err)
		}
	}
	fmt.Println("related =", context.QueryParam("related"))

	return context.JSON(http.StatusOK, postFull)
}

func PostUpdateDetails(context echo.Context) error {
	var post Post
	id := context.Param("id")
	post.Id, _ = strconv.ParseInt(id, 10, 64)
	if err := DBConnection.QueryRow("SELECT posts.id, profiles.nickname, posts.created, posts.is_edited, posts.message, posts.thread_id, threads.forum_slug FROM posts JOIN profiles ON posts.profile_id = profiles.id JOIN threads ON posts.thread_id = threads.id WHERE posts.id = $1;", post.Id).
		Scan(&post.Id, &post.ProfileNickname, &post.Created, &post.IsEdited, &post.Message, &post.Thread,
			&post.ForumSlug); err != nil {
		return context.JSON(http.StatusNotFound, Error{
			Message: "Can't find post with id " + id,
		})
	}

	updatedPost := post
	if err := context.Bind(&updatedPost); err != nil {
		panic(err)
	}

	if updatedPost.Message != post.Message {
		if _, err := DBConnection.Exec("UPDATE posts SET message = $1, is_edited = true WHERE id = $2;",
			updatedPost.Message, updatedPost.Id); err != nil {
			panic(err)
		}
		updatedPost.IsEdited = true
	}

	return context.JSON(http.StatusOK, updatedPost)
}

func ServiceClear(context echo.Context) error {
	if _, err := DBConnection.Exec("TRUNCATE TABLE profiles CASCADE;"); err != nil {
		panic(err)
	}
	return context.JSON(http.StatusOK, nil)
}

func ServiceStatus(context echo.Context) error {
	var status Status
	if err := DBConnection.QueryRow("SELECT COUNT(*) FROM forums;").Scan(&status.Forum); err != nil {
		panic(err)
	}
	if err := DBConnection.QueryRow("SELECT COUNT(*) FROM posts;").Scan(&status.Post); err != nil {
		panic(err)
	}
	if err := DBConnection.QueryRow("SELECT COUNT(*) FROM threads;").Scan(&status.Thread); err != nil {
		panic(err)
	}
	if err := DBConnection.QueryRow("SELECT COUNT(*) FROM profiles;").Scan(&status.User); err != nil {
		panic(err)
	}

	return context.JSON(http.StatusOK, status)
}

func PostCreate(context echo.Context) error {
	var thread Thread
	slugOrId := context.Param("slug_or_id")
	if err := DBConnection.QueryRow("SELECT threads.id, threads.slug, threads.forum_slug FROM threads WHERE threads.slug = $1 OR threads.id::citext = $1;",
		slugOrId).Scan(&thread.Id, &thread.Slug, &thread.ForumSlug); err != nil {
		return context.JSON(http.StatusNotFound, Error{
			Message: "Can't find thread with slug or id " + slugOrId,
		})
	}

	var posts []*Post
	result, err := ioutil.ReadAll(context.Request().Body)
	if err != nil {
		panic(err)
	}

	err = json.Unmarshal(result, &posts) //TODO
	if err != nil {
		panic(err)
	}

	if len(posts) == 0 {
		return context.JSON(http.StatusCreated, posts)
	}

	tx, err := DBConnection.Begin()
	if err != nil {
		panic(err)
	}

	for _, post := range posts {
		if post.Parent == 0 {
			if err = tx.QueryRow("INSERT INTO posts (profile_id, created, message, thread_id) (SELECT profiles.id, $1, $2, $3 FROM profiles WHERE profiles.nickname = $4) RETURNING posts.id;",
				post.Created, post.Message, thread.Id, post.ProfileNickname).Scan(&post.Id); err != nil {
				if err := tx.Rollback(); err != nil {
					panic(err)
				}
				return context.JSON(http.StatusNotFound, Error{
					Message: "Can't find post author by nickname " + post.ProfileNickname,
				})
			}
		} else {
			if err = tx.QueryRow("SELECT posts.posts FROM posts WHERE posts.id = $1 AND posts.thread_id = $2;",
				post.Parent, thread.Id).Scan(pq.Array(&post.Posts)); err != nil {
				if err := tx.Rollback(); err != nil {
					panic(err)
				}
				return context.JSON(http.StatusConflict, Error{
					Message: "One of parent posts doesn't exists or it was created in another thread",
				})
			}
			if err = tx.QueryRow("INSERT INTO posts (profile_id, created, message, posts, thread_id) (SELECT profiles.id, $1, $2, $3, $4 FROM profiles WHERE profiles.nickname = $5) RETURNING posts.id;",
				post.Created, post.Message, pq.Array(post.Posts), thread.Id, post.ProfileNickname).Scan(&post.Id); err != nil {
				if err := tx.Rollback(); err != nil {
					panic(err)
				}
				return context.JSON(http.StatusNotFound, Error{
					Message: "Can't find post author by nickname " + post.ProfileNickname,
				})
			}
		}
		post.ForumSlug = thread.ForumSlug
		post.Thread = thread.Id
	}

	if err = tx.Commit(); err != nil {
		panic(err)
	}

	return context.JSON(http.StatusCreated, posts)
}

func ThreadGetDetails(context echo.Context) error {
	var thread Thread
	slugOrId := context.Param("slug_or_id")
	if _, err := strconv.Atoi(slugOrId); err == nil {
		if err := DBConnection.QueryRow("SELECT threads.id, profiles.nickname, threads.created, threads.forum_slug, threads.message, threads.slug, threads.title, (SELECT SUM(votes.voice) FROM votes WHERE votes.thread_id = $1) FROM threads JOIN profiles ON threads.profile_id = profiles.id WHERE threads.id = $1;",
			slugOrId).Scan(&thread.Id, &thread.ProfileNickname, &thread.Created, &thread.ForumSlug, &thread.Message,
			&thread.Slug, &thread.Title, &thread.VotesNull); err != nil {
			return context.JSON(http.StatusNotFound, Error{
				Message: "Can't find thread with id " + slugOrId,
			})
		}
	} else {
		if err := DBConnection.QueryRow("SELECT threads.id, profiles.nickname, threads.created, threads.forum_slug, threads.message, threads.slug, threads.title, (SELECT SUM(votes.voice) FROM votes WHERE votes.thread_id = (SELECT threads.id FROM threads WHERE threads.slug = $1)) FROM threads JOIN profiles ON threads.profile_id = profiles.id WHERE threads.slug = $1;",
			slugOrId).Scan(&thread.Id, &thread.ProfileNickname, &thread.Created, &thread.ForumSlug, &thread.Message,
			&thread.Slug, &thread.Title, &thread.VotesNull); err != nil {
			return context.JSON(http.StatusNotFound, Error{
				Message: "Can't find thread with slug " + slugOrId,
			})
		}
	}
	if thread.VotesNull.Valid {
		thread.Votes = thread.VotesNull.Int64
	}

	return context.JSON(http.StatusOK, thread)
}

func ThreadUpdateDetails(context echo.Context) error {
	var thread Thread
	slugOrId := context.Param("slug_or_id")
	if _, err := strconv.Atoi(slugOrId); err == nil {
		if err := DBConnection.QueryRow("SELECT threads.id, profiles.nickname, threads.created, threads.forum_slug, threads.message, threads.slug, threads.title, (SELECT SUM(votes.voice) FROM votes WHERE votes.thread_id = $1) FROM threads JOIN profiles ON threads.profile_id = profiles.id WHERE threads.id = $1;",
			slugOrId).Scan(&thread.Id, &thread.ProfileNickname, &thread.Created, &thread.ForumSlug, &thread.Message,
			&thread.Slug, &thread.Title, &thread.VotesNull); err != nil {
			return context.JSON(http.StatusNotFound, Error{
				Message: "Can't find thread with id " + slugOrId,
			})
		}
	} else {
		if err := DBConnection.QueryRow("SELECT threads.id, profiles.nickname, threads.created, threads.forum_slug, threads.message, threads.slug, threads.title, (SELECT SUM(votes.voice) FROM votes WHERE votes.thread_id = (SELECT threads.id FROM threads WHERE threads.slug = $1)) FROM threads JOIN profiles ON threads.profile_id = profiles.id WHERE threads.slug = $1;",
			slugOrId).Scan(&thread.Id, &thread.ProfileNickname, &thread.Created, &thread.ForumSlug, &thread.Message,
			&thread.Slug, &thread.Title, &thread.VotesNull); err != nil {
			return context.JSON(http.StatusNotFound, Error{
				Message: "Can't find thread with slug " + slugOrId,
			})
		}
	}
	if thread.VotesNull.Valid {
		thread.Votes = thread.VotesNull.Int64
	}

	if err := context.Bind(&thread); err != nil {
		panic(err)
	}
	if _, err := DBConnection.Exec("UPDATE threads SET message = $2, title = $3 WHERE id = $1;",
		thread.Id, thread.Message, thread.Title); err != nil {
		panic(err)
	}

	return context.JSON(http.StatusOK, thread)
}

func ThreadGetPosts(context echo.Context) error {
	limit := context.QueryParam("limit")
	if limit == "" {
		limit = "NULL"
	}

	since := context.QueryParam("since")

	sort := context.QueryParam("sort")
	if sort == "" {
		sort = "NULL"
	}

	var desc string
	if context.QueryParam("desc") == "true" {
		desc = "DESC"
	}

	var thread Thread
	slugOrId := context.Param("slug_or_id")
	if err := DBConnection.QueryRow("SELECT threads.id, forums.slug FROM threads JOIN forums ON threads.forum_slug = forums.slug WHERE threads.slug = $1 OR threads.id::citext = $1;",
		slugOrId).Scan(&thread.Id, &thread.ForumSlug); err == sql.ErrNoRows {
		return context.JSON(http.StatusNotFound, Error{
			Message: "Can't find thread with slug or id " + slugOrId,
		})
	}

	var rows *sql.Rows
	var err error
	switch context.QueryParam("sort") {
	case "tree":
		if since == "" {
			rows, err = DBConnection.Query(fmt.Sprintf("SELECT posts.id, profiles.nickname, posts.created, posts.is_edited, posts.message, posts.posts FROM posts JOIN profiles ON posts.profile_id = profiles.id WHERE posts.thread_id = $1 ORDER BY posts.posts %s, posts.created, posts.id LIMIT $2;", desc),
				thread.Id, limit)
		} else {
			if desc == "" {
				rows, err = DBConnection.Query(fmt.Sprintf("SELECT posts.id, profiles.nickname, posts.created, posts.is_edited, posts.message, posts.posts FROM posts JOIN profiles ON posts.profile_id = profiles.id WHERE posts.posts > (SELECT posts.posts FROM posts WHERE posts.id = $2) AND posts.thread_id = $1 ORDER BY posts.posts %s, posts.created, posts.id LIMIT $3;", desc),
					thread.Id, since, limit)
			} else {
				rows, err = DBConnection.Query(fmt.Sprintf("SELECT posts.id, profiles.nickname, posts.created, posts.is_edited, posts.message, posts.posts FROM posts JOIN profiles ON posts.profile_id = profiles.id WHERE posts.posts < (SELECT posts.posts FROM posts WHERE posts.id = $2) AND posts.thread_id = $1 ORDER BY posts.posts %s, posts.created, posts.id LIMIT $3;", desc),
					thread.Id, since, limit)
			}
		}
		break
	case "parent_tree":
		if since == "" {
			rows, err = DBConnection.Query(fmt.Sprintf("SELECT posts.id, profiles.nickname, posts.created, posts.is_edited, posts.message, posts.posts FROM posts JOIN profiles ON posts.profile_id = profiles.id WHERE posts.posts[1] IN (SELECT DISTINCT posts.posts[1] FROM posts WHERE posts.thread_id = $1 AND array_length(posts.posts, 1) = 1 ORDER BY posts.posts[1] %s LIMIT $2) ORDER BY posts.posts[1] %s, posts.posts, posts.created, posts.id;", desc, desc),
				thread.Id, limit)
		} else {
			if desc == "" {
				rows, err = DBConnection.Query(fmt.Sprintf("SELECT posts.id, profiles.nickname, posts.created, posts.is_edited, posts.message, posts.posts FROM posts JOIN profiles ON posts.profile_id = profiles.id WHERE posts.posts[1] IN (SELECT DISTINCT posts.posts[1] FROM posts WHERE posts.posts[1] > (SELECT posts.posts[1] FROM posts WHERE posts.id = $2) AND posts.thread_id = $1 AND array_length(posts.posts, 1) = 1 LIMIT $3) ORDER BY posts.posts[1] %s, posts.posts, posts.created, posts.id;", desc),
					thread.Id, since, limit)
			} else {
				rows, err = DBConnection.Query(fmt.Sprintf("SELECT posts.id, profiles.nickname, posts.created, posts.is_edited, posts.message, posts.posts FROM posts JOIN profiles ON posts.profile_id = profiles.id WHERE posts.posts[1] IN (SELECT DISTINCT posts.posts[1] FROM posts WHERE posts.posts[1] < (SELECT posts.posts[1] FROM posts WHERE posts.id = $2) AND posts.thread_id = $1 AND array_length(posts.posts, 1) = 1 LIMIT $3) ORDER BY posts.posts[1] %s, posts.posts, posts.created, posts.id;", desc),
					thread.Id, since, limit)
			}
		}
		break
	default: //flat
		if since == "" {
			rows, err = DBConnection.Query(fmt.Sprintf("SELECT posts.id, profiles.nickname, posts.created, posts.is_edited, posts.message, posts.posts FROM posts JOIN profiles ON posts.profile_id = profiles.id WHERE posts.thread_id = $1 ORDER BY posts.created %s, posts.id %s LIMIT $2;", desc, desc),
				thread.Id, limit)
		} else {
			if desc == "" {
				rows, err = DBConnection.Query(fmt.Sprintf("SELECT posts.id, profiles.nickname, posts.created, posts.is_edited, posts.message, posts.posts FROM posts JOIN profiles ON posts.profile_id = profiles.id WHERE posts.thread_id = $1 AND posts.id > $2 ORDER BY posts.created %s, posts.id %s LIMIT $3;", desc, desc),
					thread.Id, since, limit)
			} else {
				rows, err = DBConnection.Query(fmt.Sprintf("SELECT posts.id, profiles.nickname, posts.created, posts.is_edited, posts.message, posts.posts FROM posts JOIN profiles ON posts.profile_id = profiles.id WHERE posts.thread_id = $1 AND posts.id < $2 ORDER BY posts.created %s, posts.id %s LIMIT $3;", desc, desc),
					thread.Id, since, limit)
			}
		}
		break
	}
	if err != nil {
		//panic(err)
		fmt.Println(err)
		return nil
	}
	defer func() {
		if err := rows.Close(); err != nil {
			panic(err)
		}
	}()

	var posts = make([]Post, 0)
	for rows.Next() {
		var post Post
		if err := rows.Scan(&post.Id, &post.ProfileNickname, &post.Created, &post.IsEdited, &post.Message,
			pq.Array(&post.Posts)); err != nil {
			panic(err)
		}
		post.ForumSlug = thread.ForumSlug
		if len(post.Posts) > 1 {
			post.Parent = post.Posts[len(post.Posts)-2]
		}
		post.Thread = thread.Id
		posts = append(posts, post)
	}

	return context.JSON(http.StatusOK, posts)
}

func ThreadVote(context echo.Context) error {
	var vote Vote
	if err := context.Bind(&vote); err != nil {
		panic(err)
	}
	if err := DBConnection.QueryRow("SELECT profiles.id FROM profiles WHERE profiles.nickname = $1;",
		vote.ProfileNickname).Scan(&vote.Profile); err == sql.ErrNoRows {
		return context.JSON(http.StatusNotFound, Error{
			Message: "Can't find user with nickname " + vote.ProfileNickname,
		})
	}

	var thread Thread
	slugOrId := context.Param("slug_or_id")
	if _, err := strconv.Atoi(slugOrId); err == nil {
		if err := DBConnection.QueryRow("SELECT threads.id, profiles.nickname, threads.created, threads.forum_slug, threads.message, threads.slug, threads.title, (SELECT SUM(votes.voice) FROM votes WHERE votes.thread_id = $1) FROM threads JOIN profiles ON threads.profile_id = profiles.id WHERE threads.id = $1;",
			slugOrId).Scan(&thread.Id, &thread.ProfileNickname, &thread.Created, &thread.ForumSlug, &thread.Message,
			&thread.Slug, &thread.Title, &thread.VotesNull); err != nil {
			fmt.Println(err)
			return context.JSON(http.StatusNotFound, Error{
				Message: "Can't find thread with id " + slugOrId,
			})
		}
	} else {
		if err := DBConnection.QueryRow("SELECT threads.id, profiles.nickname, threads.created, threads.forum_slug, threads.message, threads.slug, threads.title, (SELECT SUM(votes.voice) FROM votes WHERE votes.thread_id = (SELECT threads.id FROM threads WHERE threads.slug = $1)) FROM threads JOIN profiles ON threads.profile_id = profiles.id WHERE threads.slug = $1;",
			slugOrId).Scan(&thread.Id, &thread.ProfileNickname, &thread.Created, &thread.ForumSlug, &thread.Message,
			&thread.Slug, &thread.Title, &thread.VotesNull); err != nil {
			fmt.Println(err)
			return context.JSON(http.StatusNotFound, Error{
				Message: "Can't find thread with slug " + slugOrId,
			})
		}
	}
	if thread.VotesNull.Valid { //TODO: убрать вообще суммирование здесь...
		thread.Votes = thread.VotesNull.Int64
	}
	vote.Thread = thread.Id

	if _, err := DBConnection.Exec("INSERT INTO votes (profile_id, thread_id, voice) VALUES ($1, $2, $3) ON CONFLICT (profile_id, thread_id) DO UPDATE SET voice = $3;",
		vote.Profile, vote.Thread, vote.Voice); err != nil {
		panic(err)
	}

	if err := DBConnection.QueryRow("SELECT SUM(votes.voice) FROM votes WHERE votes.thread_id = $1;", thread.Id).
		Scan(&thread.VotesNull); err != nil {
		fmt.Println(err)
		return context.JSON(http.StatusNotFound, Error{
			Message: "Can't find thread with slug " + slugOrId,
		})
	}
	if thread.VotesNull.Valid {
		thread.Votes = thread.VotesNull.Int64
	}

	return context.JSON(http.StatusOK, thread)
}

func UserCreate(context echo.Context) error {
	var profile Profile
	if err := context.Bind(&profile); err != nil {
		panic(err)
	}
	profile.Nickname = context.Param("nickname")

	rows, err := DBConnection.Query("SELECT profiles.id, profiles.nickname, profiles.about, profiles.email, profiles.fullname FROM profiles WHERE profiles.nickname = $1 OR profiles.email = $2;",
		profile.Nickname, profile.Email)
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			panic(err)
		}
	}()

	var existingProfiles []Profile
	for rows.Next() {
		var existingProfile Profile
		if err := rows.Scan(&existingProfile.Id, &existingProfile.Nickname, &existingProfile.About,
			&existingProfile.Email, &existingProfile.Fullname); err != nil {
			panic(err)
		}
		existingProfiles = append(existingProfiles, existingProfile)
	}

	if len(existingProfiles) > 0 {
		return context.JSON(http.StatusConflict, existingProfiles)
	}

	if _, err = DBConnection.Exec("INSERT INTO profiles (id, nickname, about, email, fullname) VALUES (default, $1, $2, $3, $4);",
		profile.Nickname, profile.About, profile.Email, profile.Fullname); err != nil {
		panic(err)
	}

	return context.JSON(http.StatusCreated, profile)
}

func UserGet(context echo.Context) error {
	var profile Profile
	if err := DBConnection.QueryRow("SELECT profiles.id, profiles.nickname, profiles.about, profiles.email, profiles.fullname FROM profiles WHERE profiles.nickname = $1;",
		context.Param("nickname")).Scan(&profile.Id, &profile.Nickname, &profile.About, &profile.Email, &profile.Fullname); err == sql.ErrNoRows {
		return context.JSON(http.StatusNotFound, Error{
			Message: "Can't find user with nickname " + profile.Nickname,
		})
	}

	return context.JSON(http.StatusOK, profile)
}

func UserUpdate(context echo.Context) error {
	var profile Profile
	profile.Nickname = context.Param("nickname")
	if err := DBConnection.QueryRow("SELECT profiles.id, profiles.nickname, profiles.about, profiles.email, profiles.fullname FROM profiles WHERE profiles.nickname = $1;",
		profile.Nickname).Scan(&profile.Id, &profile.Nickname, &profile.About, &profile.Email, &profile.Fullname); err == sql.ErrNoRows {
		return context.JSON(http.StatusNotFound, Error{
			Message: "Can't find user with nickname " + profile.Nickname,
		})
	}

	updatedProfile := profile
	if err := context.Bind(&updatedProfile); err != nil {
		panic(err)
	}

	err := DBConnection.QueryRow("SELECT profiles.nickname FROM profiles WHERE profiles.email = $1 AND profiles.nickname != $2;",
		updatedProfile.Email, profile.Nickname).Scan(&updatedProfile.Nickname)
	if err != sql.ErrNoRows {
		return context.JSON(http.StatusConflict, Error{
			Message: "This email is already registered by user " + updatedProfile.Nickname,
		})
	}

	if _, err := DBConnection.Exec("UPDATE profiles SET about = $2, email = $3, fullname = $4 WHERE id = $1;",
		updatedProfile.Id, updatedProfile.About, updatedProfile.Email, updatedProfile.Fullname); err != nil {
		panic(err)
	}

	return context.JSON(http.StatusOK, updatedProfile)
}
