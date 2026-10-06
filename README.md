# CookieKill

A first-person multiplayer browser game with an authoritative Go server. Start in the forest with ten mystery cookies, chop trees, discover recipes, find a secret home island, build with friends, run a bakery, and throw your baking at other players.

The server owns movement and collisions, projectile hits, health, cooldowns, inventory slots, recipe discoveries, property ownership, trading, training, animal memory, and death rewards. The browser renders a procedural low-poly world with locally bundled Three.js. No Node.js build, CDN, image service, or external game account is needed to play.

## Run locally

Install **Go 1.26 or newer**, then run from this directory:

```sh
go run . -dev
```

Open **http://127.0.0.1:8080**. Enter an email address; local development displays a one-time code in the login form without sending email. Submit the code, then click **Let's play**. Use two different browser profiles, or one regular window and one private window, with different email addresses to play together on this computer. Two private windows in the same browser can share a session. One account can have only one active game connection.

On Windows, if Go is installed but absent from PATH:

```powershell
& 'C:\Program Files\Go\bin\go.exe' run . -dev
```

`scripts/play.ps1` also locates this installation. Development mode binds only to a literal loopback address and accepts local hostnames. Public play uses production mode with real email delivery and HTTPS. Both modes save progress to disk; use `-data` to choose a separate directory for local experiments. The explicit `-dev` flag starts local mode without requiring a production configuration file.

## Controls

| Input | Action |
| --- | --- |
| Mouse | Look around |
| W A S D | Move / swim |
| Shift | Sprint on land using stamina |
| Space | Jump while on foot on dry land |
| Left click | Throw the selected cookie |
| Q / right click | Eat the selected cookie |
| E | Chop a tree, gather, dig, open a chest, or interact with a nearby character or furnishing |
| I | Inventory: 3 permanent slots, 15 inventory slots, and 5 protected hotbar slots |
| C | Cookbook and campfire crafting |
| M | World map |
| 1 through 5 | Select one of the five hotbar slots |
| 6 | Open the build menu after buying a home island |
| R, while placing | Rotate the selected build by 45 degrees |
| Page Up / Page Down, while placing | Raise / lower the selected build |
| V | Mount or dismount your rented or owned camel |
| Escape | Open or close the game menu |

Click an occupied inventory slot, then another slot to move its stack. Different items swap; matching items merge. Put cookies in the hotbar to throw or eat them. Homeowners receive a sixth hotbar button for the **Build** menu; the original five item slots remain protected storage. The Escape menu includes **Friends** for friend requests, island visits, and permissions, and **Customize** for avatar and call name settings. Closing a crafting, shopping, or inventory panel returns to play without opening the Escape menu.

Running out of stamina returns you to walking speed while stamina recovers. Release **Shift** and recover at least **20%** stamina before sprinting again. Press **Space** for each jump; holding it does not repeat jumps. You can jump on the mainland, in the cave, and on home islands, but not while swimming or riding a camel.

## The world

The shared map spans **480 by 460 game units**, with a forest and desert, winding beaches, shallow water that transitions into the deep ocean, and planted city streets. North is negative Z; east is positive X. Small location notices identify the current region. Trees, cacti, rocks, walls, and buildings block movement; doors provide entrances to village buildings and city businesses. Home islands are separate multiplayer spaces reached by teleportation.

* **Wildwood - northwest.** Random forest spawn, berries, nuts, trees, sticks, stones, chests, and pale dig spots containing dough. Trees vary in color, size, and shape. Chop repeatedly to fell a tree: small trees take three chops and yield three wood; the largest take six chops and yield six wood. Felled trees regrow after three minutes. Build a campfire to bake.
* **Sunbaked Sands - northeast.** Clay villages, varied cacti, desert ingredients, and a rock-covered cave near the middle of the desert. NPC bakers inside three village buildings barter for berries, nuts, shells, or ocean treasures; village ovens provide baking heat. Abu Fanous sells and rents camels on the far eastern edge.
* **The Blue - southwest.** Collect shells, small warm-colored stones, salt, and washed-up trash along the curved shore. Peace pays **3 coins per kilogram of trash**, and buys other ocean finds. Sea lions, dolphins, sharks, turtles, fish, and whales swim in deeper water.
* **Crumb City - southeast.** Planted courtyards and palms surround streets connecting twelve individually claimable bakery businesses, a gym, kitchen equipment, provisions, paint, and outfit shops. Shops, bakeries, and the gym have distinct exteriors.

Mainland housing plots have been removed. Construction belongs on home islands; temporary cooking campfires and existing city bakery businesses remain available on the main island.

### Cookbook and inventory

