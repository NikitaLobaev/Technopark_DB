package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v4"
	_ "github.com/lib/pq"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"
)

//easyjson:json
type Error struct {
	Message string `json:"message"`
}

//easyjson:json
type Profile struct {
	Id       uint32 `json:"-"`
	Nickname string `json:"nickname"`
	About    string `json:"about"`
	Email    string `json:"email"`
	Fullname string `json:"fullname"`
}

//easyjson:json
type Forum struct {
	//Id              uint32 `json:"-"`
	Slug            string `json:"slug"`
	Title           string `json:"title"`
	ProfileId       uint32 `json:"-"`
	ProfileNickname string `json:"user"`
	Threads         uint32 `json:"threads"`
	Posts           uint64 `json:"posts"`
}

//easyjson:json
type Thread struct {
	Id              uint32    `json:"id"`
	ProfileId       uint32    `json:"-"`
	ProfileNickname string    `json:"author"`
	Created         time.Time `json:"created"`
	ForumSlug       string    `json:"forum"`
	Message         string    `json:"message"`
	Slug            string    `json:"slug"`
	Title           string    `json:"title"`
	Votes           int32     `json:"votes"`
}

//easyjson:json
type Post struct {
	Id              uint64    `json:"id"`
	ProfileId       uint32    `json:"-"`
	ProfileNickname string    `json:"author"`
	Created         time.Time `json:"created"`
	ForumSlug       string    `json:"forum"`
	IsEdited        bool      `json:"isEdited"`
	Message         string    `json:"message"`
	ParentPost      uint64    `json:"parent,omitempty"`
	ThreadId        uint32    `   json:"thread"`
}

//easyjson:json
type PostFull struct {
	Profile *Profile `json:"author,omitempty"`
	Forum   *Forum   `json:"forum,omitempty"`
	Post    Post     `json:"post"`
	Thread  *Thread  `json:"thread,omitempty"`
}

//easyjson:json
type Vote struct {
	ProfileId       uint32 `json:"-"`
	ProfileNickname string `json:"nickname"`
	ThreadId        uint32 `json:"-"`
	Voice           int8   `json:"voice"`
}

//easyjson:json
type Status struct {
	Forum  uint32 `json:"forum"`
	Post   uint64 `json:"post"`
	Thread uint32 `json:"thread"`
	User   uint32 `json:"user"`
}

//TODO: сгенерировать easyjson?
//TODO: вставку полей типа INSERT INTO ... (profile_nickname, ...) SELECT profile.nickname, ... FROM profile ... оставлять на откуп СУБД (в триггерах), а не приложению

/*var apiCalls uint8

func Api(_ echo.Context) error {
	if apiCalls++; apiCalls == 3 {
		if _, err := DBConnection.Exec("VACUUM ANALYZE;"); err != nil {
			panic(err)
		}
	}
	return nil
}*/

func ForumCreate(context echo.Context) error {
	var forum Forum
	if err := context.Bind(&forum); err != nil {
		panic(err)
	}

	if err := DBConnection.QueryRow("INSERT INTO forum (slug, title, profile_nickname) SELECT $1, $2, profile.nickname FROM profile WHERE profile.nickname = $3 RETURNING forum.profile_nickname;",
		forum.Slug, forum.Title, forum.ProfileNickname).Scan(&forum.ProfileNickname); err != nil {
		if err == sql.ErrNoRows {
			return context.JSON(http.StatusNotFound, Error{
				Message: "Can't find user with nickname " + forum.ProfileNickname,
			})
		} else {
			if err := DBConnection.QueryRow("SELECT forum.slug, forum.title, forum.profile_nickname FROM forum WHERE forum.slug = $1;",
				forum.Slug).Scan(&forum.Slug, &forum.Title, &forum.ProfileNickname); err != nil {
				panic(err)
			}
			return context.JSON(http.StatusConflict, forum)
		}
	}

	return context.JSON(http.StatusCreated, forum)
}

