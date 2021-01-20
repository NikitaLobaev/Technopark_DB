\c forums;

ALTER SYSTEM SET max_wal_senders = 0;
ALTER SYSTEM SET wal_level = minimal;
ALTER SYSTEM SET fsync = FALSE;
ALTER SYSTEM SET full_page_writes = FALSE;
ALTER SYSTEM SET synchronous_commit = FALSE;
ALTER SYSTEM SET archive_mode = FALSE;
ALTER SYSTEM SET shared_buffers = '400 MB';
ALTER SYSTEM SET effective_cache_size = '1 GB';
ALTER SYSTEM SET work_mem = '32 MB';

SELECT pg_reload_conf();

DROP SCHEMA public CASCADE;
CREATE SCHEMA public;

CREATE EXTENSION citext;

CREATE TYPE voice AS ENUM ('1', '-1');

CREATE UNLOGGED TABLE profile (
	nickname citext COLLATE "C" NOT NULL PRIMARY KEY,
	about TEXT NOT NULL DEFAULT '',
	email citext NOT NULL UNIQUE,
	fullname TEXT NOT NULL
);

CREATE UNLOGGED TABLE forum (
	slug citext NOT NULL PRIMARY KEY,
	title TEXT NOT NULL,
	profile_nickname citext NOT NULL,
	threads INT NOT NULL DEFAULT 0,
	posts INT NOT NULL DEFAULT 0
);

CREATE UNLOGGED TABLE thread (
	id SERIAL PRIMARY KEY,
    profile_nickname citext NOT NULL,
	created TIMESTAMPTZ NOT NULL,
	forum_slug citext NOT NULL,
	message TEXT NOT NULL,
	slug citext,
	title TEXT NOT NULL,
    votes INT NOT NULL DEFAULT 0
);

CREATE UNLOGGED TABLE post (
	id BIGSERIAL PRIMARY KEY,
    profile_nickname citext NOT NULL,
	created TIMESTAMP NOT NULL,
	is_edited BOOLEAN NOT NULL DEFAULT FALSE,
	message TEXT NOT NULL,
    post_root_id BIGINT NOT NULL,
	post_parent_id BIGINT,
    path_ BIGINT[] NOT NULL,
	thread_id INT NOT NULL,
    forum_slug citext NOT NULL
);

CREATE UNLOGGED TABLE vote (
    profile_nickname citext NOT NULL,
	thread_id INT NOT NULL,
    PRIMARY KEY (profile_nickname, thread_id),
	voice voice NOT NULL
);

CREATE UNLOGGED TABLE forum_user (
    forum_slug citext NOT NULL,
    profile_nickname citext NOT NULL,
    PRIMARY KEY (forum_slug, profile_nickname)
);

CREATE INDEX ON profile USING hash (nickname);
CREATE INDEX ON profile USING hash (email);

CREATE INDEX ON forum USING hash (slug);

CREATE INDEX ON thread USING hash (id);
CREATE INDEX ON thread USING hash (slug)
    WHERE slug IS NOT NULL;
CREATE INDEX ON thread USING hash (forum_slug);

CREATE INDEX ON post USING hash (id);
CREATE INDEX ON post USING hash (thread_id);
CREATE INDEX ON post (thread_id, path_, created, id);
CREATE INDEX ON post USING hash (post_root_id);
CREATE INDEX ON post (thread_id, post_root_id)
    WHERE post_parent_id IS NULL;
CREATE INDEX ON post (post_root_id, path_, created);
CREATE INDEX ON post (thread_id, id, created);

CREATE INDEX ON forum_user USING hash (forum_slug);
CREATE INDEX ON forum_user USING hash (profile_nickname);

CREATE FUNCTION trigger_thread_after_insert()
    RETURNS TRIGGER
AS $trigger_thread_after_insert$
BEGIN
    UPDATE forum SET threads = threads + 1 WHERE forum.slug = NEW.forum_slug;
    INSERT INTO forum_user (forum_slug, profile_nickname)
    VALUES (NEW.forum_slug, NEW.profile_nickname)
    ON CONFLICT (forum_slug, profile_nickname) DO NOTHING;
    RETURN NEW;
END;
$trigger_thread_after_insert$ LANGUAGE plpgsql;

CREATE TRIGGER after_insert AFTER INSERT
    ON thread
    FOR EACH ROW
    EXECUTE PROCEDURE trigger_thread_after_insert();