All recipes begin in the cookbook with visible ingredients and hidden names and effects. Successfully baking a recipe records its name, damage, healing, and abilities permanently. Starting cookies and traded cookies remain unidentified until that recipe has been baked. Use the cookbook near a campfire, village oven, home-island oven, or your own bakery oven. Desert recipes still need a heat source; cakes need an oven. Player-placed main-island campfires disappear after **25 minutes**; home-island fires burn until a builder puts them out or destroys them.

A new account receives ten basic sugar cookies in its hotbar. They damage opponents when thrown and restore health when eaten. Ingredient recipes can add damage, slow opponents, weaken their throws, or briefly improve movement and throwing range. Discovering the protein recipe reveals cookies that grant **ten seconds of superstrength**: a hit defeats another player while the effect lasts. The brief shield after spawning still protects new arrivals. Respawning does not grant another starting batch.

Each player has **3 permanent slots**, **15 inventory slots**, and **5 hotbar slots**. Every slot holds one item stack. The permanent slots and hotbar survive death unchanged. When another player defeats you, **five random occupied inventory stacks** are chosen for transfer to the killer, or all occupied stacks if there are fewer than five. Other inventory stacks stay with you. A stack that will not fit in the killer's inventory remains with the defeated player.

The coin reward compares the two players' balances immediately before the defeat, before adding the killing hit reward. A killer with fewer coins receives **10% of the victim's coins, rounded up**. A killer with equal or more coins receives **5%, rounded down**. Valid hits also award **5 coins for a headshot**, **3 for a hand or foot**, and **1 for another body hit**. Practice targets and protected players do not provide player-hit rewards.

### The cave and your home island

Look for a pile of rocks and small plants near the middle of Sunbaked Sands, around **(120, -98)**. The entrance slopes down from sand onto clay, reaching a fork **ten metres underground**, beneath a vaulted ceiling. Lanterns and crystals decorate the two chambers, with grasses and flowers around the entrance. The **pink lollipop in the left branch** offers your first home island for **200 coins**. Confirm the purchase, then choose whether to teleport home. The **blue lollipop in the other branch** opens friend-island travel.

Your home island starts with a shared materials chest and a permanent **return platform at (0, 28)**. Step onto the platform and interact to teleport directly back to the cave fork. Visitors can use it too. Existing islands also receive the platform; nearby saved furnishings move to clear space without losing their materials. Keep the platform's three-metre landing area free when building. Hotbar slot **6** opens three building categories:

* **Building:** walls, floors, roofs, stairs, window walls, and doors.
* **Decor:** tables, chairs, beds, potted plants, lanterns, and materials chests.
* **Appliances:** mixers, ovens, nut and berry garden beds, adoption ponds, and campfires.

Position pieces and rotate them in **45-degree steps**. Decor and furniture can overlap or stack; appliances cannot share a footprint, even at different heights. Built objects are solid. Floors and stairs provide walkable surfaces, doors can open, chairs can be sat in, and beds can be lain on. Use mixers to make dough and ovens to bake. Plant one nut or berry in a garden bed and harvest four after two minutes. Build a pond, then adopt a wounded fish or turtle from the ocean to give it a safe home.

Permitted builders can move pieces at any time. Destroying a piece returns **half of each material cost, rounded down**. Island construction and permissions are saved. Use the home controls to return to the main island or teleport home again.

### Friends and shared building

Every player receives a permanent friend code containing a **four-letter word and four random digits**, such as `FERN0427`. It is linked to the permanent account username; neither can be changed. Open **Escape → Friends**, enter another player's code, and send a request. The recipient receives a notification and can accept or decline. The Friends page also displays pending requests after reconnecting.

Friend codes are private by default. Enable public sharing to show yours under your call name to other players. Your call name can still change once per hour.

Taste the blue cave lollipop to unlock friend-island travel. A friend must own an island and enable **visitors** before you can teleport there. Visiting does not grant building rights: the owner must grant **you specifically** building permission. Owners can close visits or revoke an individual builder's permission.

Owners and permitted builders can withdraw building materials from the island's chest. Visitors may donate materials by depositing them in that chest. Builders can withdraw those contributions and spend them on the owner's house, decor, or appliances.

### Bakeries and camels

A city bakery plot costs **250 coins** and can belong to only one player. Twelve plots are available in the shared world. The first upgrade costs **50 coins**, followed by **75, 100, 125, 150**, and so on, adding 25 per level. The first upgrade raises dough production from two to three per batch and reduces production cooldowns by 10%. Later upgrades gradually improve dough yield, cookie batches, and sale prices, with diminishing gains at higher levels. Sale bonuses are calculated over the whole transaction before rounding down: at level one, five basic cookies sell for 11 coins, compared with 10 at level zero. The bakery panel shows exact totals for selling one or five.

