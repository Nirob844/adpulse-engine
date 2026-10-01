# Task Specification: Mini Ad Server

## Scenario
You are building a tiny version of an ad-serving system. A publisher page asks for an ad, the server picks a campaign, the browser renders it, and the system tracks impressions and clicks and reports on them.

---

## Tech Stack
- **Backend**: Express.js or Go (both are equally acceptable)
- **Database**: PostgreSQL
- **Frontend**: Plain HTML/CSS/JavaScript or React
- **Optional**: Redis

---

## Data Model (PostgreSQL)
Design tables for at least the following:

### 1. `campaigns`
- `id`
- `name`
- `status` (`active` | `paused`)
- `daily_impression_limit` (max impressions per day, integer; stored only, enforcement is a bonus)
- `created_at`

### 2. `creatives`
- `id`
- `campaign_id`
- `width`
- `height`
- `image_url`
- `click_url` (landing page)

### 3. `ad_events`
- `id`
- `campaign_id`
- `creative_id`
- `type` (`impression` | `click`)
- `ip`
- `user_agent`
- `created_at`

---

## API Endpoints

| Endpoint | Behavior |
| :--- | :--- |
| `GET /ad?size=300x250` | Returns JSON with a chosen creative: `creative_id`, `html`, `impression_url`, `click_url`. `html` is an `<img>` tag built from the creative's `image_url`. `click_url` is the tracking URL (`/track/click?c=<creative_id>`), not the landing page itself. Selection may be random from active campaigns. If nothing matches the requested size, return `204 No Content`. |
| `GET /track/impression?c=<creative_id>` | Records an impression event and returns a 1x1 transparent GIF (`200 OK`). Returns `404` for an unknown creative. |
| `GET /track/click?c=<creative_id>` | Records a click event and responds with a `302` redirect to the creative's `click_url`. |
| `GET /report?from=&to=&group_by=campaign or day` | Returns impressions, clicks and CTR (%) per group. `from` and `to` are `YYYY-MM-DD` dates (inclusive). `group_by` defaults to `campaign`. Return `400` for invalid dates, from after to, or an unknown group_by. CTR is `clicks / impressions * 100`, and `0` when impressions are `0`. |
| `POST /campaigns` | Creates a campaign. Required: `name` (non-empty string), `daily_impression_limit` (positive integer). Optional: `status` (default `active`). |
| `PATCH /campaigns/:id` | Updates a campaign, mainly to pause or resume it by changing status. |

---

## Demo Publisher Page
1. Build `publisher.html` with two ad slots (`300x250` and `728x90`).
2. Write a small JS snippet (an "ad tag") that:
   - fetches `/ad` for each slot's size.
   - renders the returned creative (image) into the slot, wrapped in a link to the tracking `click_url` so that clicking the ad goes through `/track/click`.
   - Counts impression when creative is loaded in the browser.
   - fires the impression at most once per ad load.

---

## Bonus (not required)
- Cache the `/ad` candidate list in Redis with a TTL.
- Add a frequency cap (for example, no more than 3 impressions per creative per IP per day).
- Enforce each campaign's `daily_impression_limit` (stop serving a campaign once it has reached its daily impression count).

---

## Submission Requirements
Submit a GitHub repository link (public, or private with access shared with `riaadmorshed@gmail.com`). If you can, also host the project on a free hosting service (for example, Render, Railway or Vercel) and include the live URL, so we can check it quickly.

The repository must contain a `README.md` that covers:
- Architecture overview and request flow
- How to run the project
- Any bonus features implemented
- Assumptions you made and what you would improve with more time

