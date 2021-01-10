\c forums;

DROP SCHEMA public CASCADE;
CREATE SCHEMA public;

CREATE EXTENSION citext;

CREATE UNLOGGED TABLE profile (
	nickname citext COLLATE "C" NOT NULL PRIMARY KEY,
	about text NOT NULL DEFAULT '',
	email citext NOT NULL UNIQUE,
	fullname varchar(100) NOT NULL
);

CREATE UNLOGGED TABLE forum (
	slug citext NOT NULL PRIMARY KEY,
	title varchar(100) NOT NULL,
	profile_nickname citext NOT NULL REFERENCES profile (nickname) ON DELETE CASCADE,
	threads INT NOT NULL DEFAULT 0,
	posts INT NOT NULL DEFAULT 0
);

CREATE UNLOGGED TABLE thread (
	id serial NOT NULL PRIMARY KEY,
	profile_nickname citext NOT NULL REFERENCES profile (nickname) ON DELETE CASCADE,
	created timestamptz NOT NULL,
	forum_slug citext NOT NULL REFERENCES forum (slug) ON DELETE CASCADE,
	message text NOT NULL,
	slug citext NOT NULL,
	title varchar(100) NOT NULL,
    votes INT NOT NULL DEFAULT 0
);

CREATE UNLOGGED TABLE post ( --TODO: возможно, добавить доп. поле parent_post_id... ?
	id bigserial NOT NULL PRIMARY KEY,
	profile_nickname citext NOT NULL REFERENCES profile (nickname) ON DELETE CASCADE,
	created timestamp NOT NULL,
	is_edited boolean NOT NULL DEFAULT false,
	message text NOT NULL,
	path bigint[] NOT NULL,
	thread_id int NOT NULL REFERENCES thread (id) ON DELETE CASCADE,
    forum_slug citext NOT NULL REFERENCES forum (slug) ON DELETE CASCADE
);

CREATE UNLOGGED TABLE vote (
	profile_nickname citext NOT NULL REFERENCES profile (nickname) ON DELETE CASCADE,
	thread_id int NOT NULL REFERENCES thread (id) ON DELETE CASCADE,
	voice INT NOT NULL,
	PRIMARY KEY(profile_nickname, thread_id)
);

CREATE UNLOGGED TABLE forum_user (
    forum_slug citext NOT NULL REFERENCES forum (slug) ON DELETE CASCADE,
    profile_nickname citext NOT NULL REFERENCES profile (nickname) ON DELETE CASCADE,
    PRIMARY KEY (forum_slug, profile_nickname)
);

CREATE INDEX ON profile USING hash (nickname);
CREATE INDEX ON profile USING hash (email);

CREATE INDEX ON forum USING hash (slug);

CREATE INDEX ON thread USING hash (id);
CREATE INDEX ON thread (forum_slug, created);
CREATE INDEX ON thread (created);
CREATE INDEX ON thread USING hash (slug)
    WHERE slug != '';

CREATE INDEX ON post (thread_id);
CREATE INDEX ON post (path, created, id);
CREATE INDEX ON post (path);
CREATE INDEX ON post (thread_id, path);
CREATE INDEX ON post (thread_id, array_length(path, 1))
    WHERE array_length(path, 1) = 1;
CREATE INDEX ON post ((path[1]));
--CREATE INDEX ON post ((path[1]), path, created, id);
CREATE INDEX ON post ((path[1]), (path[2:]), created, id);
CREATE INDEX ON post (thread_id, array_length(path, 1), (path[1]))
    WHERE array_length(path, 1) = 1;
CREATE INDEX ON post (created, id);
CREATE INDEX ON post (thread_id, id);

CREATE INDEX ON forum_user (profile_nickname, forum_slug);

/*CREATE INDEX ON profile USING hash (nickname);
CREATE INDEX ON profile USING hash (email);

CREATE INDEX ON forum (slug, title, profile_nickname, posts, threads);
CREATE INDEX ON forum USING hash (slug);

CREATE INDEX ON thread (forum_slug, created);
CREATE INDEX ON thread (created);
CREATE INDEX ON thread USING hash (forum_slug);
CREATE INDEX ON thread USING hash (id);

CREATE INDEX ON post (id);
CREATE INDEX ON post (thread_id, created, id);
CREATE INDEX ON post (thread_id, id);
CREATE INDEX ON post (thread_id, path);
CREATE INDEX ON post (thread_id, (path[1]), path);
CREATE INDEX ON post ((path[1]), path);

CREATE UNIQUE INDEX ON vote (thread_id, profile_nickname);

CREATE INDEX ON forum_user (profile_nickname);*/

CREATE FUNCTION trigger_thread_after_insert()
    RETURNS trigger AS $trigger_thread_after_insert$
BEGIN
    UPDATE forum SET threads = threads + 1 WHERE forum.slug = NEW.forum_slug;
    INSERT INTO forum_user (forum_slug, profile_nickname) VALUES (NEW.forum_slug, NEW.profile_nickname)
    ON CONFLICT DO NOTHING;
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
    IF NEW.path[1] <> 0 THEN
        NEW.path := (SELECT post.path FROM post WHERE post.id = NEW.path[1] AND post.thread_id = NEW.thread_id) || ARRAY[NEW.id];
        IF array_length(NEW.path, 1) = 1 THEN
            RAISE 'Parent post is in another thread';
        END IF;
    ELSE
        NEW.path[1] := NEW.id;
    END IF;
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
    INSERT INTO forum_user (forum_slug, profile_nickname) VALUES (NEW.forum_slug, NEW.profile_nickname)
    ON CONFLICT DO NOTHING;
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
    IF OLD.voice != NEW.voice THEN
        UPDATE thread SET votes = votes - OLD.voice + NEW.voice WHERE thread.id = NEW.thread_id;
    END IF;
    RETURN OLD;
END;
$trigger_vote_after_update$ LANGUAGE plpgsql;

CREATE TRIGGER after_update AFTER UPDATE
    ON vote
    FOR EACH ROW
    EXECUTE PROCEDURE trigger_vote_after_update();

/*CREATE EXTENSION pg_stat_statements;
ANALYZE;
SELECT pg_stat_statements_reset();*/
