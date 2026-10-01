-- Mini Ad Server Database Initialization Schema
-- PostgreSQL 14+

-- 1. Campaigns Table
CREATE TABLE IF NOT EXISTS campaigns (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'paused')),
    daily_impression_limit INT NOT NULL CHECK (daily_impression_limit > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 2. Creatives Table
CREATE TABLE IF NOT EXISTS creatives (
    id SERIAL PRIMARY KEY,
    campaign_id INT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    width INT NOT NULL CHECK (width > 0),
    height INT NOT NULL CHECK (height > 0),
    image_url TEXT NOT NULL,
    click_url TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 3. Ad Events Table
CREATE TABLE IF NOT EXISTS ad_events (
    id BIGSERIAL PRIMARY KEY,
    campaign_id INT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    creative_id INT NOT NULL REFERENCES creatives(id) ON DELETE CASCADE,
    type VARCHAR(20) NOT NULL CHECK (type IN ('impression', 'click')),
    ip VARCHAR(45) NOT NULL,
    user_agent TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for Fast Ad Matching and Reporting Aggregations
CREATE INDEX IF NOT EXISTS idx_campaigns_status ON campaigns(status);
CREATE INDEX IF NOT EXISTS idx_creatives_campaign_id ON creatives(campaign_id);
CREATE INDEX IF NOT EXISTS idx_creatives_size_campaign ON creatives(width, height, campaign_id);
CREATE INDEX IF NOT EXISTS idx_ad_events_created_at_type ON ad_events(created_at, type);
CREATE INDEX IF NOT EXISTS idx_ad_events_campaign_created ON ad_events(campaign_id, created_at, type);
CREATE INDEX IF NOT EXISTS idx_ad_events_fcap ON ad_events(creative_id, ip, created_at);

