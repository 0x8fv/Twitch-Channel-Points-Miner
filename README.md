![Twitch Channel Points Miner](assets/banner.png)
<p align="center">
<a href="https://github.com/0x8fv/Twitch-Channel-Points-Miner/releases"><img alt="Latest Version" src="https://img.shields.io/github/v/release/0x8fv/Twitch-Channel-Points-Miner?style=flat&color=white&logo=github&logoColor=white"/></a>
<a href="https://github.com/0x8fv/Twitch-Channel-Points-Miner/stargazers"><img alt="GitHub Repo stars" src="https://img.shields.io/github/stars/0x8fv/Twitch-Channel-Points-Miner?style=flat&color=limegreen&logo=github&logoColor=white"/></a>
<a href="https://github.com/MShawon/github-clone-count-badge"><img alt="GitHub Traffic" src="https://hits.sh/github.com/0x8fv/Twitch-Channel-Points-Miner.svg?style=flat&label=views"/></a>
<a href="https://github.com/0x8fv/Twitch-Channel-Points-Miner"><img alt="GitHub last commit" src="https://img.shields.io/github/last-commit/0x8fv/Twitch-Channel-Points-Miner?style=flat&color=lightyellow&logo=github&logoColor=white"/></a>
</p>

# Twitch Channel Points Miner (Go)

Go rewrite of [0x8fv/Twitch-Channel-Points-Miner-v2](https://github.com/0x8fv/Twitch-Channel-Points-Miner-v2) with a focus on speed and low overhead. This version keeps the same behavior (auto-claims bonuses, drops, and predictions) but trims dependencies, streams point updates over PubSub.

> Credits to the original maintained fork by [rdavydov](https://github.com/rdavydov/Twitch-Channel-Points-Miner-v2), whose work inspired this Go rewrite.

## Why this version
- Written in Go for faster start-up and lower CPU compared to the original runtime.
- Uses Twitch PubSub for near-instant point gain logging.
- Can target a manual list of streamers or mine everyone you follow.

## Quick start
1) Install Go 1.22+ (see `go.mod`).
2) Copy `config.json` (it will be created/extended on first run) and set `username` plus any options you want. Leave the `streamers` array empty to mine all followed channels.
3) Run `go run .` (or `go build -o twitch-miner` and execute `./twitch-miner`).
4) On first launch you will see a device code prompt. Open `https://www.twitch.tv/activate`, enter the code, and wait until the app confirms login. Cookies are saved to `cookies/<username>.json` for future runs.
5) Press Ctrl+C to stop; a session summary is printed on exit.

When running a prebuilt binary, the miner uses `./config.json` if it exists in the current directory; otherwise it uses the directory containing the executable (so `config.json`, `cookies/`, and `log/` live next to the binary by default). Override with `-config`, `-data-dir`, `TCPM_CONFIG`, or `TCPM_DATA_DIR`.