func ThreadCreate(context echo.Context) error {
	var thread Thread
	if err := context.Bind(&thread); err != nil {
		panic(err)
	}
	thread.ForumSlug = context.Param("slug_")

	if err := DBConnection.QueryRow("INSERT INTO thread (profile_nickname, created, forum_slug, message, slug, title) SELECT profile.nickname, $2, forum.slug, $4, $5, $6 FROM profile, forum WHERE profile.nickname = $1 AND forum.slug = $3 RETURNING thread.id, thread.profile_nickname, thread.forum_slug;",
		thread.ProfileNickname, thread.Created, thread.ForumSlug, thread.Message, thread.Slug, thread.Title).
		Scan(&thread.Id, &thread.ProfileNickname, &thread.ForumSlug); err != nil {
		if err == sql.ErrNoRows {
			return context.JSON(http.StatusNotFound, Error{
				Message: "Can't find user with nickname " + thread.ProfileNickname + " or forum with slug " + thread.ForumSlug,
			})
		} else {
			if err := DBConnection.QueryRow("SELECT thread.id, thread.profile_nickname, thread.created, thread.forum_slug, thread.message, thread.slug, thread.title FROM thread WHERE thread.slug = $1;",
				thread.Slug).Scan(&thread.Id, &thread.ProfileNickname, &thread.Created, &thread.ForumSlug, &thread.Message,
				&thread.Slug, &thread.Title); err != nil {
				panic(err)
			}
			return context.JSON(http.StatusConflict, thread)
		}
	}

	return context.JSON(http.StatusCreated, thread)
}

func ForumGetOne(context echo.Context) error {
	var forum Forum
	forum.Slug = context.Param("slug")
	if err := DBConnection.QueryRow("SELECT forum.slug, forum.title, forum.profile_nickname, forum.threads, forum.posts FROM forum WHERE forum.slug = $1;", //"EXECUTE prepared_forum_get_one($1);", //
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
			rows, err = DBConnection.Query("SELECT thread.id, thread.profile_nickname, thread.created, thread.message, thread.slug, thread.title, thread.votes FROM thread WHERE thread.forum_slug = $1 ORDER BY thread.created LIMIT $2;",
				forum.Slug, limit)
		} else {
			rows, err = DBConnection.Query("SELECT thread.id, thread.profile_nickname, thread.created, thread.message, thread.slug, thread.title, thread.votes FROM thread WHERE thread.forum_slug = $1 AND thread.created >= $2 ORDER BY thread.created LIMIT $3;",
				forum.Slug, since, limit)
		}
	} else {
		if since == "" {
			rows, err = DBConnection.Query("SELECT thread.id, thread.profile_nickname, thread.created, thread.message, thread.slug, thread.title, thread.votes FROM thread WHERE thread.forum_slug = $1 ORDER BY thread.created DESC LIMIT $2;",
				forum.Slug, limit)
		} else {
			rows, err = DBConnection.Query("SELECT thread.id, thread.profile_nickname, thread.created, thread.message, thread.slug, thread.title, thread.votes FROM thread WHERE thread.forum_slug = $1 AND thread.created <= $2 ORDER BY thread.created DESC LIMIT $3;",
				forum.Slug, since, limit)
		}
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
		var threadSlug sql.NullString
		if err := rows.Scan(&thread.Id, &thread.ProfileNickname, &thread.Created, &thread.Message, &threadSlug,
			&thread.Title, &thread.Votes); err != nil {
			panic(err)
		}

		if threadSlug.Valid {
			thread.Slug = threadSlug.String
		}
		thread.ForumSlug = forum.Slug

		threads = append(threads, thread)
	}

	return context.JSON(http.StatusOK, threads)
}

