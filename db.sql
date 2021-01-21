\c forums;

ALTER SYSTEM SET max_wal_senders = 0;
ALTER SYSTEM SET wal_level = minimal;
ALTER SYSTEM SET fsync = OFF;
ALTER SYSTEM SET full_page_writes = OFF;
ALTER SYSTEM SET synchronous_commit = OFF;
ALTER SYSTEM SET archive_mode = OFF;
ALTER SYSTEM SET shared_buffers = '400 MB';
ALTER SYSTEM SET effective_cache_size = '1 GB';
ALTER SYSTEM SET work_mem = '32 MB';

SELECT pg_reload_conf();

DROP SCHEMA public CASCADE;
CREATE SCHEMA public;

CREATE EXTENSION citext;

CREATE TYPE voice AS ENUM ('1', '-1');

CREATE UNLOGGED TABLE profile (
    id SERIAL PRIMARY KEY,
    nickname citext COLLATE "C" NOT NULL UNIQUE,
    about TEXT NOT NULL DEFAULT '',
    email citext NOT NULL UNIQUE,
    fullname TEXT NOT NULL
);

CREATE UNLOGGED TABLE forum (
    slug citext NOT NULL PRIMARY KEY,
    title TEXT NOT NULL,
    profile_nickname citext NOT NULL REFERENCES profile (nickname) ON DELETE CASCADE,
    threads INT NOT NULL DEFAULT 0,
    posts INT NOT NULL DEFAULT 0
);

CREATE UNLOGGED TABLE thread (
    id SERIAL PRIMARY KEY,
    profile_nickname citext NOT NULL REFERENCES profile (nickname) ON DELETE CASCADE,
    created TIMESTAMPTZ NOT NULL,
    forum_slug citext NOT NULL REFERENCES forum ON DELETE CASCADE,
    message TEXT NOT NULL,
    slug citext UNIQUE,
    title TEXT NOT NULL,
    votes INT NOT NULL DEFAULT 0
);

CREATE UNLOGGED TABLE post (
    id BIGSERIAL PRIMARY KEY,
    profile_nickname citext NOT NULL REFERENCES profile (nickname) ON DELETE CASCADE,
    created TIMESTAMP NOT NULL,
    is_edited BOOLEAN NOT NULL DEFAULT FALSE,
    message TEXT NOT NULL,
    post_root_id BIGINT NOT NULL REFERENCES post ON DELETE CASCADE,
    post_parent_id BIGINT REFERENCES post ON DELETE CASCADE,
    path_ BIGINT[] NOT NULL,
    thread_id INT NOT NULL REFERENCES thread ON DELETE CASCADE,
    forum_slug citext NOT NULL REFERENCES forum ON DELETE CASCADE
);

CREATE UNLOGGED TABLE vote (
    profile_id INT NOT NULL REFERENCES profile ON DELETE CASCADE,
    thread_id INT NOT NULL REFERENCES thread ON DELETE CASCADE,
    PRIMARY KEY (profile_id, thread_id),
    voice voice NOT NULL
);

CREATE UNLOGGED TABLE forum_user (
    forum_slug citext NOT NULL REFERENCES forum ON DELETE CASCADE,
    profile_nickname citext COLLATE "C" NOT NULL REFERENCES profile (nickname) ON DELETE CASCADE,
    PRIMARY KEY (forum_slug, profile_nickname),
    profile_about TEXT NOT NULL,
    profile_email citext NOT NULL REFERENCES profile (email) ON DELETE CASCADE ON UPDATE CASCADE,
    profile_fullname TEXT NOT NULL
);

CREATE INDEX ON profile USING hash (nickname);
CREATE INDEX ON profile USING hash (email);

CREATE INDEX ON forum USING hash (slug);

--CREATE INDEX ON thread USING hash (id);
CREATE INDEX ON thread USING hash (slug)
    WHERE slug IS NOT NULL;
CREATE INDEX ON thread USING hash (forum_slug);
CREATE INDEX ON thread (forum_slug, created);

--CREATE INDEX ON post USING hash (id);
CREATE INDEX ON post USING hash (thread_id);
CREATE INDEX ON post (thread_id, path_, created, id);
CREATE INDEX ON post (thread_id, id)
    INCLUDE (id)
    WHERE post_parent_id IS NULL;
CREATE INDEX ON post (thread_id, post_root_id, id)
    INCLUDE (id)
    WHERE post_parent_id IS NULL;
CREATE INDEX ON post USING hash (post_root_id);
CREATE INDEX ON post (post_root_id, path_, created, id);
CREATE INDEX ON post (thread_id, created, id);
/*CREATE INDEX ON post (thread_id, id, created)
    INCLUDE (id);*/

CREATE INDEX ON forum_user USING hash (forum_slug);
--CREATE INDEX ON forum_user USING hash (profile_nickname);
CREATE INDEX ON forum_user (profile_nickname, forum_slug);

CREATE FUNCTION trigger_profile_after_update()
    RETURNS TRIGGER
AS $trigger_profile_after_update$
BEGIN
    IF OLD.about != NEW.about OR OLD.fullname != NEW.fullname THEN
        UPDATE forum_user SET profile_about = NEW.about, profile_fullname = NEW.fullname
        WHERE forum_user.profile_nickname = NEW.nickname;
    END IF;
    RETURN NEW;
END;
$trigger_profile_after_update$ LANGUAGE plpgsql;

CREATE TRIGGER after_update AFTER INSERT
    ON profile
    FOR EACH ROW
EXECUTE PROCEDURE trigger_profile_after_update();

CREATE FUNCTION trigger_thread_before_insert()
    RETURNS TRIGGER
AS $trigger_thread_before_insert$
BEGIN
    IF NEW.slug = '' THEN
        NEW.slug := NULL;
    END IF;
    RETURN NEW;
END;
$trigger_thread_before_insert$ LANGUAGE plpgsql;

CREATE TRIGGER before_insert BEFORE INSERT
    ON thread
    FOR EACH ROW
EXECUTE PROCEDURE trigger_thread_before_insert();

CREATE FUNCTION trigger_thread_after_insert()
    RETURNS TRIGGER
AS $trigger_thread_after_insert$
BEGIN --TODO: нужно вынести INSERT INTO forum_user ... в дополнительную функцию (т.к. есть копипаст ниже)
    UPDATE forum SET threads = threads + 1 WHERE forum.slug = NEW.forum_slug;
    INSERT INTO forum_user (forum_slug, profile_nickname, profile_about, profile_email, profile_fullname)
    SELECT NEW.forum_slug, NEW.profile_nickname, profile.about, profile.email, profile.fullname FROM profile
    WHERE profile.nickname = NEW.profile_nickname
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
    INSERT INTO forum_user (forum_slug, profile_nickname, profile_about, profile_email, profile_fullname)
    SELECT NEW.forum_slug, NEW.profile_nickname, profile.about, profile.email, profile.fullname FROM profile
    WHERE profile.nickname = NEW.profile_nickname
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

/*PREPARE prepared_forum_get_one AS
    SELECT forum.slug, forum.title, forum.profile_nickname, forum.threads, forum.posts
    FROM forum
    WHERE forum.slug = $1;*/

/*CREATE FUNCTION clear()
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
$clear$ LANGUAGE plpgsql;*/

/*CREATE EXTENSION pg_stat_statements;
ANALYZE;
SELECT pg_stat_statements_reset();*/
