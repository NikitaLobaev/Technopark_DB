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
	Nickname string `json:"nickname"`
	About    string `json:"about"`
	Email    string `json:"email"`
	Fullname string `json:"fullname"`
}

type Forum struct {
	Slug            string `json:"slug"`
	Title           string `json:"title"`
	ProfileNickname string `json:"user"`
	Threads         int32  `json:"threads"`
	Posts           int64  `json:"posts"`
}

type Thread struct {
	Id              int32     `json:"id"`
	ProfileNickname string    `json:"author"`
	Created         time.Time `json:"created"`
	ForumSlug       string    `json:"forum"`
	Message         string    `json:"message"`
	Slug            string    `json:"slug"`
	Title           string    `json:"title"`
	Votes           int64     `json:"votes"`
}

type Post struct {
	Id              int64     `json:"id"`
	ProfileNickname string    `json:"author"`
	Created         time.Time `json:"created"`
	ForumSlug       string    `json:"forum"`
	IsEdited        bool      `json:"isEdited"`
	Message         string    `json:"message"`
	Parent          int64     `json:"parent,omitempty"`
	Thread          int32     `json:"thread"`
}

type PostFull struct {
	Profile *Profile `json:"author,omitempty"`
	Forum   *Forum   `json:"forum,omitempty"`
	Post    Post     `json:"post"`
	Thread  *Thread  `json:"thread,omitempty"`
}

type Vote struct {
	ProfileNickname string `json:"nickname"`
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

	if err := DBConnection.QueryRow("SELECT forum.slug, forum.title, forum.profile_nickname FROM forum WHERE forum.slug = $1;",
		forum.Slug).Scan(&forum.Slug, &forum.Title, &forum.ProfileNickname); err != sql.ErrNoRows {
		return context.JSON(http.StatusConflict, forum)
	}

	if err := DBConnection.QueryRow("INSERT INTO forum (slug, title, profile_nickname) SELECT $1, $2, profile.nickname FROM profile WHERE profile.nickname = $3 RETURNING forum.profile_nickname;",
		forum.Slug, forum.Title, forum.ProfileNickname).Scan(&forum.ProfileNickname); err != nil {
		return context.JSON(http.StatusNotFound, Error{
			Message: "Can't find user with nickname " + forum.ProfileNickname,
		})
	}

	/*if err := DBConnection.QueryRow("INSERT INTO forum (slug, title, profile_nickname) SELECT $1, $2, profile.nickname FROM profile WHERE profile.nickname = $3 RETURNING forum.profile_nickname;",
		forum.Slug, forum.Title, forum.ProfileNickname).Scan(&forum.ProfileNickname); err != nil {
		if err == sql.ErrNoRows {
			return context.JSON(http.StatusNotFound, Error{
				Message: "Can't find user with nickname " + forum.ProfileNickname,
			})
		}
		pqErr := err.(*pq.Error)
		switch pqErr.Code {
		case "23505":
			if err := DBConnection.QueryRow("SELECT forum.slug, forum.title, forum.profile_nickname FROM forum WHERE forum.slug = $1;",
				forum.Slug).Scan(&forum.Slug, &forum.Title, &forum.ProfileNickname); err == sql.ErrNoRows {
				panic(err)
			}
			return context.JSON(http.StatusConflict, forum)
		default:
			panic(err)
		}
	}*/

	return context.JSON(http.StatusCreated, forum)
}