Buy a **mixer for 40 coins** to make dough 15% sooner, an **oven for 60** to bake 20% sooner, and a **display for 35** to earn one extra coin per cookie or cake sold. Owning all three unlocks paint purchases at the kitchen and paint shops. A paint tin costs **12 coins** and recolors the bakery walls or a purchased piece of equipment from the bakery panel. Seven colors are available.

Visit **Abu Fanous**, near **(226, -126)** on the far eastern side of Sunbaked Sands, to rent a camel for **20 minutes for 50 coins** or buy one permanently for **350 coins**. Give him one of every cookie and cake recipe to unlock a permanent purchase discount to **175 coins**. Camels speed up land travel; rentals use real elapsed time and expire even while you are away.

### Ocean life and identity

The ocean starts with **57 animals**, 1.5 times its previous population: nine turtles, nine dolphins, eighteen fish, nine sea lions, six sharks, and six whales. Small groups leave open water to explore. Whales travel in ones or twos, sharks alone, turtles in groups of one to three, dolphins and sea lions in twos or threes, and fish in schools of three to six. A small fraction need help; some turtles are trapped in plastic.

Each animal has its own swimming pace and heading. A member of a group stays within **one metre of another member**, without marching in a straight line. Dolphins jump individually and slow down during their jump. Larger animals have more health. Gym swimming training improves your ability to keep up.

An untreated animal disappears after **three minutes**; treatment stops that deadline. Animals also eventually die of old age. A replacement appears as an egg after a short delay. Eggs hatch only after **two uninterrupted minutes**: approaching within 1.5 metres, interacting, or hitting the egg resets its timer. Newborns do not inherit the previous animal's trust or grudges.

Witnesses remember animal kills: sharks and sea lions retaliate, while other species flee. Helping trapped or wounded animals builds trust and can earn pearls and other ocean valuables. Injury and reputation are separate: healthy animals can still learn to trust or avoid you.

Every account has a permanent username and a call name that can change **once per hour**. Customize five skin tones, T-shirts, hoodies or tank tops, trousers or shorts, and seven clothing colors. Hats are earned through achievements: discover a recipe, trade 10 kilograms of trash, rescue three animals, defeat five players, or own a home or bakery. Appearance and identity changes are validated and saved by the server.

## Windows service deployment

Build the complete Windows deployment package:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\build-windows.ps1
```

The result is **`dist/CookieKill-windows-amd64.zip`**, plus its SHA-256 checksum. Use `-Architecture arm64` for ARM64 Windows. The package includes the standalone executable with embedded browser files, `config.json`, install/uninstall scripts, deployment instructions, and third-party licenses. The server does not need Go or Node.js installed.

Extract to a permanent directory such as `C:\Services\CookieKill`. Edit **`config.json` next to `cookiekill.exe`**, filling in your public domain and SMTP settings. The top-level `email` is an optional Let's Encrypt contact address. Then run from an **administrator PowerShell** in that directory:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\Install-Service.ps1 -OpenFirewall
```

`-OpenFirewall` explicitly adds a Windows inbound firewall rule for TCP 80/443; omit it to manage firewall rules yourself. The installer registers **CookieKill** with automatic startup, failure recovery, graceful stop, and the **LocalService** account. The registered binary path has **no parameters**. It restricts access to the package and grants the service write access only to data/log directories. Installing into an existing deployment preserves configuration and saved progress. See [the deployment instructions](deployment/windows/README.txt) for updates, removal, permissions, backups, and troubleshooting.

```powershell
Get-Service CookieKill
Restart-Service CookieKill
Get-Content C:\Services\CookieKill\logs\cookiekill.log -Tail 50
```

The service writes to its configured log file. Startup failures also appear in **Event Viewer > Windows Logs > Application > CookieKill**. The log is append-only; arrange log retention. The service has not been installed on your machine by the build script.

## Public hosting and automatic HTTPS

Copy [config.example.json](config.example.json) to **`config.json` beside the built executable**, then configure it:

```json
{
  "dev": false,
  "addr": "127.0.0.1:8080",
  "domain": "play.example.com",
  "email": "admin@example.com",
  "dataDir": "data",
  "logFile": "logs/cookiekill.log",
  "smtp": {
    "host": "smtp.example.com",
    "port": 587,
    "user": "",
    "password": "",
    "from": "game@example.com"
  }
}
```

