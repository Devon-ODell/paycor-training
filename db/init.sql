-- Create a simple table to store user info fetched from Paycor
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,                  -- Internal database ID
    paycor_id VARCHAR(255) UNIQUE NOT NULL, -- Paycor's unique identifier for the user/employee
    first_name VARCHAR(255),
    last_name VARCHAR(255),
    -- Add other columns based on the data you fetch from Paycor API
    -- e.g., email VARCHAR(255), department VARCHAR(100), etc.
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    fetched_at TIMESTAMP WITH TIME ZONE -- Track when the data was last fetched/updated
);

-- Optional: Add indexes for performance
CREATE INDEX IF NOT EXISTS idx_users_paycor_id ON users(paycor_id);

-- You might want a table to store OAuth tokens securely in a real application
-- CREATE TABLE IF NOT EXISTS oauth_tokens (
--     user_id VARCHAR(255) PRIMARY KEY, -- Correlate with your app's user or a unique identifier
--     access_token TEXT NOT NULL,
--     refresh_token TEXT,
--     token_type VARCHAR(50),
--     expiry TIMESTAMP WITH TIME ZONE,
--     updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
-- );