func ForumGetUsers(context echo.Context) error {
	limit := context.QueryParam("limit")
	if limit == "" {
		limit = "100"
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
			rows, err = DBConnection.Query("SELECT forum_user.profile_nickname, forum_user.profile_about, forum_user.profile_email, forum_user.profile_fullname FROM forum_user WHERE forum_user.forum_slug = $1 ORDER BY forum_user.profile_nickname LIMIT $2;",
				forum.Slug, limit)
		} else {
			rows, err = DBConnection.Query("SELECT forum_user.profile_nickname, forum_user.profile_about, forum_user.profile_email, forum_user.profile_fullname FROM forum_user WHERE forum_user.forum_slug = $1 AND forum_user.profile_nickname > $2 ORDER BY forum_user.profile_nickname LIMIT $3;",
				forum.Slug, since, limit)
		}
	} else {
		if since == "" {
			rows, err = DBConnection.Query("SELECT forum_user.profile_nickname, forum_user.profile_about, forum_user.profile_email, forum_user.profile_fullname FROM forum_user WHERE forum_user.forum_slug = $1 ORDER BY forum_user.profile_nickname DESC LIMIT $2;",
				forum.Slug, limit)
		} else {
			rows, err = DBConnection.Query("SELECT forum_user.profile_nickname, forum_user.profile_about, forum_user.profile_email, forum_user.profile_fullname FROM forum_user WHERE forum_user.forum_slug = $1 AND forum_user.profile_nickname < $2 ORDER BY forum_user.profile_nickname DESC LIMIT $3;",
				forum.Slug, since, limit)
		}
	}
	if err != nil {
		panic(err)
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
	id := context.Param("id")
	postFull.Post.Id, _ = strconv.ParseUint(id, 10, 64)
	var user, forum, thread bool
	var bitFlag uint8
	for _, related := range strings.Split(context.QueryParam("related"), ",") {
		switch related {
		case "user":
			user = true
			bitFlag |= 1
			break
		case "forum":
			forum = true
			bitFlag |= 2
		case "thread":
			thread = true
			bitFlag |= 4
			break
		}
	}

	/*var parentPostId sql.NullInt64
	switch bitFlag {
	case 7: //post + thread + forum + user
		postFull.Thread = &Thread{}
		var threadSlug sql.NullString
		postFull.Forum = &Forum{}
		postFull.Profile = &Profile{}
		if err := DBConnection.QueryRow("SELECT post.profile_nickname, post.created, post.is_edited, post.message, post.post_parent_id, post.thread_id, post.forum_slug, thread.id, thread.profile_nickname, thread.created, thread.forum_slug, thread.message, thread.slug, thread.title, thread.votes FROM post, thread WHERE post.id = $1 AND thread.id = $2;",
			postFull.Post.Id).Scan(&postFull.Post.ProfileNickname, &postFull.Post.Created, &postFull.Post.IsEdited,
			&postFull.Post.Message, &parentPostId, &postFull.Post.ThreadId, &postFull.Post.ForumSlug); err != nil {
			return context.JSON(http.StatusNotFound, Error{
				Message: "Can't find post with id " + id,
			})
		}
		if threadSlug.Valid {
			postFull.Thread.Slug = threadSlug.String
		}
		break
	case 6: //post + thread + forum
		postFull.Thread = &Thread{}
		var threadSlug sql.NullString
		postFull.Forum = &Forum{}
		if err := DBConnection.QueryRow("SELECT post.profile_nickname, post.created, post.is_edited, post.message, post.post_parent_id, post.thread_id, post.forum_slug, thread.id, thread.profile_nickname, thread.created, thread.forum_slug, thread.message, thread.slug, thread.title, thread.votes FROM post WHERE post.id = $1;",
			postFull.Post.Id).Scan(&postFull.Post.ProfileNickname, &postFull.Post.Created, &postFull.Post.IsEdited,
			&postFull.Post.Message, &parentPostId, &postFull.Post.ThreadId, &postFull.Post.ForumSlug); err != nil {
			return context.JSON(http.StatusNotFound, Error{
				Message: "Can't find post with id " + id,
			})
		}
		if threadSlug.Valid {
			postFull.Thread.Slug = threadSlug.String
		}
		break
	case 5: //post + thread + user
		postFull.Thread = &Thread{}
		var threadSlug sql.NullString
		postFull.Profile = &Profile{}
		if err := DBConnection.QueryRow("SELECT post.profile_nickname, post.created, post.is_edited, post.message, post.post_parent_id, post.thread_id, post.forum_slug FROM post WHERE post.id = $1;",
			postFull.Post.Id).Scan(&postFull.Post.ProfileNickname, &postFull.Post.Created, &postFull.Post.IsEdited,
			&postFull.Post.Message, &parentPostId, &postFull.Post.ThreadId, &postFull.Post.ForumSlug); err != nil {
			return context.JSON(http.StatusNotFound, Error{
				Message: "Can't find post with id " + id,
			})
		}
		if threadSlug.Valid {
			postFull.Thread.Slug = threadSlug.String
		}
		break
	case 4: //post + thread
		postFull.Thread = &Thread{}
		var threadSlug sql.NullString
		if err := DBConnection.QueryRow("SELECT post.profile_nickname, post.created, post.is_edited, post.message, post.post_parent_id, post.thread_id, post.forum_slug FROM post WHERE post.id = $1;",
			postFull.Post.Id).Scan(&postFull.Post.ProfileNickname, &postFull.Post.Created, &postFull.Post.IsEdited,
			&postFull.Post.Message, &parentPostId, &postFull.Post.ThreadId, &postFull.Post.ForumSlug); err != nil {
			return context.JSON(http.StatusNotFound, Error{
				Message: "Can't find post with id " + id,
			})
		}
		if threadSlug.Valid {
			postFull.Thread.Slug = threadSlug.String
		}
		break
	case 3: //post + forum + user
		postFull.Forum = &Forum{}
		postFull.Profile = &Profile{}
		if err := DBConnection.QueryRow("SELECT post.profile_nickname, post.created, post.is_edited, post.message, post.post_parent_id, post.thread_id, post.forum_slug FROM post WHERE post.id = $1;",
			postFull.Post.Id).Scan(&postFull.Post.ProfileNickname, &postFull.Post.Created, &postFull.Post.IsEdited,
			&postFull.Post.Message, &parentPostId, &postFull.Post.ThreadId, &postFull.Post.ForumSlug); err != nil {
			return context.JSON(http.StatusNotFound, Error{
				Message: "Can't find post with id " + id,
			})
		}
		break
	case 2: //post + forum
		postFull.Forum = &Forum{}
		if err := DBConnection.QueryRow("SELECT post.profile_nickname, post.created, post.is_edited, post.message, post.post_parent_id, post.thread_id, post.forum_slug FROM post WHERE post.id = $1;",
			postFull.Post.Id).Scan(&postFull.Post.ProfileNickname, &postFull.Post.Created, &postFull.Post.IsEdited,
			&postFull.Post.Message, &parentPostId, &postFull.Post.ThreadId, &postFull.Post.ForumSlug); err != nil {
			return context.JSON(http.StatusNotFound, Error{
				Message: "Can't find post with id " + id,
			})
		}
		break
	case 1: //post + user
		postFull.Profile = &Profile{}
		if err := DBConnection.QueryRow("SELECT post.profile_nickname, post.created, post.is_edited, post.message, post.post_parent_id, post.thread_id, post.forum_slug FROM post WHERE post.id = $1;",
			postFull.Post.Id).Scan(&postFull.Post.ProfileNickname, &postFull.Post.Created, &postFull.Post.IsEdited,
			&postFull.Post.Message, &parentPostId, &postFull.Post.ThreadId, &postFull.Post.ForumSlug); err != nil {
			return context.JSON(http.StatusNotFound, Error{
				Message: "Can't find post with id " + id,
			})
		}
		break
	default: //post
		if err := DBConnection.QueryRow("SELECT post.profile_nickname, post.created, post.is_edited, post.message, post.post_parent_id, post.thread_id, post.forum_slug FROM post WHERE post.id = $1;",
			postFull.Post.Id).Scan(&postFull.Post.ProfileNickname, &postFull.Post.Created, &postFull.Post.IsEdited,
			&postFull.Post.Message, &parentPostId, &postFull.Post.ThreadId, &postFull.Post.ForumSlug); err != nil {
			return context.JSON(http.StatusNotFound, Error{
				Message: "Can't find post with id " + id,
			})
		}
		break
	}

	if parentPostId.Valid {
		postFull.Post.ParentPost = uint64(parentPostId.Int64)
	}*/

	var parentPostId sql.NullInt64
	if err := DBConnection.QueryRow("SELECT post.profile_nickname, post.created, post.is_edited, post.message, post.post_parent_id, post.thread_id, post.forum_slug FROM post WHERE post.id = $1;",
		postFull.Post.Id).Scan(&postFull.Post.ProfileNickname, &postFull.Post.Created, &postFull.Post.IsEdited,
		&postFull.Post.Message, &parentPostId, &postFull.Post.ThreadId, &postFull.Post.ForumSlug); err != nil {
		return context.JSON(http.StatusNotFound, Error{
			Message: "Can't find post with id " + id,
		})
	}
	if parentPostId.Valid {
		postFull.Post.ParentPost = uint64(parentPostId.Int64)
	}

	if user {
		postFull.Profile = &Profile{}
		if err := DBConnection.QueryRow("SELECT profile.nickname, profile.about, profile.email, profile.fullname FROM profile WHERE profile.nickname = $1;", postFull.Post.ProfileNickname).
			Scan(&postFull.Profile.Nickname, &postFull.Profile.About, &postFull.Profile.Email,
				&postFull.Profile.Fullname); err != nil {
			panic(err)
		}
	}

	if forum {
		postFull.Forum = &Forum{}
		if err := DBConnection.QueryRow("SELECT forum.slug, forum.title, forum.profile_nickname, forum.threads, forum.posts FROM forum WHERE forum.slug = $1;", postFull.Post.ForumSlug).
			Scan(&postFull.Forum.Slug, &postFull.Forum.Title, &postFull.Forum.ProfileNickname, &postFull.Forum.Threads,
				&postFull.Forum.Posts); err != nil {
			panic(err)
		}
	}

	if thread {
		postFull.Thread = &Thread{}
		var threadSlug sql.NullString
		if err := DBConnection.QueryRow("SELECT thread.id, thread.profile_nickname, thread.created, thread.forum_slug, thread.message, thread.slug, thread.title, thread.votes FROM thread WHERE thread.id = $1;", postFull.Post.ThreadId).
			Scan(&postFull.Thread.Id, &postFull.Thread.ProfileNickname, &postFull.Thread.Created,
				&postFull.Thread.ForumSlug, &postFull.Thread.Message, &threadSlug, &postFull.Thread.Title,
				&postFull.Thread.Votes); err != nil {
			panic(err)
		}

		if threadSlug.Valid {
			postFull.Thread.Slug = threadSlug.String
		}
	}

	return context.JSON(http.StatusOK, postFull)
}