CREATE FUNCTION trigger_post_before_insert()
    RETURNS TRIGGER
AS $trigger_post_before_insert$
BEGIN
    IF NEW.post_parent_id != 0 THEN
        NEW.path_ := (SELECT post.path_ FROM post WHERE post.thread_id = NEW.thread_id
                                                    AND post.id = NEW.post_parent_id) || ARRAY[NEW.id];
        IF cardinality(NEW.path_) = 1 THEN
            RAISE 'Parent post is in another thread';
        END IF;
        NEW.post_root_id := NEW.path_[1];
    ELSE
        NEW.post_parent_id := NULL;
        NEW.post_root_id := NEW.id;
        NEW.path_ := ARRAY[NEW.id];
    END IF;
    RETURN NEW;
END;
$trigger_post_before_insert$ LANGUAGE plpgsql;

CREATE TRIGGER before_insert BEFORE INSERT
    ON post
    FOR EACH ROW
    EXECUTE PROCEDURE trigger_post_before_insert();

CREATE FUNCTION trigger_post_after_insert()
    RETURNS TRIGGER
AS $trigger_post_after_insert$
BEGIN
    UPDATE forum SET posts = posts + 1 WHERE forum.slug = NEW.forum_slug;
    INSERT INTO forum_user (forum_slug, profile_nickname)
    VALUES (NEW.forum_slug, NEW.profile_nickname)
    ON CONFLICT (forum_slug, profile_nickname) DO NOTHING;
    RETURN NEW;
END;
$trigger_post_after_insert$ LANGUAGE plpgsql;

CREATE TRIGGER after_insert AFTER INSERT
    ON post
    FOR EACH ROW
    EXECUTE PROCEDURE trigger_post_after_insert();

CREATE FUNCTION trigger_post_before_update()
    RETURNS TRIGGER
AS $trigger_post_before_insert$
BEGIN
    NEW.is_edited := TRUE;
    RETURN NEW;
END;
$trigger_post_before_insert$ LANGUAGE plpgsql;

CREATE TRIGGER before_update BEFORE UPDATE
    ON post
    FOR EACH ROW
EXECUTE PROCEDURE trigger_post_before_update();

CREATE FUNCTION trigger_vote_after_insert()
    RETURNS TRIGGER
AS $trigger_vote_after_insert$
BEGIN
    IF NEW.voice = '1' THEN
         UPDATE thread SET votes = votes + 1 WHERE thread.id = NEW.thread_id;
    ELSE
        UPDATE thread SET votes = votes - 1 WHERE thread.id = NEW.thread_id;
    END IF;
    RETURN NEW;
END;
$trigger_vote_after_insert$ LANGUAGE plpgsql;

CREATE TRIGGER after_insert AFTER INSERT
    ON vote
    FOR EACH ROW
    EXECUTE PROCEDURE trigger_vote_after_insert();

CREATE FUNCTION trigger_vote_after_update()
    RETURNS TRIGGER
AS $trigger_vote_after_update$
BEGIN
    IF OLD.voice != NEW.voice THEN
        IF NEW.voice = '1' THEN
            UPDATE thread SET votes = votes + 2 WHERE thread.id = NEW.thread_id;
        ELSE
            UPDATE thread SET votes = votes - 2 WHERE thread.id = NEW.thread_id;
        END IF;
    END IF;
    RETURN OLD;
END;
$trigger_vote_after_update$ LANGUAGE plpgsql;

CREATE TRIGGER after_update AFTER UPDATE
    ON vote
    FOR EACH ROW
    EXECUTE PROCEDURE trigger_vote_after_update();

CREATE FUNCTION clear()
    RETURNS VOID
AS $clear$
BEGIN
    TRUNCATE TABLE profile RESTART IDENTITY CASCADE;
    TRUNCATE TABLE forum RESTART IDENTITY CASCADE;
    TRUNCATE TABLE thread RESTART IDENTITY CASCADE;
    TRUNCATE TABLE post RESTART IDENTITY CASCADE;
    TRUNCATE TABLE vote RESTART IDENTITY CASCADE;
    TRUNCATE TABLE forum_user RESTART IDENTITY CASCADE;
END;
$clear$ LANGUAGE plpgsql;

/*CREATE EXTENSION pg_stat_statements;
ANALYZE;
SELECT pg_stat_statements_reset();*/
