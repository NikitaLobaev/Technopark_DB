\c forums;

DROP TABLE IF EXISTS profiles, forums, threads, posts, votes CASCADE;

CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE profiles (
	id serial NOT NULL PRIMARY KEY,--NOT NULL точно нужно?
	nickname citext NOT NULL UNIQUE,
	about text NOT NULL DEFAULT '',
	email citext NOT NULL UNIQUE,
	fullname varchar(100) NOT NULL
);

CREATE TABLE forums (
	slug citext NOT NULL PRIMARY KEY,
	title varchar(100) NOT NULL,
	profile_id int NOT NULL REFERENCES profiles (id) ON DELETE CASCADE
);

CREATE TABLE threads (
	id serial NOT NULL PRIMARY KEY,
	profile_id int NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
	created timestamptz NOT NULL,
	forum_slug citext NOT NULL REFERENCES forums (slug) ON DELETE CASCADE,
	message text NOT NULL,
	slug citext NOT NULL,
	title varchar(100) NOT NULL
);

CREATE TABLE posts (
	id serial NOT NULL PRIMARY KEY,
	profile_id int NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
	created timestamp NOT NULL,--timestamptz
	is_edited boolean NOT NULL DEFAULT false,
	message text NOT NULL,
	posts integer[] NOT NULL,--post_id int REFERENCES posts (id) ON DELETE CASCADE; какое значение DEFAULT?
	thread_id int NOT NULL REFERENCES threads (id) ON DELETE CASCADE
);

CREATE TABLE votes (
	profile_id int NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
	thread_id int NOT NULL REFERENCES threads (id) ON DELETE CASCADE,
	voice int NOT NULL,
	PRIMARY KEY(profile_id, thread_id)
);

CREATE OR REPLACE FUNCTION trg_posts_parent_default()
    RETURNS trigger
    LANGUAGE plpgsql AS
$func$
BEGIN
    NEW.posts := NEW.posts || ARRAY[NEW.id];
    /*NEW.posts := CASE NEW.posts
        WHEN NULL THEN ARRAY[NEW.id]
            ELSE NEW.posts || ARRAY[NEW.id]
                END;*/
    RETURN NEW;
END
$func$;

CREATE TRIGGER b_default
    BEFORE INSERT ON posts
    FOR EACH ROW--IS IT NEEDED?
EXECUTE PROCEDURE trg_posts_parent_default();

GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO forums_user;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO forums_user;
