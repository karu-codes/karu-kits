-- Create posts table
CREATE TABLE IF NOT EXISTS posts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(500) NOT NULL,
    content TEXT,
    slug VARCHAR(500) NOT NULL UNIQUE,
    published BOOLEAN NOT NULL DEFAULT false,
    published_at TIMESTAMPTZ,
    view_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Create index on user_id for faster lookups
CREATE INDEX IF NOT EXISTS idx_posts_user_id ON posts(user_id);

-- Create index on slug for faster lookups
CREATE INDEX IF NOT EXISTS idx_posts_slug ON posts(slug);

-- Create index on published for filtering
CREATE INDEX IF NOT EXISTS idx_posts_published ON posts(published);

-- Create index on published_at for sorting
CREATE INDEX IF NOT EXISTS idx_posts_published_at ON posts(published_at DESC) WHERE published = true;
