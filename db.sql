\c forums;

DROP SCHEMA public CASCADE;
CREATE SCHEMA public;

CREATE EXTENSION citext;

CREATE TYPE voice AS ENUM ('1', '-1');

CREATE UNLOGGED TABLE profile (
    id SERIAL PRIMARY KEY,
	nickname citext COLLATE "C" NOT NULL UNIQUE,
	about TEXT NOT NULL DEFAULT '',
	email citext NOT NULL UNIQUE,
	fullname VARCHAR(100) NOT NULL
);

CREATE UNLOGGED TABLE forum (
    id SERIAL PRIMARY KEY,
	slug citext NOT NULL UNIQUE,
	title VARCHAR(100) NOT NULL,
	profile_id INT NOT NULL REFERENCES profile (id) ON DELETE CASCADE,
	profile_nickname citext NOT NULL,
	threads INT NOT NULL DEFAULT 0,
	posts INT NOT NULL DEFAULT 0
);

CREATE UNLOGGED TABLE thread (
	id SERIAL PRIMARY KEY,
    profile_id INT NOT NULL REFERENCES profile (id) ON DELETE CASCADE,
    profile_nickname citext NOT NULL,
	created TIMESTAMPTZ NOT NULL,
	forum_id INT NOT NULL REFERENCES forum (id) ON DELETE CASCADE,
	forum_slug citext NOT NULL,
	message TEXT NOT NULL,
	slug citext NOT NULL,--TODO: возможно, лучше будет сделать NULL UNIQUE
	title VARCHAR(100) NOT NULL,
    votes INT NOT NULL DEFAULT 0
);

CREATE UNLOGGED TABLE post ( --TODO: возможно, добавить доп. поле parent_post_id... ?
	id BIGSERIAL PRIMARY KEY,
    profile_id INT NOT NULL REFERENCES profile (id) ON DELETE CASCADE,
    profile_nickname citext NOT NULL,
	created TIMESTAMP NOT NULL,
	is_edited BOOLEAN NOT NULL DEFAULT FALSE,
	message TEXT NOT NULL,
    posts_path BIGINT[] NOT NULL,
	thread_id INT NOT NULL REFERENCES thread (id) ON DELETE CASCADE,
    forum_id INT NOT NULL REFERENCES forum (id) ON DELETE CASCADE,
    forum_slug citext NOT NULL
);

CREATE UNLOGGED TABLE vote (
    profile_id INT NOT NULL REFERENCES profile (id) ON DELETE CASCADE,
	thread_id INT NOT NULL REFERENCES thread (id) ON DELETE CASCADE,
	voice voice NOT NULL,
	PRIMARY KEY(profile_id, thread_id)
);

CREATE UNLOGGED TABLE forum_user (
    forum_id INT NOT NULL REFERENCES forum (id) ON DELETE CASCADE,
    profile_id INT NOT NULL REFERENCES profile (id) ON DELETE CASCADE,
    PRIMARY KEY (forum_id, profile_id)
);

CREATE INDEX ON profile USING hash (nickname);

CREATE INDEX ON forum USING hash (slug);

CREATE INDEX ON thread USING hash (forum_id);
CREATE INDEX ON thread USING hash (id);
CREATE INDEX ON thread USING hash (slug)
    WHERE slug != '';
CREATE INDEX ON thread (created);
CREATE INDEX ON thread (forum_id, created);

CREATE INDEX ON post USING hash (id);
CREATE INDEX ON post USING hash (thread_id);
CREATE INDEX ON post USING hash (thread_id)
    WHERE cardinality(posts_path) = 1;
CREATE INDEX ON post USING hash ((posts_path[1]));
CREATE INDEX ON post (created, id);
CREATE INDEX ON post (posts_path, created, id);
CREATE INDEX ON post ((posts_path[1]), posts_path, created, id);
CREATE INDEX ON post ((posts_path[1]), (posts_path[2:]), created, id);
CREATE INDEX ON post (thread_id, id);
CREATE INDEX ON post (thread_id, id)
    WHERE cardinality(posts_path) = 1;
CREATE INDEX ON post (thread_id, posts_path, created, id);
CREATE INDEX ON post (thread_id, (posts_path[1]))
    WHERE cardinality(posts_path) = 1;

CREATE INDEX ON forum_user USING hash (forum_id);
CREATE INDEX ON forum_user USING hash (profile_id);
CREATE INDEX ON forum_user (forum_id, profile_id);
CREATE INDEX ON forum_user (profile_id, forum_id);

CREATE FUNCTION trigger_thread_after_insert()
    RETURNS TRIGGER AS $trigger_thread_after_insert$
BEGIN
    UPDATE forum SET threads = threads + 1 WHERE forum.slug = NEW.forum_slug;
    INSERT INTO forum_user (forum_id, profile_id)--forum_slug,
    VALUES (NEW.forum_id, NEW.profile_id)--, NEW.forum_slug
    ON CONFLICT (forum_id, profile_id) DO NOTHING;
    RETURN NEW;
END;
$trigger_thread_after_insert$ LANGUAGE plpgsql;

CREATE TRIGGER after_insert AFTER INSERT
    ON thread
    FOR EACH ROW
    EXECUTE PROCEDURE trigger_thread_after_insert();

CREATE FUNCTION trigger_post_before_insert()
    RETURNS TRIGGER AS $trigger_post_before_insert$
BEGIN
    IF NEW.posts_path[1] <> 0 THEN
        NEW.posts_path := (SELECT post.posts_path FROM post WHERE post.id = NEW.posts_path[1]
                                                              AND post.thread_id = NEW.thread_id) || ARRAY[NEW.id];
        IF cardinality(NEW.posts_path) = 1 THEN
            RAISE 'Parent post is in another thread';
        END IF;
    ELSE
        NEW.posts_path[1] := NEW.id;
    END IF;
    RETURN NEW;
END;
$trigger_post_before_insert$ LANGUAGE plpgsql;

CREATE TRIGGER before_insert BEFORE INSERT
    ON post
    FOR EACH ROW
    EXECUTE PROCEDURE trigger_post_before_insert();

CREATE FUNCTION trigger_post_after_insert()
    RETURNS TRIGGER AS $trigger_post_after_insert$
BEGIN
    UPDATE forum SET posts = posts + 1 WHERE forum.slug = NEW.forum_slug;
    INSERT INTO forum_user (forum_id, profile_id)--, forum_slug
    VALUES (NEW.forum_id, NEW.profile_id)--, NEW.forum_slug
    ON CONFLICT (forum_id, profile_id) DO NOTHING;
    RETURN NEW;
END;
$trigger_post_after_insert$ LANGUAGE plpgsql;

CREATE TRIGGER after_insert AFTER INSERT
    ON post
    FOR EACH ROW
    EXECUTE PROCEDURE trigger_post_after_insert();

CREATE FUNCTION trigger_post_before_update()
    RETURNS TRIGGER AS $trigger_post_before_insert$
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
    RETURNS TRIGGER AS $trigger_vote_after_insert$
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
    RETURNS TRIGGER AS $trigger_vote_after_update$
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

/*CREATE EXTENSION pg_stat_statements;
ANALYZE;
SELECT pg_stat_statements_reset();*/
