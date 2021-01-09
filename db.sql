\c forums;

DROP SCHEMA public CASCADE;
CREATE SCHEMA public;

CREATE EXTENSION citext;

CREATE TABLE profile (
	nickname citext COLLATE "C" NOT NULL PRIMARY KEY,
	about text NOT NULL DEFAULT '',
	email citext NOT NULL UNIQUE,
	fullname varchar(100) NOT NULL
);

CREATE TABLE forum (
	slug citext NOT NULL PRIMARY KEY,
	title varchar(100) NOT NULL,
	profile_nickname citext NOT NULL REFERENCES profile (nickname) ON DELETE CASCADE,
	threads INT NOT NULL DEFAULT 0,
	posts INT NOT NULL DEFAULT 0
);

CREATE TABLE thread (
	id serial NOT NULL PRIMARY KEY,
	profile_nickname citext NOT NULL REFERENCES profile (nickname) ON DELETE CASCADE,
	created timestamptz NOT NULL,
	forum_slug citext NOT NULL REFERENCES forum (slug) ON DELETE CASCADE,
	message text NOT NULL,
	slug citext NOT NULL,
	title varchar(100) NOT NULL,
    votes INT NOT NULL DEFAULT 0
);

CREATE TABLE post (
	id serial NOT NULL PRIMARY KEY,
	profile_nickname citext NOT NULL REFERENCES profile (nickname) ON DELETE CASCADE,
	created timestamp NOT NULL,
	is_edited boolean NOT NULL DEFAULT false,
	message text NOT NULL,
	posts integer[] NOT NULL,
	thread_id int NOT NULL REFERENCES thread (id) ON DELETE CASCADE,
    forum_slug citext NOT NULL REFERENCES forum (slug) ON DELETE CASCADE
);

CREATE TABLE vote (
	profile_nickname citext NOT NULL REFERENCES profile (nickname) ON DELETE CASCADE,
	thread_id int NOT NULL REFERENCES thread (id) ON DELETE CASCADE,
	voice INT NOT NULL,
	PRIMARY KEY(profile_nickname, thread_id)
);

CREATE INDEX ON thread (slug);

CREATE INDEX ON thread (forum_slug, profile_nickname);

CREATE INDEX ON profile (email)
    INCLUDE (nickname);

CREATE INDEX ON thread (forum_slug, created);

CREATE INDEX ON post (forum_slug, profile_nickname);

CREATE INDEX ON post (id, profile_nickname);

CREATE INDEX ON post (id, forum_slug);

CREATE INDEX ON post (id, thread_id, posts);

CREATE INDEX ON post (posts, id);

CREATE INDEX ON post (thread_id, posts, created, id);

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
    IF NEW.posts[1] <> 0 THEN
        NEW.posts := (SELECT post.posts FROM post WHERE post.id = NEW.posts[1] AND post.thread_id = NEW.thread_id) || ARRAY[NEW.id];
        IF array_length(NEW.posts, 1) = 1 THEN
            RAISE 'Parent post in another thread';
        END IF;
    ELSE
        NEW.posts[1] := NEW.id;
    END IF;
    --NEW.posts := NEW.posts || ARRAY[NEW.id];
    RETURN NEW;
END;
$trigger_post_before_insert$ LANGUAGE plpgsql;

CREATE TRIGGER before_insert BEFORE INSERT
    ON post
    FOR EACH ROW
    EXECUTE PROCEDURE trigger_post_before_insert();

CREATE FUNCTION trigger_post_after_insert() --TODO: возможно, лучше объединить этот триггер в один (вместе с trigger_post_before_insert)
    RETURNS trigger AS $trigger_post_after_insert$
BEGIN
    UPDATE forum SET posts = posts + 1 WHERE forum.slug = NEW.forum_slug;
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
BEGIN --TODO: IF OLD.voice != NEW.voice... не только здесь так?...
    UPDATE thread SET votes = votes - OLD.voice + NEW.voice WHERE thread.id = NEW.thread_id;
    RETURN OLD;
END;
$trigger_vote_after_update$ LANGUAGE plpgsql;

CREATE TRIGGER after_update AFTER UPDATE
    ON vote
    FOR EACH ROW
    EXECUTE PROCEDURE trigger_vote_after_update();

CREATE EXTENSION pg_stat_statements; --TODO: при нагрузочном тестировании убрать эти 3 строчки, они только для отладки
ANALYZE;
SELECT pg_stat_statements_reset();