The top-level `email` is an optional certificate contact address, separate from SMTP login; it can be empty. For a mail server without authentication, leave both `smtp.user` and `smtp.password` empty as above. `smtp.from` is the required sender address, not a login.

On other platforms build with `go build -o cookiekill .` and run `./cookiekill` after creating the configuration file. Production settings come from JSON; the old `CK_DOMAIN`, `CK_SMTP_*`, and other server environment variables are no longer used. Unknown JSON fields fail startup. Relative `dataDir` and `logFile` paths resolve from the executable directory, so the service's working directory does not affect them. Restart after configuration changes. Keep SMTP credentials private; the installer restricts file access, and credentials are never sent to the browser.

Use a public hostname with DNS pointing at the host and incoming **TCP ports 80 and 443** open through the firewall and any router/NAT. The process needs permission to bind those ports and write to data/log directories. SMTP port **465** uses implicit TLS; all other ports, including **587**, require STARTTLS. Configure both SMTP username and password when your provider requires authentication. The sender must be allowed by your email provider. Actual email delivery and certificate issuance need verification with your domain and SMTP provider.

Production uses Go's [autocert manager](https://pkg.go.dev/golang.org/x/crypto/acme/autocert) to obtain and renew Let's Encrypt certificates for the configured hostname, persists its cache under the data directory, and redirects HTTP to HTTPS. Starting production accepts the Let's Encrypt subscriber agreement through `autocert.AcceptTOS`. Certificates can only be issued after the public domain and network are configured; localhost development uses HTTP.

| JSON setting | Default / purpose |
| --- | --- |
| `dev` | `false`; set `true` for local-only login and HTTP |
| `addr` | `127.0.0.1:8080`, development only |
| `domain` | Public hostname; required for production |
| `email` | Optional Let's Encrypt contact address; empty is allowed |
| `dataDir` | `data`; profiles and certificate cache |
| `logFile` | `logs/cookiekill.log`; append-only runtime log |
| `smtp.host`, `smtp.port` | SMTP hostname; port defaults to 587 |
| `smtp.user`, `smtp.password` | SMTP authentication; leave both empty for servers without login |
| `smtp.from` | Login email sender |

For local source development, `go run . -dev` still works without a configuration file; `-addr` and `-data` select the development listener and save location. A deployed binary can use `dev: true` in JSON for local testing, with an empty domain and a loopback address.

You can also build the supplied Dockerfile. Mount configuration read-only at **`/app/config.json`**, setting `dataDir` to `/data` and `logFile` to `/data/logs/cookiekill.log`. Mount persistent storage at `/data`, writable by container UID 10001, and publish ports 80/443. The container must be allowed to bind those ports. The Docker deployment has not been exercised here. Run a single server instance per data directory.

## Persistence and authentication

Inventory, permanent and hotbar slots, recipe discoveries, coins, gym levels, bakery plots and upgrades, equipment and paint, home islands and their buildings/chests/permissions, friends and requests, permanent friend codes and visibility, camels and discounts, avatar appearance, permanent usernames, call names and their cooldowns, earned hats, kills/deaths, and animal reputation are saved every five seconds and during graceful shutdown to `data/players.json` by default. Saves write a temporary file before replacing the previous file. A malformed save fails startup without overwriting it. Back up the data directory. Abrupt process or machine failure can lose progress since the last successful save, normally up to five seconds.

Existing version-one saves migrate automatically. Valid items fill the new slot arrays; anything beyond their capacity remains in **Saved item recovery** in the inventory panel until there is room to claim it. Previously owned bakeries receive individual plots. Account progress remains linked to the same email address. The new server reads save versions one and two and writes version two; the previous binary cannot load a version-two save. Keep a pre-upgrade backup if you need to return to the earlier binary.

Old mainland tents and cabins migrate into home-island ownership. Their original building materials are returned through saved item recovery, so a full inventory cannot erase the investment. Mainland shelters and their safe zones are removed.

Email login codes expire after ten minutes and can be used once; five incorrect attempts invalidate a code. The service rate-limits requests and verification attempts using the actual connection address. Sessions last seven days and use HttpOnly, SameSite cookies and Secure cookies in production. Sessions and pending codes live in memory; a restart requires signing in again. Stable account IDs restore saved progress for the same normalized email address. The public game state does not contain email addresses.

Resources, ocean animals and eggs, world time, player positions, transient buffs, and main-island campfires are recreated when the server restarts. Home-island buildings, including campfires and adopted pond animals, persist. Signing in after a server restart spawns a saved account in the forest. Reconnecting to the same running server preserves position, health, buffs, and action cooldowns; disconnecting cannot refill health or bypass bakery production timers. Disconnected players remain in the world for ten seconds to prevent immediately escaping incoming attacks.