func ThreadCreate(context echo.Context) error {
	var thread Thread
	if err := context.Bind(&thread); err != nil {
		panic(err)
	}
	thread.ForumSlug = context.Param("slug_")

	/*if err := DBConnection.QueryRow("SELECT profile.nickname FROM profile WHERE profile.nickname = $1;",
		thread.ProfileNickname).Scan(&thread.ProfileNickname); err == sql.ErrNoRows {
		return context.JSON(http.StatusNotFound, Error{
			Message: "Can't find user with nickname " + thread.ProfileNickname,
		})
	}

	if err := DBConnection.QueryRow("SELECT forum.slug FROM forum WHERE forum.slug = $1;",
		thread.ForumSlug).Scan(&thread.ForumSlug); err == sql.ErrNoRows {
		return context.JSON(http.StatusNotFound, Error{
			Message: "Can't find forum with slug " + thread.ForumSlug,
		})
	}

	if thread.Slug != "" {
		if err := DBConnection.QueryRow("SELECT thread.id, thread.profile_nickname, thread.created, thread.forum_slug, thread.message, thread.slug, thread.title FROM thread WHERE thread.slug = $1;",
			thread.Slug).Scan(&thread.Id, &thread.ProfileNickname, &thread.Created, &thread.ForumSlug, &thread.Message,
			&thread.Slug, &thread.Title); err != sql.ErrNoRows {
			return context.JSON(http.StatusConflict, thread)
		}
	}

	if err := DBConnection.QueryRow("INSERT INTO thread (id, profile_nickname, created, forum_slug, message, slug, title) VALUES (default, $1, $2, $3, $4, $5, $6) RETURNING thread.id;",
		thread.ProfileNickname, thread.Created, thread.ForumSlug, thread.Message, thread.Slug, thread.Title).Scan(&thread.Id); err != nil {
		panic(err)
	}*/

	if thread.Slug != "" {
		if err := DBConnection.QueryRow("SELECT thread.id, thread.profile_nickname, thread.created, thread.forum_slug, thread.message, thread.slug, thread.title FROM thread WHERE thread.slug = $1;",
			thread.Slug).Scan(&thread.Id, &thread.ProfileNickname, &thread.Created, &thread.ForumSlug, &thread.Message,
			&thread.Slug, &thread.Title); err != sql.ErrNoRows {
			return context.JSON(http.StatusConflict, thread)
		}
	}

	if err := DBConnection.QueryRow("INSERT INTO thread (profile_nickname, created, forum_slug, message, slug, title) SELECT (SELECT profile.nickname FROM profile WHERE profile.nickname = $1), $2, (SELECT forum.slug FROM forum WHERE forum.slug = $3), $4, $5, $6 RETURNING thread.id, thread.forum_slug;",
		thread.ProfileNickname, thread.Created, thread.ForumSlug, thread.Message, thread.Slug, thread.Title).
		Scan(&thread.Id, &thread.ForumSlug); err != nil {
		pqErr := err.(*pq.Error)
		switch pqErr.Column {
		case "profile_nickname":
			return context.JSON(http.StatusNotFound, Error{
				Message: "Can't find user with nickname " + thread.ProfileNickname,
			})
		case "forum_slug":
			return context.JSON(http.StatusNotFound, Error{
				Message: "Can't find forum with slug " + thread.ForumSlug,
			})
		default:
			panic(err)
		}
	}

	return context.JSON(http.StatusCreated, thread)
}

