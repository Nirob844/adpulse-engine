-- Mini Ad Server Initial Seed Data
-- 4 Campaigns (3 Active, 1 Paused)
-- Each campaign has both 300x250 and 728x90 creatives with placehold.co images

-- Clear existing data when re-seeding
TRUNCATE TABLE ad_events, creatives, campaigns RESTART IDENTITY CASCADE;

-- 1. Insert Campaigns
INSERT INTO campaigns (id, name, status, daily_impression_limit, created_at, updated_at) VALUES
(1, 'TechBrand NextGen Laptop',     'active', 5000, NOW() - INTERVAL '3 days', NOW() - INTERVAL '3 days'),
(2, 'CloudDev Serverless Platform',  'active', 2500, NOW() - INTERVAL '2 days', NOW() - INTERVAL '2 days'),
(3, 'CoffeeRoast Artisanal Beans',   'active', 1500, NOW() - INTERVAL '2 days', NOW() - INTERVAL '2 days'),
(4, 'Summer Flash Sale (Ended)',     'paused', 1000, NOW() - INTERVAL '5 days', NOW() - INTERVAL '5 days');

-- 2. Insert Creatives (Every campaign has both 300x250 and 728x90)
INSERT INTO creatives (id, campaign_id, width, height, image_url, click_url, created_at) VALUES
-- Campaign 1: TechBrand NextGen Laptop (Active)
(1, 1, 300, 250, 'https://placehold.co/300x250/2563eb/ffffff?text=TechBrand+NextGen%0A300x250', 'https://example.com/techbrand-laptop?ref=miniad', NOW() - INTERVAL '3 days'),
(2, 1, 728, 90,  'https://placehold.co/728x90/2563eb/ffffff?text=TechBrand+NextGen+Leaderboard+728x90', 'https://example.com/techbrand-laptop?ref=miniad', NOW() - INTERVAL '3 days'),

-- Campaign 2: CloudDev Serverless Platform (Active)
(3, 2, 300, 250, 'https://placehold.co/300x250/7c3aed/ffffff?text=CloudDev+Serverless%0A300x250', 'https://example.com/clouddev?ref=miniad', NOW() - INTERVAL '2 days'),
(4, 2, 728, 90,  'https://placehold.co/728x90/7c3aed/ffffff?text=CloudDev+Serverless+Leaderboard+728x90', 'https://example.com/clouddev?ref=miniad', NOW() - INTERVAL '2 days'),

-- Campaign 3: CoffeeRoast Artisanal Beans (Active)
(5, 3, 300, 250, 'https://placehold.co/300x250/b45309/ffffff?text=CoffeeRoast+Artisanal%0A300x250', 'https://example.com/coffeeroast?ref=miniad', NOW() - INTERVAL '2 days'),
(6, 3, 728, 90,  'https://placehold.co/728x90/b45309/ffffff?text=CoffeeRoast+Artisanal+Leaderboard+728x90', 'https://example.com/coffeeroast?ref=miniad', NOW() - INTERVAL '2 days'),

-- Campaign 4: Summer Flash Sale (Paused)
(7, 4, 300, 250, 'https://placehold.co/300x250/64748b/ffffff?text=Summer+Sale+(Paused)%0A300x250', 'https://example.com/summer-sale?ref=miniad', NOW() - INTERVAL '5 days'),
(8, 4, 728, 90,  'https://placehold.co/728x90/64748b/ffffff?text=Summer+Sale+Banner+(Paused)+728x90', 'https://example.com/summer-sale?ref=miniad', NOW() - INTERVAL '5 days');

-- Reset sequences to align with inserted IDs
SELECT setval('campaigns_id_seq', (SELECT MAX(id) FROM campaigns));
SELECT setval('creatives_id_seq', (SELECT MAX(id) FROM creatives));

-- 3. Insert Sample Historical Ad Events (for instant report validation)
-- Yesterday's events
INSERT INTO ad_events (campaign_id, creative_id, type, ip, user_agent, created_at) VALUES
(1, 1, 'impression', '192.168.1.10', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64)', NOW() - INTERVAL '1 day 4 hours'),
(1, 1, 'impression', '192.168.1.11', 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)', NOW() - INTERVAL '1 day 3 hours'),
(1, 1, 'click',      '192.168.1.10', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64)', NOW() - INTERVAL '1 day 3 hours'),
(1, 2, 'impression', '192.168.1.12', 'Mozilla/5.0 (X11; Linux x86_64)', NOW() - INTERVAL '1 day 2 hours'),
(2, 3, 'impression', '192.168.1.13', 'Mozilla/5.0 (iPhone; CPU iPhone OS 16_5)', NOW() - INTERVAL '1 day 5 hours'),
(2, 3, 'click',      '192.168.1.13', 'Mozilla/5.0 (iPhone; CPU iPhone OS 16_5)', NOW() - INTERVAL '1 day 4 hours'),
(3, 5, 'impression', '192.168.1.14', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64)', NOW() - INTERVAL '1 day 1 hour');

-- Today's events
INSERT INTO ad_events (campaign_id, creative_id, type, ip, user_agent, created_at) VALUES
(1, 1, 'impression', '127.0.0.1', 'Mozilla/5.0 (Chrome/120.0)', NOW() - INTERVAL '2 hours'),
(1, 1, 'impression', '127.0.0.2', 'Mozilla/5.0 (Chrome/120.0)', NOW() - INTERVAL '1 hour'),
(1, 1, 'click',      '127.0.0.1', 'Mozilla/5.0 (Chrome/120.0)', NOW() - INTERVAL '1 hour'),
(2, 3, 'impression', '127.0.0.3', 'Mozilla/5.0 (Firefox/120.0)', NOW() - INTERVAL '45 minutes'),
(2, 4, 'impression', '127.0.0.4', 'Mozilla/5.0 (Safari/17.0)', NOW() - INTERVAL '30 minutes'),
(3, 5, 'impression', '127.0.0.5', 'Mozilla/5.0 (Edge/120.0)', NOW() - INTERVAL '20 minutes'),
(3, 5, 'click',      '127.0.0.5', 'Mozilla/5.0 (Edge/120.0)', NOW() - INTERVAL '10 minutes');