## Configuration (config.json)
- `username`: Twitch login used for mining and for the cookie filename.
- `password`: Optional; device login is used, so you can leave this as-is.
- `debug`: Enable debug logs (high-level request summaries, PubSub message metadata).
- `debug_deep`: Enable deep debug logs (full request/response bodies and PubSub payloads); only emitted when `privacy.anonymize_logs` is `false`.
- `smart_logging`, `emojis`, `show_seconds`, `show_username_in_console`, `show_claimed_bonus_msg`: Console output preferences.
- `save_logs`: Write console output to `log/<username>.log` in addition to stdout.
- `privacy.anonymize_logs`: Replace streamer names and point balances in all logs with anonymized values; when enabled, `save_logs` writes to `log/miner.log`.
- `discord`: Optional Discord webhook notifications. Provide `webhook_api` plus `events` (list of event names to send). Supported events: `STARTUP`, `SHUTDOWN`, `STREAMER_ONLINE`, `STREAMER_OFFLINE`, `GAIN_FOR_RAID`, `GAIN_FOR_CLAIM`, `GAIN_FOR_WATCH`, `GAIN_FOR_WATCH_STREAK`, `BET_WIN`, `BET_LOSE`, `BET_REFUND`, `BET_FILTERS`, `BET_GENERAL`, `BET_FAILED`, `BET_START`, `BONUS_CLAIM`, `MOMENT_CLAIM`, `JOIN_RAID`, `DROP_CLAIM`, `DROP_STATUS`, `CHAT_MENTION`.
- `disable_ssl_cert_verification`: Disables TLS certificate/hostname verification for PubSub WebSockets only; leave `false` unless you know you need it.
- `timezone`: Optional IANA timezone name (e.g. `Europe/Berlin`) to override auto-detection for environments like Android/Termux; leave `null`/empty to auto-detect.
- `claim_drops_startup`, `claim_drops`, `follow_raid`: Auto-claim drops at boot, continue claiming while running, and auto-follow raid targets.
- `show_drops_progress`: When `true`, log compact `DROP_PROGRESS` status lines during the existing 30-minute drop inventory checks. Default `false` keeps drop logging limited to successful claims.
- `claim_moments`: Global default for listening to and claiming Twitch Moments via PubSub (`community-moments-channel-v1`). Per-streamer `streamer_overrides.<name>.claim_moments` still takes precedence.
- `betting(make_predictions)`: Enable Twitch prediction betting.
- `watch_streak_warm_start_cache`: When `true` (default), reuse recent watch-streak state for the same still-live stream across restarts to avoid re-probing every online channel at startup. The miner stores this account-scoped cache at `log/watch_streak_cache.<username>.json`.
- `disable_at_in_nickname`: When `true`, `CHAT_MENTION` also triggers on plain username mentions in chat (without requiring `@username`), matching the Python miner behavior.
- `streamers`: List of channel logins to mine; if empty, followers are mined in descending follow order.
- `streamers_exclude`: List of channel logins to skip (useful with an empty `streamers` list to mine all followed channels except specific ones); case-insensitive.
- `streamer_overrides`: Per-streamer overrides keyed by login; inherit from the global flags above. Example:
  ```json
  "streamer1": {
    "make_predictions": true,
    "follow_raid": false,
    "claim_drops": true,
    "claim_moments": false,
    "watch_streak": true,
    "community_goals": false,
    "bet": {
      "strategy": "SMART",
      "percentage": 5,
      "percentage_gap": 20,
      "max_points": 234,
      "minimum_points": 20000,
      "stealth_mode": true,
      "delay_mode": "FROM_END",
      "delay": 6,
      "filter_condition": {
        "by": "TOTAL_USERS",
        "where": "GTE",
        "value": 50
      }
    }
  },
  "streamer2": {
    "make_predictions": false,
    "follow_raid": true,
    "claim_drops": false,
    "claim_moments": true,
    "watch_streak": false,
    "community_goals": true,
    "bet": {
      "strategy": "PERCENTAGE",
      "percentage": 10,
      "percentage_gap": 15,
      "max_points": 5000,
      "minimum_points": 0,
      "stealth_mode": false,
      "delay_mode": "FROM_START",
      "delay": 3,
      "filter_condition": null
    }
  }
  ```
- `show_game`: When true, log the game a streamer is playing on join and on watch-point gains.
- `game_priority` / `game_exclude`: Ordered game names to prefer and names to skip entirely. Leave empty to keep the default behavior; matches are case-insensitive.
- `bet`: Advanced prediction tuning. Defaults are applied when values are `null`:
  - `strategy`: Default `SMART` (see `entities.Strategy` for options such as `MOST_VOTED`, `HIGH_ODDS`, `SMART_MONEY`, etc.).
  - `percentage`: Percent of points to bet (default 5).
  - `percentage_gap`: Minimum edge between outcomes before betting (default 20).
  - `max_points`: Cap per bet (default 50000).
  - `minimum_points`: Skip bets below this balance (default 0).
  - `stealth_mode`: If true, bets favor lower-visibility patterns (default false).
  - `delay_mode` / `delay`: When to place the bet (default `FROM_END`, 6 seconds).
  - `filter_condition`: Optional object to skip betting unless a live metric meets your rule. Example: `{ "by": "TOTAL_USERS", "where": "GTE", "value": 50 }` to skip bets until at least 50 users participate. `by` supports `PERCENTAGE_USERS`, `ODDS`, `ODDS_PERCENTAGE`, `TOP_POINTS`, `TOTAL_USERS`, `TOTAL_POINTS`; `where` supports `GT`, `LT`, `GTE`, `LTE`.

## Environment variable overrides

Every key in `config.json` can be overridden with an environment variable. The
variable name is `TCPM_` plus the uppercased key path joined by `_` (nested keys use underscores instead of dots). Values are parsed type-aware:

- booleans: `true`/`false`/`1`/`0`/`yes`/`no`
- integers and floats: `123`, `0.5`
- strings: as-is
- arrays of strings: comma-separated, e.g. `chan1,chan2`
- objects: a JSON object passed to the parent variable

Punctuation in a key also becomes `_`, so `betting(make_predictions)` is `TCPM_BETTING_MAKE_PREDICTIONS_` (trailing underscore; the shorter form is ignored). The `.env.example` uses the correct name.

### Usage examples
```
TCPM_STREAMERS=coolstreamer,beststreamer ./tcp-miner
TCPM_BET_STRATEGY=smart TCPM_DEBUG=true ./tcp-miner
TCPM_PRIVACY_ANONYMIZE_LOGS=true ./tcp-miner
```

Path overrides still take precedence: `TCPM_CONFIG` and `TCPM_DATA_DIR`.

This is what the Docker setup uses. See [Docker](#docker), `docker-compose.yml`, and `.env.example`.

## How it works
- Authenticates via Twitch device flow, persists cookies per user, and refreshes the client build id for GQL calls.
- Loads channel points context to grab balances and blue chests; watches two live streams at a time for minute-watched events to keep streaks active.
- The same watch loop reads HLS playlists and sends HEAD requests for every listed segment to generate Drops playback traffic, without downloading audio or video. Playlist URLs are cached and renewed after failures or a broadcast change; the existing telemetry cadence is unchanged.
- If you have an active subscription (multiplier) on a streamer, their name is highlighted in gold everywhere it appears in the logs so you can quickly spot priority channels.
- Listens to PubSub (`community-points-user-v1`) for instant point gain updates and logs deltas with reasons.
- Periodically claims inventory drops, can optionally log drop progress during those checks, and can auto-join raids and continue mining the destination channel.

## Docker

The two-stage `Dockerfile` builds a static binary and runs it on `scratch`. The final image has no shell or package manager, runs as root, and stores its data under `/app/data`. Compose builds `tcpm:latest` and persists cookies in the named volume `tcpm-twitch-cookies`.

### Quick start

```bash
cp .env.example .env      # set TCPM_USERNAME
docker compose up -d --build
docker compose logs -f tcpm
```

Enter the device code from the logs on first start. `docker compose down` removes the container; `docker compose down -v` also deletes the cookies volume.

### Configuration

Compose reads `TCPM_USERNAME` from `.env`, but only forwards the variables in its `environment:` block. To use more `TCPM_*` variables, uncomment `env_file: .env`; `environment:` overwrites variables from `env_file:`. Env overrides are runtime-only and are not saved to `config.json`.

Cookies are persisted by default. `config.json`, logs, and the watch-streak cache are lost when the container is recreated. To keep everything, mount `/app/data`:

```yaml
services:
  tcpm:
    volumes:
      - ./tcpm-data:/app/data
```

The default container runs as root, so it can write to the bind mount. If you switch to `user: "10001:10001"`, chown `tcpm-data` to that UID first or you will have file system permission issues.

### Build and update

```bash
docker build -t tcpm:latest .          # or: docker compose build
docker run -d --name tcpm \
  -e TCPM_USERNAME=your-twitch-username \
  -e TCPM_AUTO_UPDATE=false \
  -v tcpm-cookies:/app/data/cookies \
  tcpm:latest
```

To update, pull the git repo and rebuild: `git pull && docker compose build --pull && docker compose up -d`. The in-container updater is disabled because changes to the container disappear when it is recreated.

The image has no shell, so `docker exec` will not work; use `docker logs` and `docker cp` instead. 

## Notes
- Requires Go 1.22+; dependencies are in `go.mod`.
- Shortcut commands: `go fmt ./...` to format, `go test ./...` to run unit tests.
- Accepted minute-watched telemetry does not prove Drops credit. Check Twitch's own Drops inventory for authoritative progress.
- macOS: if Finder opens the downloaded binary in a text editor, run it from Terminal and make it executable first (e.g. `chmod +x TwitchChannelPointsMiner-darwin-arm64`).

## Disclaimer
- This project comes with no guarantee or warranty. You are responsible for whatever happens from using this project. It is possible to get soft or hard banned by using this project if you are not careful. This is a personal project and is in no way affiliated with Twitch.