func ForumGetOne(context echo.Context) error {
	var forum Forum
	forum.Slug = context.Param("slug")
	if err := DBConnection.QueryRow("SELECT forum.slug, forum.title, forum.profile_nickname, forum.threads, forum.posts FROM forum WHERE forum.slug = $1;",
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
	if err := DBConnection.QueryRow("SELECT forum.slug FROM forum WHERE forum.slug = $1;",
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
		rows, err = DBConnection.Query("SELECT thread.id, thread.profile_nickname, thread.created, thread.message, thread.slug, thread.title, thread.votes FROM thread WHERE thread.forum_slug = $1 AND thread.created >= $2 ORDER BY thread.created LIMIT $3;",
			forum.Slug, since, limit)
	} else {
		if since == "" {
			since = "infinity"
		}
		rows, err = DBConnection.Query("SELECT thread.id, thread.profile_nickname, thread.created, thread.message, thread.slug, thread.title, thread.votes FROM thread WHERE thread.forum_slug = $1 AND thread.created <= $2 ORDER BY thread.created DESC LIMIT $3;",
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
		if err := rows.Scan(&thread.Id, &thread.ProfileNickname, &thread.Created, &thread.Message, &thread.Slug,
			&thread.Title, &thread.Votes); err != nil {
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
	if err := DBConnection.QueryRow("SELECT forum.slug FROM forum WHERE forum.slug = $1;",
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
			rows, err = DBConnection.Query(fmt.Sprintf("SELECT profile.nickname, profile.about, profile.email, profile.fullname FROM thread JOIN profile ON thread.profile_nickname = profile.nickname WHERE thread.forum_slug = $1 UNION SELECT profile.nickname, profile.about, profile.email, profile.fullname FROM post JOIN thread ON post.thread_id = thread.id AND thread.forum_slug = $1 JOIN profile ON post.profile_nickname = profile.nickname ORDER BY nickname LIMIT %s;", limit),
				forum.Slug)
		} else {
			rows, err = DBConnection.Query(fmt.Sprintf("SELECT profile.nickname, profile.about, profile.email, profile.fullname FROM thread JOIN profile ON thread.profile_nickname = profile.nickname WHERE thread.forum_slug = $1 AND profile.nickname > $2 UNION SELECT profile.nickname, profile.about, profile.email, profile.fullname FROM post JOIN thread ON post.thread_id = thread.id AND thread.forum_slug = $1 JOIN profile ON post.profile_nickname = profile.nickname WHERE profile.nickname > $2 ORDER BY nickname LIMIT %s;", limit),
				forum.Slug, since)
		}
	} else {
		if since == "" {
			rows, err = DBConnection.Query(fmt.Sprintf("SELECT profile.nickname, profile.about, profile.email, profile.fullname FROM thread JOIN profile ON thread.profile_nickname = profile.nickname WHERE thread.forum_slug = $1 UNION SELECT profile.nickname, profile.about, profile.email, profile.fullname FROM post JOIN thread ON post.thread_id = thread.id AND thread.forum_slug = $1 JOIN profile ON post.profile_nickname = profile.nickname ORDER BY nickname DESC LIMIT %s;", limit),
				forum.Slug)
		} else {
			rows, err = DBConnection.Query(fmt.Sprintf("SELECT profile.nickname, profile.about, profile.email, profile.fullname FROM thread JOIN profile ON thread.profile_nickname = profile.nickname WHERE thread.forum_slug = $1 AND profile.nickname < $2 UNION SELECT profile.nickname, profile.about, profile.email, profile.fullname FROM post JOIN thread ON post.thread_id = thread.id AND thread.forum_slug = $1 JOIN profile ON post.profile_nickname = profile.nickname WHERE profile.nickname < $2 ORDER BY nickname DESC LIMIT %s;", limit),
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
		if err := rows.Scan(&profile.Nickname, &profile.About, &profile.Email, &profile.Fullname); err != nil {
			panic(err)
		}
		profiles = append(profiles, profile)
	}

	return context.JSON(http.StatusOK, profiles)
}

func PostGetOne(context echo.Context) error {
	var postFull PostFull
	var posts []int64
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

	if err := DBConnection.QueryRow("SELECT post.id, post.profile_nickname, post.created, post.is_edited, post.message, post.posts, post.thread_id, thread.forum_slug FROM post JOIN thread ON post.thread_id = thread.id WHERE post.id = $1;", postFull.Post.Id).
		Scan(&postFull.Post.Id, &postFull.Post.ProfileNickname, &postFull.Post.Created, &postFull.Post.IsEdited,
			&postFull.Post.Message, pq.Array(&posts), &postFull.Post.Thread, &postFull.Post.ForumSlug); err != nil {
		return context.JSON(http.StatusNotFound, Error{
			Message: "Can't find post with id " + id,
		})
	}
	if len(posts) > 1 {
		postFull.Post.Parent = posts[len(posts)-2]
	}

	if user {
		postFull.Profile = &Profile{}
		if err := DBConnection.QueryRow("SELECT profile.nickname, profile.about, profile.email, profile.fullname FROM post JOIN profile ON post.profile_nickname = profile.nickname WHERE post.id = $1;", postFull.Post.Id).
			Scan(&postFull.Profile.Nickname, &postFull.Profile.About, &postFull.Profile.Email,
				&postFull.Profile.Fullname); err != nil {
			panic(err)
		}
	}

	if forum {
		postFull.Forum = &Forum{}
		if err := DBConnection.QueryRow("SELECT forum.slug, forum.title, forum.profile_nickname, forum.threads, forum.posts FROM post JOIN thread ON post.thread_id = thread.id JOIN forum ON thread.forum_slug = forum.slug WHERE post.id = $1;", postFull.Post.Id).
			Scan(&postFull.Forum.Slug, &postFull.Forum.Title, &postFull.Forum.ProfileNickname, &postFull.Forum.Threads,
				&postFull.Forum.Posts); err != nil {
			panic(err)
		}
	}

	if thread {
		postFull.Thread = &Thread{}
		if err := DBConnection.QueryRow("SELECT thread.id, thread.profile_nickname, thread.created, thread.forum_slug, thread.message, thread.slug, thread.title, thread.votes FROM post JOIN thread ON post.thread_id = thread.id WHERE post.id = $1;", postFull.Post.Id).
			Scan(&postFull.Thread.Id, &postFull.Thread.ProfileNickname, &postFull.Thread.Created,
				&postFull.Thread.ForumSlug, &postFull.Thread.Message, &postFull.Thread.Slug, &postFull.Thread.Title,
				&postFull.Thread.Votes); err != nil {
			panic(err)
		}
	}

	return context.JSON(http.StatusOK, postFull)
}

func PostUpdate(context echo.Context) error {
	var post Post
	id := context.Param("id")
	post.Id, _ = strconv.ParseInt(id, 10, 64)
	if err := DBConnection.QueryRow("SELECT post.id, post.profile_nickname, post.created, post.is_edited, post.message, post.thread_id, thread.forum_slug FROM post JOIN thread ON post.thread_id = thread.id WHERE post.id = $1;", post.Id).
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
		if _, err := DBConnection.Exec("UPDATE post SET message = $1, is_edited = true WHERE id = $2;",
			updatedPost.Message, updatedPost.Id); err != nil {
			panic(err)
		}
		updatedPost.IsEdited = true
	}

	return context.JSON(http.StatusOK, updatedPost)
}

func ServiceClear(context echo.Context) error {
	if _, err := DBConnection.Exec("TRUNCATE TABLE profile CASCADE;"); err != nil {
		panic(err)
	}
	return context.JSON(http.StatusOK, nil)
}

func ServiceStatus(context echo.Context) error {
	var status Status
	if err := DBConnection.QueryRow("SELECT (SELECT COUNT(*) FROM forum), (SELECT COUNT(*) FROM post), (SELECT COUNT(*) FROM thread), (SELECT COUNT(*) FROM profile);").
		Scan(&status.Forum, &status.Post, &status.Thread, &status.User); err != nil {
		panic(err)
	}

	return context.JSON(http.StatusOK, status)
}

func PostsCreate(context echo.Context) error {
	var thread Thread
	slugOrId := context.Param("slug_or_id")

	tx, err := DBConnection.Begin()
	if err != nil {
		panic(err)
	}

	if _, err := strconv.Atoi(slugOrId); err == nil {
		if err := tx.QueryRow("SELECT thread.id, thread.slug, thread.forum_slug FROM thread WHERE thread.id = $1;",
			slugOrId).Scan(&thread.Id, &thread.Slug, &thread.ForumSlug); err != nil {
			if err := tx.Rollback(); err != nil {
				panic(err)
			}
			return context.JSON(http.StatusNotFound, Error{
				Message: "Can't find thread with slug or id " + slugOrId,
			})
		}
	} else {
		if err := tx.QueryRow("SELECT thread.id, thread.slug, thread.forum_slug FROM thread WHERE thread.slug = $1;",
			slugOrId).Scan(&thread.Id, &thread.Slug, &thread.ForumSlug); err != nil {
			if err := tx.Rollback(); err != nil {
				panic(err)
			}
			return context.JSON(http.StatusNotFound, Error{
				Message: "Can't find thread with slug or id " + slugOrId,
			})
		}
	}

	var posts []*Post
	result, err := ioutil.ReadAll(context.Request().Body)
	if err != nil {
		if err := tx.Rollback(); err != nil {
			panic(err)
		}
		panic(err)
	}

	err = json.Unmarshal(result, &posts) //TODO
	if err != nil {
		if err := tx.Rollback(); err != nil {
			panic(err)
		}
		panic(err)
	}

	if len(posts) == 0 {
		if err := tx.Rollback(); err != nil {
			panic(err)
		}
		return context.JSON(http.StatusCreated, posts)
	}

	location, _ := time.LoadLocation("UTC")
	now := time.Now().In(location).Round(time.Microsecond)
	for _, post := range posts {
		var posts []int64
		if post.Created.IsZero() {
			post.Created = now
		}
		if post.Parent == 0 {
			if err = tx.QueryRow("INSERT INTO post (profile_nickname, created, message, thread_id) (SELECT profile.nickname, $1, $2, $3 FROM profile WHERE profile.nickname = $4) RETURNING post.id;",
				post.Created, post.Message, thread.Id, post.ProfileNickname).Scan(&post.Id); err != nil {
				if err := tx.Rollback(); err != nil {
					panic(err)
				}
				return context.JSON(http.StatusNotFound, Error{
					Message: "Can't find post author by nickname " + post.ProfileNickname,
				})
			}
		} else { //TODO: этот запрос можно (но очень сложно) объединить в один, даже можно без транзакции... (с помощью, наверное, триггера, который будет делать эти select для переданных values... ?)
			if err = tx.QueryRow("SELECT post.posts FROM post WHERE post.id = $1 AND post.thread_id = $2;",
				post.Parent, thread.Id).Scan(pq.Array(&posts)); err != nil {
				if err := tx.Rollback(); err != nil {
					panic(err)
				}
				return context.JSON(http.StatusConflict, Error{
					Message: "One of parent posts doesn't exists or it was created in another thread",
				})
			}
			if err = tx.QueryRow("INSERT INTO post (profile_nickname, created, message, posts, thread_id) (SELECT profile.nickname, $1, $2, $3, $4 FROM profile WHERE profile.nickname = $5) RETURNING post.id;",
				post.Created, post.Message, pq.Array(posts), thread.Id, post.ProfileNickname).Scan(&post.Id); err != nil {
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

func ThreadGetOne(context echo.Context) error {
	var thread Thread
	slugOrId := context.Param("slug_or_id")
	if _, err := strconv.Atoi(slugOrId); err == nil {
		if err := DBConnection.QueryRow("SELECT thread.id, thread.profile_nickname, thread.created, thread.forum_slug, thread.message, thread.slug, thread.title, thread.votes FROM thread WHERE thread.id = $1;",
			slugOrId).Scan(&thread.Id, &thread.ProfileNickname, &thread.Created, &thread.ForumSlug, &thread.Message,
			&thread.Slug, &thread.Title, &thread.Votes); err != nil {
			return context.JSON(http.StatusNotFound, Error{
				Message: "Can't find thread with id " + slugOrId,
			})
		}
	} else {
		if err := DBConnection.QueryRow("SELECT thread.id, thread.profile_nickname, thread.created, thread.forum_slug, thread.message, thread.slug, thread.title, thread.votes FROM thread WHERE thread.slug = $1;",
			slugOrId).Scan(&thread.Id, &thread.ProfileNickname, &thread.Created, &thread.ForumSlug, &thread.Message,
			&thread.Slug, &thread.Title, &thread.Votes); err != nil {
			return context.JSON(http.StatusNotFound, Error{
				Message: "Can't find thread with slug " + slugOrId,
			})
		}
	}

	return context.JSON(http.StatusOK, thread)
}

func ThreadUpdate(context echo.Context) error {
	var thread Thread
	slugOrId := context.Param("slug_or_id")
	if _, err := strconv.Atoi(slugOrId); err == nil {
		if err := DBConnection.QueryRow("SELECT thread.id, thread.profile_nickname, thread.created, thread.forum_slug, thread.message, thread.slug, thread.title, thread.votes FROM thread WHERE thread.id = $1;",
			slugOrId).Scan(&thread.Id, &thread.ProfileNickname, &thread.Created, &thread.ForumSlug, &thread.Message,
			&thread.Slug, &thread.Title, &thread.Votes); err != nil {
			return context.JSON(http.StatusNotFound, Error{
				Message: "Can't find thread with id " + slugOrId,
			})
		}
	} else {
		if err := DBConnection.QueryRow("SELECT thread.id, thread.profile_nickname, thread.created, thread.forum_slug, thread.message, thread.slug, thread.title, thread.votes FROM thread WHERE thread.slug = $1;",
			slugOrId).Scan(&thread.Id, &thread.ProfileNickname, &thread.Created, &thread.ForumSlug, &thread.Message,
			&thread.Slug, &thread.Title, &thread.Votes); err != nil {
			return context.JSON(http.StatusNotFound, Error{
				Message: "Can't find thread with slug " + slugOrId,
			})
		}
	}

	if err := context.Bind(&thread); err != nil {
		panic(err)
	}

	if _, err := DBConnection.Exec("UPDATE thread SET message = $2, title = $3 WHERE id = $1;",
		thread.Id, thread.Message, thread.Title); err != nil {
		panic(err)
	}

	return context.JSON(http.StatusOK, thread)
}

func ThreadGetPosts(context echo.Context) error { //TODO: too slow
	limit := context.QueryParam("limit")
	if limit == "" {
		limit = "NULL"
	}

	since := context.QueryParam("since")

	var desc string
	if context.QueryParam("desc") == "true" {
		desc = "DESC"
	}

	var thread Thread
	slugOrId := context.Param("slug_or_id")
	if _, err := strconv.Atoi(slugOrId); err == nil {
		if err := DBConnection.QueryRow("SELECT thread.id, forum.slug FROM thread JOIN forum ON thread.forum_slug = forum.slug WHERE thread.id = $1;",
			slugOrId).Scan(&thread.Id, &thread.ForumSlug); err == sql.ErrNoRows {
			return context.JSON(http.StatusNotFound, Error{
				Message: "Can't find thread with id " + slugOrId,
			})
		}
	} else {
		if err := DBConnection.QueryRow("SELECT thread.id, forum.slug FROM thread JOIN forum ON thread.forum_slug = forum.slug WHERE thread.slug = $1;",
			slugOrId).Scan(&thread.Id, &thread.ForumSlug); err == sql.ErrNoRows {
			return context.JSON(http.StatusNotFound, Error{
				Message: "Can't find thread with slug " + slugOrId,
			})
		}
	}

	var rows *sql.Rows
	var err error
	switch context.QueryParam("sort") {
	case "tree":
		if since == "" {
			rows, err = DBConnection.Query(fmt.Sprintf("SELECT post.id, post.profile_nickname, post.created, post.is_edited, post.message, post.posts FROM post WHERE post.thread_id = $1 ORDER BY post.posts %s, post.created, post.id LIMIT $2;", desc),
				thread.Id, limit)
		} else {
			if desc == "" {
				rows, err = DBConnection.Query(fmt.Sprintf("SELECT post.id, post.profile_nickname, post.created, post.is_edited, post.message, post.posts FROM post WHERE post.posts > (SELECT post.posts FROM post WHERE post.id = $2) AND post.thread_id = $1 ORDER BY post.posts %s, post.created, post.id LIMIT $3;", desc),
					thread.Id, since, limit)
			} else {
				rows, err = DBConnection.Query(fmt.Sprintf("SELECT post.id, post.profile_nickname, post.created, post.is_edited, post.message, post.posts FROM post WHERE post.posts < (SELECT post.posts FROM post WHERE post.id = $2) AND post.thread_id = $1 ORDER BY post.posts %s, post.created, post.id LIMIT $3;", desc),
					thread.Id, since, limit)
			}
		}
		break
	case "parent_tree":
		if since == "" {
			rows, err = DBConnection.Query(fmt.Sprintf("SELECT post.id, post.profile_nickname, post.created, post.is_edited, post.message, post.posts FROM post WHERE post.posts[1] IN (SELECT DISTINCT post.posts[1] FROM post WHERE post.thread_id = $1 AND array_length(post.posts, 1) = 1 ORDER BY post.posts[1] %s LIMIT $2) ORDER BY post.posts[1] %s, post.posts, post.created, post.id;", desc, desc),
				thread.Id, limit)
		} else {
			if desc == "" {
				rows, err = DBConnection.Query("SELECT post.id, post.profile_nickname, post.created, post.is_edited, post.message, post.posts FROM post JOIN (SELECT post.posts[1] AS root FROM post WHERE post.thread_id = $1 AND post.posts[1] > (SELECT post.posts[1] FROM post WHERE post.id = $2) AND array_length(post.posts, 1) = 1 ORDER BY root LIMIT $3) root_posts ON post.posts[1] = root_posts.root ORDER BY post.posts, post.created, post.id;",
					thread.Id, since, limit)
			} else {
				rows, err = DBConnection.Query("SELECT post.id, post.profile_nickname, post.created, post.is_edited, post.message, post.posts FROM post JOIN (SELECT post.posts[1] AS root FROM post WHERE post.thread_id = $1 AND post.posts[1] < (SELECT post.posts[1] FROM post WHERE post.id = $2) AND array_length(post.posts, 1) = 1 ORDER BY root DESC LIMIT $3) root_posts ON post.posts[1] = root_posts.root ORDER BY post.posts[1] DESC, post.posts[2:], post.created, post.id;",
					thread.Id, since, limit)
			}
		}
		break
	default: //flat
		if since == "" {
			rows, err = DBConnection.Query(fmt.Sprintf("SELECT post.id, post.profile_nickname, post.created, post.is_edited, post.message, post.posts FROM post WHERE post.thread_id = $1 ORDER BY post.created %s, post.id %s LIMIT $2;", desc, desc),
				thread.Id, limit)
		} else {
			if desc == "" {
				rows, err = DBConnection.Query(fmt.Sprintf("SELECT post.id, post.profile_nickname, post.created, post.is_edited, post.message, post.posts FROM post WHERE post.thread_id = $1 AND post.id > $2 ORDER BY post.created %s, post.id %s LIMIT $3;", desc, desc),
					thread.Id, since, limit)
			} else {
				rows, err = DBConnection.Query(fmt.Sprintf("SELECT post.id, post.profile_nickname, post.created, post.is_edited, post.message, post.posts FROM post WHERE post.thread_id = $1 AND post.id < $2 ORDER BY post.created %s, post.id %s LIMIT $3;", desc, desc),
					thread.Id, since, limit)
			}
		}
		break
	}
	if err != nil {
		panic(err)
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
		var postPosts []int64
		if err := rows.Scan(&post.Id, &post.ProfileNickname, &post.Created, &post.IsEdited, &post.Message,
			pq.Array(&postPosts)); err != nil {
			panic(err)
		}
		post.ForumSlug = thread.ForumSlug
		if len(postPosts) > 1 {
			post.Parent = postPosts[len(postPosts)-2]
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

	var thread Thread
	slugOrId := context.Param("slug_or_id")
	if _, err := strconv.Atoi(slugOrId); err == nil {
		if err := DBConnection.QueryRow("SELECT thread.id, thread.profile_nickname, thread.created, thread.forum_slug, thread.message, thread.slug, thread.title, thread.votes FROM thread WHERE thread.id = $1;",
			slugOrId).Scan(&thread.Id, &thread.ProfileNickname, &thread.Created, &thread.ForumSlug, &thread.Message,
			&thread.Slug, &thread.Title, &thread.Votes); err != nil {
			return context.JSON(http.StatusNotFound, Error{
				Message: "Can't find thread with id " + slugOrId,
			})
		}
	} else {
		if err := DBConnection.QueryRow("SELECT thread.id, thread.profile_nickname, thread.created, thread.forum_slug, thread.message, thread.slug, thread.title, thread.votes FROM thread WHERE thread.slug = $1;",
			slugOrId).Scan(&thread.Id, &thread.ProfileNickname, &thread.Created, &thread.ForumSlug, &thread.Message,
			&thread.Slug, &thread.Title, &thread.Votes); err != nil {
			return context.JSON(http.StatusNotFound, Error{
				Message: "Can't find thread with slug " + slugOrId,
			})
		}
	}

	if _, err := DBConnection.Exec("INSERT INTO vote (profile_nickname, thread_id, voice) VALUES ($1, $2, $3) ON CONFLICT (profile_nickname, thread_id) DO UPDATE SET voice = $3;",
		vote.ProfileNickname, thread.Id, vote.Voice); err != nil {
		return context.JSON(http.StatusNotFound, Error{
			Message: "Can't find user with nickname " + vote.ProfileNickname,
		})
	}

	if err := DBConnection.QueryRow("SELECT thread.votes FROM thread WHERE thread.id = $1;", thread.Id).
		Scan(&thread.Votes); err != nil {
		return context.JSON(http.StatusNotFound, Error{
			Message: "Can't find thread with id " + strconv.Itoa(int(thread.Id)),
		})
	}

	return context.JSON(http.StatusOK, thread)
}

func UserCreate(context echo.Context) error {
	var profile Profile
	if err := context.Bind(&profile); err != nil {
		panic(err)
	}
	profile.Nickname = context.Param("nickname")

	rows, err := DBConnection.Query("SELECT profile.nickname, profile.about, profile.email, profile.fullname FROM profile WHERE profile.nickname = $1 OR profile.email = $2;",
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
		if err := rows.Scan(&existingProfile.Nickname, &existingProfile.About, &existingProfile.Email,
			&existingProfile.Fullname); err != nil {
			panic(err)
		}
		existingProfiles = append(existingProfiles, existingProfile)
	}

	if len(existingProfiles) > 0 {
		return context.JSON(http.StatusConflict, existingProfiles)
	}

	if _, err = DBConnection.Exec("INSERT INTO profile (nickname, about, email, fullname) VALUES ($1, $2, $3, $4);",
		profile.Nickname, profile.About, profile.Email, profile.Fullname); err != nil {
		panic(err)
	}

	return context.JSON(http.StatusCreated, profile)
}

func UserGetOne(context echo.Context) error {
	var profile Profile
	if err := DBConnection.QueryRow("SELECT profile.nickname, profile.about, profile.email, profile.fullname FROM profile WHERE profile.nickname = $1;",
		context.Param("nickname")).Scan(&profile.Nickname, &profile.About, &profile.Email,
		&profile.Fullname); err == sql.ErrNoRows {
		return context.JSON(http.StatusNotFound, Error{
			Message: "Can't find user with nickname " + profile.Nickname,
		})
	}

	return context.JSON(http.StatusOK, profile)
}

func UserUpdate(context echo.Context) error {
	var profile Profile
	profile.Nickname = context.Param("nickname")
	if err := DBConnection.QueryRow("SELECT profile.nickname, profile.about, profile.email, profile.fullname FROM profile WHERE profile.nickname = $1;",
		profile.Nickname).Scan(&profile.Nickname, &profile.About, &profile.Email, &profile.Fullname); err == sql.ErrNoRows {
		return context.JSON(http.StatusNotFound, Error{
			Message: "Can't find user with nickname " + profile.Nickname,
		})
	}

	updatedProfile := profile
	if err := context.Bind(&updatedProfile); err != nil {
		panic(err)
	}

	err := DBConnection.QueryRow("SELECT profile.nickname FROM profile WHERE profile.email = $1 AND profile.nickname != $2;",
		updatedProfile.Email, profile.Nickname).Scan(&updatedProfile.Nickname)
	if err != sql.ErrNoRows {
		return context.JSON(http.StatusConflict, Error{
			Message: "This email is already registered by user " + updatedProfile.Nickname,
		})
	}

	if _, err := DBConnection.Exec("UPDATE profile SET about = $2, email = $3, fullname = $4 WHERE nickname = $1;",
		updatedProfile.Nickname, updatedProfile.About, updatedProfile.Email, updatedProfile.Fullname); err != nil {
		panic(err)
	}

	return context.JSON(http.StatusOK, updatedProfile)
}