func PostUpdate(context echo.Context) error { //TODO: тоже можно сократить количество походов в СУБД, но есть ли смысл? это update...
	var post Post
	id := context.Param("id")
	post.Id, _ = strconv.ParseUint(id, 10, 64)
	if err := DBConnection.QueryRow("SELECT post.id, post.profile_nickname, post.created, post.is_edited, post.message, post.thread_id, post.forum_slug FROM post WHERE post.id = $1;", post.Id).
		Scan(&post.Id, &post.ProfileNickname, &post.Created, &post.IsEdited, &post.Message, &post.ThreadId,
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
		if _, err := DBConnection.Exec("UPDATE post SET message = $1 WHERE id = $2;",
			updatedPost.Message, updatedPost.Id); err != nil {
			panic(err)
		}
		updatedPost.IsEdited = true
	}

	return context.JSON(http.StatusOK, updatedPost)
}

func ServiceClear(context echo.Context) error {
	if _, err := DBConnection.Exec("TRUNCATE TABLE profile RESTART IDENTITY CASCADE;"); err != nil {
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
	var threadSlug sql.NullString
	slugOrId := context.Param("slug_or_id")

	if _, err := strconv.Atoi(slugOrId); err == nil {
		if err := DBConnection.QueryRow("SELECT thread.id, thread.slug, thread.forum_slug FROM thread WHERE thread.id = $1;",
			slugOrId).Scan(&thread.Id, &threadSlug, &thread.ForumSlug); err != nil {
			return context.JSON(http.StatusNotFound, Error{
				Message: "Can't find thread with id " + slugOrId,
			})
		}
	} else {
		if err := DBConnection.QueryRow("SELECT thread.id, thread.slug, thread.forum_slug FROM thread WHERE thread.slug = $1;",
			slugOrId).Scan(&thread.Id, &threadSlug, &thread.ForumSlug); err != nil {
			return context.JSON(http.StatusNotFound, Error{
				Message: "Can't find thread with slug " + slugOrId,
			})
		}
	}

	if threadSlug.Valid {
		thread.Slug = threadSlug.String
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

	location, _ := time.LoadLocation("UTC")
	now := time.Now().In(location).Round(time.Microsecond)

	tx, err := DBConnection.Begin()
	defer func() {
		_ = tx.Rollback()
	}()
	if err != nil {
		panic(err)
	}

	statement, err := tx.Prepare("INSERT INTO post (profile_nickname, created, message, post_parent_id, thread_id, forum_slug) SELECT profile.nickname, $2, $3, $4, $5, $6 FROM profile WHERE profile.nickname = $1 RETURNING post.id;")
	defer func() {
		if err := statement.Close(); err != nil {
			panic(err)
		}
	}()

	for _, post := range posts { //TODO: возможно везде заменить создание массивов/данных в стеке на make... везде так
		if post.Created.IsZero() {
			post.Created = now
		}

		if err = statement.QueryRow(post.ProfileNickname, post.Created, post.Message, post.ParentPost, thread.Id,
			thread.ForumSlug).Scan(&post.Id); err != nil {
			if err == sql.ErrNoRows {
				return context.JSON(http.StatusNotFound, Error{
					Message: "Can't find post author by nickname " + post.ProfileNickname,
				})
			} else {
				return context.JSON(http.StatusConflict, Error{
					Message: "One of parent posts doesn't exists or it was created in another thread",
				})
			}
		}

		post.ThreadId = thread.Id
		post.ForumSlug = thread.ForumSlug
	}
	if err = tx.Commit(); err != nil {
		panic(err)
	}

	return context.JSON(http.StatusCreated, posts)
}

func ThreadGetOne(context echo.Context) error {
	var thread Thread
	var threadSlug sql.NullString
	slugOrId := context.Param("slug_or_id")
	if _, err := strconv.Atoi(slugOrId); err == nil {
		if err := DBConnection.QueryRow("SELECT thread.id, thread.profile_nickname, thread.created, thread.forum_slug, thread.message, thread.slug, thread.title, thread.votes FROM thread WHERE thread.id = $1;",
			slugOrId).Scan(&thread.Id, &thread.ProfileNickname, &thread.Created, &thread.ForumSlug, &thread.Message,
			&threadSlug, &thread.Title, &thread.Votes); err != nil {
			return context.JSON(http.StatusNotFound, Error{
				Message: "Can't find thread with id " + slugOrId,
			})
		}
	} else {
		if err := DBConnection.QueryRow("SELECT thread.id, thread.profile_nickname, thread.created, thread.forum_slug, thread.message, thread.slug, thread.title, thread.votes FROM thread WHERE thread.slug = $1;",
			slugOrId).Scan(&thread.Id, &thread.ProfileNickname, &thread.Created, &thread.ForumSlug, &thread.Message,
			&threadSlug, &thread.Title, &thread.Votes); err != nil {
			return context.JSON(http.StatusNotFound, Error{
				Message: "Can't find thread with slug " + slugOrId,
			})
		}
	}

	if threadSlug.Valid {
		thread.Slug = threadSlug.String
	}

	return context.JSON(http.StatusOK, thread)
}

func ThreadUpdate(context echo.Context) error { //TODO: тоже можно сократить количество походов в СУБД, но есть ли смысл? это update...
	var thread Thread
	var threadSlug sql.NullString
	slugOrId := context.Param("slug_or_id")
	if _, err := strconv.Atoi(slugOrId); err == nil {
		if err := DBConnection.QueryRow("SELECT thread.id, thread.profile_nickname, thread.created, thread.forum_slug, thread.message, thread.slug, thread.title, thread.votes FROM thread WHERE thread.id = $1;",
			slugOrId).Scan(&thread.Id, &thread.ProfileNickname, &thread.Created, &thread.ForumSlug, &thread.Message,
			&threadSlug, &thread.Title, &thread.Votes); err != nil {
			return context.JSON(http.StatusNotFound, Error{
				Message: "Can't find thread with id " + slugOrId,
			})
		}
	} else {
		if err := DBConnection.QueryRow("SELECT thread.id, thread.profile_nickname, thread.created, thread.forum_slug, thread.message, thread.slug, thread.title, thread.votes FROM thread WHERE thread.slug = $1;",
			slugOrId).Scan(&thread.Id, &thread.ProfileNickname, &thread.Created, &thread.ForumSlug, &thread.Message,
			&threadSlug, &thread.Title, &thread.Votes); err != nil {
			return context.JSON(http.StatusNotFound, Error{
				Message: "Can't find thread with slug " + slugOrId,
			})
		}
	}

	if threadSlug.Valid {
		thread.Slug = threadSlug.String
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

func ThreadGetPosts(context echo.Context) error {
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
		if err := DBConnection.QueryRow("SELECT thread.id, thread.forum_slug FROM thread WHERE thread.id = $1;",
			slugOrId).Scan(&thread.Id, &thread.ForumSlug); err != nil {
			return context.JSON(http.StatusNotFound, Error{
				Message: "Can't find thread with id " + slugOrId,
			})
		}
	} else {
		if err := DBConnection.QueryRow("SELECT thread.id, thread.forum_slug FROM thread WHERE thread.slug = $1;",
			slugOrId).Scan(&thread.Id, &thread.ForumSlug); err != nil {
			return context.JSON(http.StatusNotFound, Error{
				Message: "Can't find thread with slug " + slugOrId,
			})
		}
	}

	var rows *sql.Rows
	var err error
	switch context.QueryParam("sort") { //TODO: заменить " на `
	case "tree":
		if since == "" {
			rows, err = DBConnection.Query(fmt.Sprintf("SELECT post.id, post.profile_nickname, post.created, post.is_edited, post.message, post.post_parent_id FROM post WHERE post.thread_id = $1 ORDER BY post.path_ %s, post.created, post.id LIMIT $2;", desc),
				thread.Id, limit)
		} else {
			if desc == "" {
				rows, err = DBConnection.Query("SELECT post.id, post.profile_nickname, post.created, post.is_edited, post.message, post.post_parent_id FROM post WHERE post.thread_id = $1 AND post.path_ > (SELECT post.path_ FROM post WHERE post.id = $2) ORDER BY post.path_, post.created, post.id LIMIT $3;",
					thread.Id, since, limit)
			} else {
				rows, err = DBConnection.Query("SELECT post.id, post.profile_nickname, post.created, post.is_edited, post.message, post.post_parent_id FROM post WHERE post.thread_id = $1 AND post.path_ < (SELECT post.path_ FROM post WHERE post.id = $2) ORDER BY post.path_ DESC, post.created, post.id LIMIT $3;",
					thread.Id, since, limit)
			}
		}
		break
	case "parent_tree":
		if since == "" {
			rows, err = DBConnection.Query(fmt.Sprintf("SELECT post.id, post.profile_nickname, post.created, post.is_edited, post.message, post.post_parent_id FROM post WHERE post.post_root_id IN (SELECT post.id FROM post WHERE post.post_parent_id IS NULL AND post.thread_id = $1 ORDER BY post.id %s LIMIT $2) ORDER BY post.post_root_id %s, post.path_, post.created, post.id;", desc, desc),
				thread.Id, limit)
		} else {
			if desc == "" {
				rows, err = DBConnection.Query("SELECT post.id, post.profile_nickname, post.created, post.is_edited, post.message, post.post_parent_id FROM post WHERE post.post_root_id IN (SELECT post.id FROM post WHERE post.post_parent_id IS NULL AND post.thread_id = $1 AND post.post_root_id > (SELECT post.post_root_id FROM post WHERE post.id = $2) ORDER BY post.id LIMIT $3) ORDER BY post.post_root_id, post.path_, post.created, post.id;",
					thread.Id, since, limit)
			} else {
				rows, err = DBConnection.Query("SELECT post.id, post.profile_nickname, post.created, post.is_edited, post.message, post.post_parent_id FROM post WHERE post.post_root_id IN (SELECT post.id FROM post WHERE post.post_parent_id IS NULL AND post.thread_id = $1 AND post.post_root_id < (SELECT post.post_root_id FROM post WHERE post.id = $2) ORDER BY post.id DESC LIMIT $3) ORDER BY post.post_root_id DESC, post.path_, post.created, post.id;",
					thread.Id, since, limit)
			}
		}
		break
	default: //flat
		if since == "" {
			rows, err = DBConnection.Query(fmt.Sprintf("SELECT post.id, post.profile_nickname, post.created, post.is_edited, post.message, post.post_parent_id FROM post WHERE post.thread_id = $1 ORDER BY post.created %s, post.id %s LIMIT $2;", desc, desc),
				thread.Id, limit)
		} else {
			if desc == "" {
				rows, err = DBConnection.Query("SELECT post.id, post.profile_nickname, post.created, post.is_edited, post.message, post.post_parent_id FROM post WHERE post.thread_id = $1 AND post.id > $2 ORDER BY post.created, post.id LIMIT $3;",
					thread.Id, since, limit)
			} else {
				rows, err = DBConnection.Query("SELECT post.id, post.profile_nickname, post.created, post.is_edited, post.message, post.post_parent_id FROM post WHERE post.thread_id = $1 AND post.id < $2 ORDER BY post.created DESC, post.id DESC LIMIT $3;",
					thread.Id, since, limit)
			}
		}
		break
	}
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			panic(err)
		}
	}()

	var posts = make([]Post, 0)
	for rows.Next() {
		var post Post
		var parentPostId sql.NullInt64
		if err := rows.Scan(&post.Id, &post.ProfileNickname, &post.Created, &post.IsEdited, &post.Message,
			&parentPostId); err != nil {
			panic(err)
		}
		if parentPostId.Valid {
			post.ParentPost = uint64(parentPostId.Int64)
		}

		post.ForumSlug = thread.ForumSlug
		post.ThreadId = thread.Id

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
	if _, err := strconv.Atoi(slugOrId); err == nil { //TODO: тут какая-то хрень, если делать Exec
		if err := DBConnection.QueryRow("INSERT INTO vote (profile_id, thread_id, voice) SELECT profile.id, thread.id, $3 FROM profile, thread WHERE profile.nickname = $1 AND thread.id = $2 ON CONFLICT (profile_id, thread_id) DO UPDATE SET voice = $3 RETURNING vote.thread_id;",
			vote.ProfileNickname, slugOrId, vote.Voice).Scan(&vote.ThreadId); err != nil {
			return context.JSON(http.StatusNotFound, Error{
				Message: "Can't find user by nickname " + vote.ProfileNickname + " or thread by id " + slugOrId,
			})
		}
	} else {
		if err = DBConnection.QueryRow("INSERT INTO vote (profile_id, thread_id, voice) SELECT profile.id, thread.id, $3 FROM profile, thread WHERE profile.nickname = $1 AND thread.slug = $2 ON CONFLICT (profile_id, thread_id) DO UPDATE SET voice = $3 RETURNING vote.thread_id;",
			vote.ProfileNickname, slugOrId, vote.Voice).Scan(&vote.ThreadId); err != nil {
			return context.JSON(http.StatusNotFound, Error{
				Message: "Can't find user by nickname " + vote.ProfileNickname + " or thread by slug " + slugOrId,
			})
		}
	}

	var threadSlug sql.NullString
	if err := DBConnection.QueryRow("SELECT thread.id, thread.profile_nickname, thread.created, thread.forum_slug, thread.message, thread.slug, thread.title, thread.votes FROM thread WHERE thread.id = $1;",
		vote.ThreadId).Scan(&thread.Id, &thread.ProfileNickname, &thread.Created, &thread.ForumSlug, &thread.Message,
		&threadSlug, &thread.Title, &thread.Votes); err != nil {
		panic(err)
	}

	if threadSlug.Valid {
		thread.Slug = threadSlug.String
	}

	return context.JSON(http.StatusOK, thread)
}

func UserCreate(context echo.Context) error {
	var profile Profile
	if err := context.Bind(&profile); err != nil {
		panic(err)
	}
	profile.Nickname = context.Param("nickname")

	_, err := DBConnection.Exec("INSERT INTO profile (nickname, about, email, fullname) VALUES ($1, $2, $3, $4);",
		profile.Nickname, profile.About, profile.Email, profile.Fullname)
	if err == nil {
		return context.JSON(http.StatusCreated, profile)
	}

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

	return context.JSON(http.StatusConflict, existingProfiles)
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

func UserUpdate(context echo.Context) error { //TODO: тоже можно сократить количество походов в СУБД, но есть ли смысл? это update...
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
