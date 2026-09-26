DROP TABLE IF EXISTS post_category CASCADE;
DROP TABLE IF EXISTS likes CASCADE;
DROP TABLE IF EXISTS comments CASCADE;
DROP TABLE IF EXISTS posts CASCADE;
DROP TABLE IF EXISTS profiles CASCADE;
DROP TABLE IF EXISTS sessions CASCADE;
DROP TABLE IF EXISTS medias CASCADE;
DROP TABLE IF EXISTS categories CASCADE;
DROP TABLE IF EXISTS accounts CASCADE;

DROP TYPE IF EXISTS target_type_enum;
DROP TYPE IF EXISTS reaction_enum;

CREATE TYPE target_type_enum AS ENUM ('post', 'comment');
CREATE TYPE reaction_enum AS ENUM ('like', 'dislike');

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    username VARCHAR(255) UNIQUE NOT NULL,
    password VARCHAR(255) NOT NULL,
    role VARCHAR(20) NOT NULL DEFAULT 'user',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TRIGGER trg_accounts_updated_at
    BEFORE UPDATE ON accounts
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE categories (
    id BIGSERIAL PRIMARY KEY,
    label VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE medias (
    id BIGSERIAL PRIMARY KEY,
    object_key TEXT NOT NULL,
    public_url TEXT NOT NULL,
    mime_type VARCHAR(255) NOT NULL,
    size BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TRIGGER trg_medias_updated_at
    BEFORE UPDATE ON medias
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    session_id VARCHAR(255) UNIQUE NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE UNIQUE INDEX idx_sessions_user_id ON sessions (user_id);

CREATE TABLE posts (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    media_id BIGINT NULL REFERENCES medias(id) ON DELETE RESTRICT,
    like_count INT NOT NULL DEFAULT 0,
    dislike_count INT NOT NULL DEFAULT 0,
    comment_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_posts_user_id ON posts (user_id);
CREATE INDEX idx_posts_created_at ON posts (created_at);
CREATE INDEX idx_posts_media_id ON posts (media_id);
CREATE INDEX idx_posts_like_count ON posts (like_count);
CREATE INDEX idx_posts_comment_count ON posts (comment_count);

CREATE TABLE profiles (
    account_id UUID PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    first_name VARCHAR(255) NOT NULL DEFAULT '',
    last_name VARCHAR(255) NOT NULL DEFAULT '',
    location VARCHAR(255) NOT NULL DEFAULT '',
    avatar_media_id BIGINT NULL REFERENCES medias(id) ON DELETE SET NULL,
    cover_media_id BIGINT NULL REFERENCES medias(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TRIGGER trg_profiles_updated_at
    BEFORE UPDATE ON profiles
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX idx_profiles_avatar_media_id ON profiles (avatar_media_id);
CREATE INDEX idx_profiles_cover_media_id ON profiles (cover_media_id);

CREATE TABLE post_category (
    id BIGSERIAL PRIMARY KEY,
    post_id BIGINT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    category_id BIGINT NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    CONSTRAINT unique_post_category UNIQUE (post_id, category_id)
);

CREATE INDEX idx_post_category_category_id ON post_category (category_id);
CREATE INDEX idx_post_category_category_post ON post_category (category_id, post_id);

CREATE TABLE comments (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    post_id BIGINT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    parent_comment_id BIGINT NULL REFERENCES comments(id) ON DELETE SET NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_comments_post_id ON comments (post_id);
CREATE INDEX idx_comments_user_id ON comments (user_id);
CREATE INDEX idx_comments_parent_comment_id ON comments (parent_comment_id);

CREATE TABLE likes (
    user_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    target_type target_type_enum NOT NULL,
    target_id BIGINT NOT NULL,
    reaction reaction_enum NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, target_type, target_id)
);

CREATE INDEX idx_likes_target ON likes (target_type, target_id);
CREATE INDEX idx_likes_target_reaction ON likes (target_type, target_id, reaction);
