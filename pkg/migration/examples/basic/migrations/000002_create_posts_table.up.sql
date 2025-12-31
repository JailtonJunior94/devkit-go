-- Create posts table
CREATE TABLE IF NOT EXISTS posts (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(500) NOT NULL,
    content TEXT,
    published BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Create index on user_id for faster joins
CREATE INDEX IF NOT EXISTS idx_posts_user_id ON posts(user_id);

-- Create index on published for filtering
CREATE INDEX IF NOT EXISTS idx_posts_published ON posts(published);

-- Create composite index for common queries
CREATE INDEX IF NOT EXISTS idx_posts_user_published ON posts(user_id, published);
