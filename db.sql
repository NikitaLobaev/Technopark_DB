DROP SCHEMA public CASCADE;
CREATE SCHEMA public;

CREATE EXTENSION citext;

CREATE TABLE profile (
	id serial NOT NULL PRIMARY KEY,
	nickname citext COLLATE "C" NOT NULL UNIQUE,
	about text NOT NULL DEFAULT '',
	email citext NOT NULL UNIQUE,
	fullname varchar(100) NOT NULL
);

CREATE TABLE forum (
	slug citext NOT NULL PRIMARY KEY,
	title varchar(100) NOT NULL,
	profile_id int NOT NULL REFERENCES profile (id) ON DELETE CASCADE,
	threads INT NOT NULL DEFAULT 0,
	posts INT NOT NULL DEFAULT 0
);

CREATE TABLE thread (
	id serial NOT NULL PRIMARY KEY,
	profile_id int NOT NULL REFERENCES profile (id) ON DELETE CASCADE,
	created timestamptz NOT NULL,
	forum_slug citext NOT NULL REFERENCES forum (slug) ON DELETE CASCADE,
	message text NOT NULL,
	slug citext NOT NULL,
	title varchar(100) NOT NULL,
    votes INT NOT NULL DEFAULT 0
);

CREATE INDEX ON thread (forum_slug);

CREATE INDEX ON thread (slug);

CREATE INDEX ON thread (forum_slug, created);

CREATE TABLE post (
	id serial NOT NULL PRIMARY KEY,
	profile_id int NOT NULL REFERENCES profile (id) ON DELETE CASCADE,
	created timestamp NOT NULL,
	is_edited boolean NOT NULL DEFAULT false,
	message text NOT NULL,
	posts integer[] NOT NULL,
	thread_id int NOT NULL REFERENCES thread (id) ON DELETE CASCADE
);

CREATE INDEX ON post (thread_id);

/*CREATE INDEX ON post (id, thread_id)
    INCLUDE (posts);*/

CREATE TABLE vote (
	profile_id int NOT NULL REFERENCES profile (id) ON DELETE CASCADE,
	thread_id int NOT NULL REFERENCES thread (id) ON DELETE CASCADE,
	voice INT NOT NULL,
	PRIMARY KEY(profile_id, thread_id)
);

CREATE FUNCTION trigger_thread_after_insert()
    RETURNS trigger AS $trigger_thread_after_insert$
BEGIN
    UPDATE forum SET threads = threads + 1 WHERE forum.slug = NEW.forum_slug;
    RETURN NEW;
END;
$trigger_thread_after_insert$ LANGUAGE plpgsql;

CREATE TRIGGER after_insert AFTER INSERT
    ON thread
    FOR EACH ROW
    EXECUTE PROCEDURE trigger_thread_after_insert();

CREATE FUNCTION trigger_post_before_insert()
    RETURNS trigger AS $trigger_post_before_insert$
BEGIN
    NEW.posts := NEW.posts || ARRAY[NEW.id];
    RETURN NEW;
END;
$trigger_post_before_insert$ LANGUAGE plpgsql;

CREATE TRIGGER before_insert BEFORE INSERT
    ON post
    FOR EACH ROW
    EXECUTE PROCEDURE trigger_post_before_insert();

CREATE FUNCTION trigger_post_after_insert()
    RETURNS trigger AS $trigger_post_after_insert$
BEGIN
    UPDATE forum SET posts = posts + 1 WHERE forum.slug =
                                             (SELECT thread.forum_slug FROM thread WHERE thread.id = NEW.thread_id);
    RETURN NEW;
END;
$trigger_post_after_insert$ LANGUAGE plpgsql;

CREATE TRIGGER after_insert AFTER INSERT
    ON post
    FOR EACH ROW
    EXECUTE PROCEDURE trigger_post_after_insert();

CREATE FUNCTION trigger_vote_after_insert()
    RETURNS trigger AS $trigger_vote_after_insert$
BEGIN
    UPDATE thread SET votes = votes + NEW.voice WHERE thread.id = NEW.thread_id;
    RETURN NEW;
END;
$trigger_vote_after_insert$ LANGUAGE plpgsql;

CREATE TRIGGER after_insert AFTER INSERT --TODO: точно AFTER INSERT? может быть BEFORE INSERT?...
    ON vote
    FOR EACH ROW
    EXECUTE PROCEDURE trigger_vote_after_insert();

CREATE FUNCTION trigger_vote_after_update()
    RETURNS trigger AS $trigger_vote_after_update$
BEGIN
    UPDATE thread SET votes = votes - OLD.voice + NEW.voice WHERE thread.id = NEW.thread_id;
    RETURN OLD;
END;
$trigger_vote_after_update$ LANGUAGE plpgsql;

CREATE TRIGGER after_update AFTER UPDATE
    ON vote
    FOR EACH ROW
    EXECUTE PROCEDURE trigger_vote_after_update();