## Development and verification

```sh
go test ./...
go vet ./...
go build ./...
node --test scripts/state-sync.test.cjs
node scripts/island-panels.test.cjs
```

On a machine with the C compiler needed by Go's race detector:

```sh
go test -race ./...
```

The GitHub Actions workflow runs race-enabled tests, vet, the build, browser JavaScript syntax validation, and state reconstruction tests on Linux. Its Windows job tests the service lifecycle and builds an installable deployment ZIP with a checksum as a downloadable artifact. Backend tests cover configuration paths, graceful shutdown, game rules, login security, persistence, real authenticated WebSocket connections, delta reconstruction, queue drops, and compression negotiation. `/healthz` provides a basic process health check.

Run `go test ./internal/server -run TestDeltaSteadyStateBandwidth -v` to compare complete snapshots, deltas, and compressed deltas over 100 simulated updates. This measures payload sizes in a stock local world, excluding the initial snapshot and network overhead; actual hosting traffic depends on player activity and connection count.

For the optional browser smoke check, keep a development server running and install Playwright locally with Node.js/npm:

```sh
npm install --prefix .tools --no-save playwright
node scripts/browser-smoke.cjs
```

The script uses installed Google Chrome at `C:/Program Files/Google/Chrome/Application/chrome.exe` on Windows. Set `CK_CHROME` to another installed Chrome/Chromium executable if needed; for example, in PowerShell: `$env:CK_CHROME = 'C:\path\to\chrome.exe'`. On other systems, set `CK_CHROME` or install Playwright's Chromium with `node .tools/node_modules/playwright/cli.js install chromium`. Set `CK_TEST_URL` if the development server uses a different local address.

The smoke check creates a local test account and checks rendering, login, authoritative state updates, movement, throwing, inventory, and the map. Screenshots are written to the ignored `test-results/` directory. It does not test real email delivery, certificate issuance, or production deployment. Node.js and Playwright are needed only for this optional check.

`node scripts/island-browser-smoke.cjs` builds and runs its own isolated development server with two seeded test accounts. It checks home travel, the sixth hotbar slot, placement and rotation, moving and destroying builds, chest transfers, friend requests and permissions, and both cave branches in Chrome. It saves screenshots under `test-results/` and stops its server afterward. Its default port is 8091; set `CK_ISLAND_TEST_PORT` to use another free port.

Add `--tour` to walk from the forest through the desert village and city gym to Peace's beach stand, checking entrances and interaction screens. `--tour-only` runs that walkthrough without repeating the main UI checks. Use an isolated development world for test accounts:

```sh
node scripts/browser-smoke.cjs --tour
```

```text
main.go             embedded browser client, HTTPS, logging, graceful shutdown
config.go           executable-relative JSON configuration and development flags
service_windows.go  native Windows service startup, status, stop and shutdown
deployment/windows/ installation/removal scripts and deployment instructions
scripts/build-windows.ps1  standalone Windows deployment ZIP and checksum
internal/auth/      passwordless email login and sessions
internal/game/      authoritative world simulation and game rules
internal/server/    HTTP, WebSocket connections, tick loop, save coordination
internal/store/     versioned profile saves
web/                browser UI, first-person renderer, local Three.js assets
```

The simulation runs at 20 ticks per second and publishes state ten times per second using [coder/websocket](https://github.com/coder/websocket). The browser requests `/ws?updates=delta-v1`: each connection receives a complete initial snapshot, then only changed player fields and changed entity fields, additions, and removals. Inventory maps and slots replace their previous values when changed; recipes and event lists are sent only when changed. WebSocket compression is negotiated when supported. The server computes each delta against the last successfully written state, so discarding queued updates for a slow connection cannot lose changes. Sequence checks reconnect the browser with a fresh snapshot if its baseline is lost. Older tabs using `/ws` continue receiving full snapshots until refreshed; rebuild and deploy the executable with its embedded browser assets to enable the new protocol.

The server limits the world to 64 simultaneous connections, validates actions and movement, bounds input rates, and disconnects superseded account connections. This is a playable alpha with one shared world, procedural graphics, fixed discoverable recipes, twelve exclusively owned bakery plots, player homes, and a compact economy. Swimming changes speed and height; free diving is not implemented. World geometry blocks movement and projectiles, with door openings for accessible buildings. The connection limit is a guardrail, not a measured capacity guarantee; public launch would benefit from load testing, playtesting/balance, monitoring, and a transactional database for multiple server instances.

Three.js is vendored under its MIT license in `web/vendor/THREE-LICENSE.txt`.
