-- Database Initialization Script for Game Platform
-- Version: 2.1
-- Created: 2026-03-23

-- ==================== game_platform_db ====================

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    password VARCHAR(255) NOT NULL,
    email VARCHAR(100) UNIQUE,
    phone VARCHAR(20) UNIQUE,
    nickname VARCHAR(50),
    avatar VARCHAR(255),
    status TINYINT DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_status ON users(status);

-- User profiles table
CREATE TABLE IF NOT EXISTS user_profiles (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL UNIQUE,
    gender VARCHAR(10),
    birthday DATE,
    location VARCHAR(100),
    bio TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_user_profiles_user_id ON user_profiles(user_id);

-- Friends table
CREATE TABLE IF NOT EXISTS friends (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    friend_id BIGINT NOT NULL,
    status VARCHAR(20) DEFAULT 'pending',
    remark VARCHAR(50),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (user_id, friend_id)
);

CREATE INDEX idx_friends_user_id ON friends(user_id);
CREATE INDEX idx_friends_status ON friends(status);

-- Organizations table
CREATE TABLE IF NOT EXISTS organizations (
    id BIGSERIAL PRIMARY KEY,
    org_code VARCHAR(50) UNIQUE NOT NULL,
    org_name VARCHAR(100) NOT NULL,
    org_type VARCHAR(20) NOT NULL,
    contact_person VARCHAR(50),
    contact_email VARCHAR(100),
    contact_phone VARCHAR(20),
    logo_url VARCHAR(255),
    description TEXT,
    website VARCHAR(255),
    status TINYINT DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_organizations_org_code ON organizations(org_code);
CREATE INDEX idx_organizations_org_type ON organizations(org_type);

-- Organization members table
CREATE TABLE IF NOT EXISTS organization_members (
    id BIGSERIAL PRIMARY KEY,
    org_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    role VARCHAR(20) NOT NULL,
    status TINYINT DEFAULT 1,
    joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (org_id, user_id)
);

CREATE INDEX idx_org_members_org_id ON organization_members(org_id);
CREATE INDEX idx_org_members_user_id ON organization_members(user_id);

-- Roles table
CREATE TABLE IF NOT EXISTS roles (
    id BIGSERIAL PRIMARY KEY,
    role_name VARCHAR(50) NOT NULL,
    role_code VARCHAR(50) UNIQUE NOT NULL,
    description VARCHAR(200),
    is_system TINYINT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_roles_role_code ON roles(role_code);

-- Permissions table
CREATE TABLE IF NOT EXISTS permissions (
    id BIGSERIAL PRIMARY KEY,
    permission_name VARCHAR(50) NOT NULL,
    permission_code VARCHAR(100) UNIQUE NOT NULL,
    module VARCHAR(50),
    resource VARCHAR(50) NOT NULL,
    action VARCHAR(20) NOT NULL,
    description VARCHAR(200),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_permissions_permission_code ON permissions(permission_code);
CREATE INDEX idx_permissions_module ON permissions(module);

-- Role permissions table
CREATE TABLE IF NOT EXISTS role_permissions (
    id BIGSERIAL PRIMARY KEY,
    role_id BIGINT NOT NULL,
    permission_id BIGINT NOT NULL,
    org_id BIGINT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (role_id, permission_id, org_id)
);

CREATE INDEX idx_role_permissions_role_id ON role_permissions(role_id);

-- User roles table
CREATE TABLE IF NOT EXISTS user_roles (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    role_id BIGINT NOT NULL,
    org_id BIGINT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (user_id, role_id, org_id)
);

CREATE INDEX idx_user_roles_user_id ON user_roles(user_id);

-- ==================== game_core_db ====================

-- Games table
CREATE TABLE IF NOT EXISTS games (
    id BIGSERIAL PRIMARY KEY,
    org_id BIGINT NOT NULL,
    game_code VARCHAR(50) UNIQUE NOT NULL,
    game_name VARCHAR(100) NOT NULL,
    game_type VARCHAR(20) NOT NULL,
    game_icon VARCHAR(255),
    game_cover VARCHAR(255),
    game_config JSONB,
    status TINYINT DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_games_org_id ON games(org_id);
CREATE INDEX idx_games_game_code ON games(game_code);
CREATE INDEX idx_games_status ON games(status);

-- Game versions table
CREATE TABLE IF NOT EXISTS game_versions (
    id BIGSERIAL PRIMARY KEY,
    game_id BIGINT NOT NULL,
    version VARCHAR(20) NOT NULL,
    script_type VARCHAR(20) NOT NULL,
    script_path VARCHAR(255),
    script_hash VARCHAR(64),
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by BIGINT
);

CREATE INDEX idx_game_versions_game_id ON game_versions(game_id);

-- Game rooms table
CREATE TABLE IF NOT EXISTS game_rooms (
    id BIGSERIAL PRIMARY KEY,
    room_id VARCHAR(50) UNIQUE NOT NULL,
    game_id BIGINT NOT NULL,
    room_name VARCHAR(100),
    max_players INT DEFAULT 4,
    player_count INT DEFAULT 0,
    status VARCHAR(20) DEFAULT 'waiting',
    config JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    closed_at TIMESTAMP
);

CREATE INDEX idx_game_rooms_room_id ON game_rooms(room_id);
CREATE INDEX idx_game_rooms_game_id ON game_rooms(game_id);
CREATE INDEX idx_game_rooms_status ON game_rooms(status);

-- Game sessions table
CREATE TABLE IF NOT EXISTS game_sessions (
    id BIGSERIAL PRIMARY KEY,
    room_id VARCHAR(50) NOT NULL,
    game_id BIGINT NOT NULL,
    start_time TIMESTAMP NOT NULL,
    end_time TIMESTAMP,
    duration INT DEFAULT 0,
    status VARCHAR(20) NOT NULL,
    final_state JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_game_sessions_room_id ON game_sessions(room_id);
CREATE INDEX idx_game_sessions_game_id ON game_sessions(game_id);

-- Players table
CREATE TABLE IF NOT EXISTS players (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    game_id BIGINT NOT NULL,
    nickname VARCHAR(50),
    avatar VARCHAR(255),
    level INT DEFAULT 1,
    exp BIGINT DEFAULT 0,
    score BIGINT DEFAULT 0,
    status TINYINT DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (user_id, game_id)
);

CREATE INDEX idx_players_user_id ON players(user_id);
CREATE INDEX idx_players_game_id ON players(game_id);

-- Player stats table
CREATE TABLE IF NOT EXISTS player_stats (
    id BIGSERIAL PRIMARY KEY,
    player_id BIGINT NOT NULL UNIQUE,
    games_played INT DEFAULT 0,
    games_won INT DEFAULT 0,
    total_score BIGINT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_player_stats_player_id ON player_stats(player_id);

-- Guilds table
CREATE TABLE IF NOT EXISTS guilds (
    id BIGSERIAL PRIMARY KEY,
    org_id BIGINT NOT NULL,
    guild_name VARCHAR(100) NOT NULL,
    leader_id BIGINT NOT NULL,
    level INT DEFAULT 1,
    exp BIGINT DEFAULT 0,
    member_count INT DEFAULT 1,
    max_members INT DEFAULT 50,
    status TINYINT DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_guilds_org_id ON guilds(org_id);
CREATE INDEX idx_guilds_leader_id ON guilds(leader_id);

-- Guild members table
CREATE TABLE IF NOT EXISTS guild_members (
    id BIGSERIAL PRIMARY KEY,
    guild_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    role VARCHAR(20) NOT NULL,
    joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_guild_members_guild_id ON guild_members(guild_id);
CREATE INDEX idx_guild_members_user_id ON guild_members(user_id);

-- Activities table
CREATE TABLE IF NOT EXISTS activities (
    id BIGSERIAL PRIMARY KEY,
    org_id BIGINT NOT NULL,
    name VARCHAR(100) NOT NULL,
    type VARCHAR(20) NOT NULL,
    description TEXT,
    start_time TIMESTAMP NOT NULL,
    end_time TIMESTAMP NOT NULL,
    status VARCHAR(20) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_activities_org_id ON activities(org_id);
CREATE INDEX idx_activities_status ON activities(status);

-- Activity rewards table
CREATE TABLE IF NOT EXISTS activity_rewards (
    id BIGSERIAL PRIMARY KEY,
    activity_id BIGINT NOT NULL,
    name VARCHAR(100) NOT NULL,
    reward_type VARCHAR(20) NOT NULL,
    amount BIGINT NOT NULL
);

CREATE INDEX idx_activity_rewards_activity_id ON activity_rewards(activity_id);

-- Items table
CREATE TABLE IF NOT EXISTS items (
    id BIGSERIAL PRIMARY KEY,
    org_id BIGINT NOT NULL,
    item_code VARCHAR(50) NOT NULL,
    item_name VARCHAR(100) NOT NULL,
    item_type VARCHAR(20) NOT NULL,
    price BIGINT NOT NULL,
    description TEXT,
    status TINYINT DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_items_org_id ON items(org_id);
CREATE INDEX idx_items_item_code ON items(item_code);

-- User items table
CREATE TABLE IF NOT EXISTS user_items (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    item_id BIGINT NOT NULL,
    quantity INT DEFAULT 1,
    expires_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (user_id, item_id)
);

CREATE INDEX idx_user_items_user_id ON user_items(user_id);

-- ==================== game_payment_db ====================

-- Orders table
CREATE TABLE IF NOT EXISTS orders (
    id BIGSERIAL PRIMARY KEY,
    order_no VARCHAR(50) UNIQUE NOT NULL,
    user_id BIGINT NOT NULL,
    product_type VARCHAR(20) NOT NULL,
    product_id VARCHAR(50) NOT NULL,
    amount BIGINT NOT NULL,
    currency VARCHAR(10) DEFAULT 'USD',
    status VARCHAR(20) DEFAULT 'pending',
    payment_method VARCHAR(20),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_orders_user_id ON orders(user_id);
CREATE INDEX idx_orders_order_no ON orders(order_no);
CREATE INDEX idx_orders_status ON orders(status);

-- Scores table
CREATE TABLE IF NOT EXISTS scores (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT UNIQUE NOT NULL,
    balance BIGINT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_scores_user_id ON scores(user_id);

-- Score logs table
CREATE TABLE IF NOT EXISTS score_logs (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    type VARCHAR(20) NOT NULL,
    amount BIGINT NOT NULL,
    balance BIGINT NOT NULL,
    order_id BIGINT,
    reason TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_score_logs_user_id ON score_logs(user_id);
CREATE INDEX idx_score_logs_type ON score_logs(type);

-- ==================== game_notification_db ====================

-- Notifications table
CREATE TABLE IF NOT EXISTS notifications (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    type VARCHAR(20) NOT NULL,
    title VARCHAR(100) NOT NULL,
    content TEXT,
    read BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_notifications_user_id ON notifications(user_id);
CREATE INDEX idx_notifications_read ON notifications(read);

-- Messages table
CREATE TABLE IF NOT EXISTS messages (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    type VARCHAR(20) NOT NULL,
    title VARCHAR(100) NOT NULL,
    content TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_messages_user_id ON messages(user_id);

-- ==================== game_file_db ====================

-- Files table
CREATE TABLE IF NOT EXISTS files (
    id BIGSERIAL PRIMARY KEY,
    file_name VARCHAR(255) NOT NULL,
    file_path VARCHAR(500) NOT NULL,
    file_size BIGINT NOT NULL,
    mime_type VARCHAR(100),
    hash VARCHAR(64),
    uploader_id BIGINT,
    status TINYINT DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_files_hash ON files(hash);
CREATE INDEX idx_files_uploader_id ON files(uploader_id);

-- ==================== Indexes for performance ====================

-- Add composite indexes for common queries
CREATE INDEX IF NOT EXISTS idx_players_user_game ON players(user_id, game_id);
CREATE INDEX IF NOT EXISTS idx_guild_members_guild_user ON guild_members(guild_id, user_id);
CREATE INDEX IF NOT EXISTS idx_game_rooms_game_status ON game_rooms(game_id, status);
CREATE INDEX IF NOT EXISTS idx_orders_user_status ON orders(user_id, status);

COMMENT ON DATABASE game_platform_db IS 'Main platform database with users, organizations, and permissions';
COMMENT ON DATABASE game_core_db IS 'Game core database with games, players, guilds, and activities';
COMMENT ON DATABASE game_payment_db IS 'Payment database with orders, scores, and transactions';
COMMENT ON DATABASE game_notification_db IS 'Notification database with messages and notifications';
COMMENT ON DATABASE game_file_db IS 'File storage database with file metadata';
