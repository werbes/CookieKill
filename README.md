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
| Space | Jump on dry land, on foot or riding a camel |
| Left click | Throw the selected cookie |
| Q | Dismount your camel; eat the selected cookie when on foot |
| Right click | Eat the selected cookie |
| E | Mount a nearby camel you own, chop a tree, gather, dig, open a chest, or interact |
| L, near the forest club | Toggle reduced flashing for the club lights |
| I | Inventory: 20 protected inventory slots and 5 protected hotbar slots |
| C | Cookbook and campfire crafting |
| M | World map |
| 1 through 5 | Select one of the five hotbar slots |
| 6 | Open the build menu after buying a home island |
| R, while placing | Rotate the selected build by 45 degrees |
| E or R, in a camel race | Standings, prizes, and return from the race |
| E or Q, in camel training | Choose exercises, feed training treats, or return |
| R, in cookie fighting or practice | Activity details and return to the tree hut |
| E, near gym equipment | Start exercising; E or Q stops |
| W, on a treadmill | Run to train stamina |
| Space, on a bench or leg press | Perform one repetition |
| E, in build mode | Take control of the furnishing you are looking at |
| T / F, while editing | Destroy for materials / pack into inventory (owner only) |
| Left click, while placing | Place the selected furnishing |
| Page Up / Page Down, while placing | Raise / lower the selected build |
| Escape | Open or close the game menu |

Click an occupied inventory slot, then another slot to move its stack. Different items swap; matching items merge. Put cookies in the hotbar to throw or eat them. Homeowners receive a sixth hotbar button for the **Build** menu; the original five item slots remain protected storage. The Escape menu includes **Friends** for friend requests, island visits, and permissions, and **Customize** for avatar and call name settings. Closing a crafting, shopping, or inventory panel returns to play without opening the Escape menu.

Players start with **125 stamina**, enough for roughly **seven seconds of continuous sprinting**; stamina training adds ten per level. Running out of stamina returns you to walking speed while stamina recovers. Release **Shift** and recover at least **20%** stamina before sprinting again. Press **Space** for each jump; holding it does not repeat jumps. You can jump on the mainland, in caves, and on home islands, including while riding a camel. Swimming prevents jumping. Mounted movement, jumping, and stamina use that camel's individual traits.

Every **50 metres of actual grounded travel on foot** triggers a **0.1% chance of falling**. Each alcoholic drink adds **one percentage point**, up to 100%; soft drinks do not change it. The HUD shows your current chance. Falling leaves you lying down for **two seconds**; then press **E**, **Space**, or a movement key to stand. Swimming, camel rides, jumps, teleports, and pushing against walls do not count toward the distance. Respawning resets the alcohol increase.

## The world

The shared map spans **480 by 460 game units**, with a forest and desert, winding beaches, shallow water that transitions into the deep ocean, and planted city streets. North is negative Z; east is positive X. Small location notices identify the current region. Trees, cacti, rocks, walls, and buildings block movement; doors provide entrances to village buildings and city businesses. Home islands are separate multiplayer spaces reached by teleportation.

* **Wildwood - northwest.** Random forest spawn, berries, nuts, trees, sticks, stones, and chests containing starter dough. Dough cannot be dug from the ground. Trees vary in color, size, and shape, with several kinds of wildflowers growing among them. Chop repeatedly to fell a tree: small trees take three chops and yield three wood; the largest take six chops and yield six wood. Felled trees regrow after three minutes. Build a campfire to bake.
* **Sunbaked Sands - northeast.** Clay villages, flowering cacti, desert ingredients, and a sunken cave entrance near the middle of the desert. Every cactus can be chopped down in two to four hits for two to four cactus ingredients, with safe regrowth after three minutes. NPC bakers inside three village houses sell berries, nuts, cactus, salt, protein, and basic materials, and barter for forest finds and ocean treasures; village ovens provide baking heat. Abu Fanous sells and rents camels on the far eastern edge.
* **The Blue - southwest.** Collect shells, small warm-colored stones, salt, and washed-up trash along the curved shore. Peace pays **3 coins per kilogram of trash**, and buys other ocean finds. Sea lions, dolphins, sharks, turtles, fish, and whales swim in deeper water.
* **Crumb City - southeast.** Colorful flower beds, planted courtyards, and palms surround streets connecting twelve individually claimable bakery businesses, kitchen equipment, paint, and outfit shops. **Baker's Pantry**, near **(145, 22)**, sells sugar for **4 coins**, butter for **5**, flour for **3**, and eggs for **3**. Desert village houses sell recipe add-ins. The gym has moved to the silver-cookie practice center in the southern tree hut.

Mainland housing plots have been removed. Build homes on home islands and decorate the interiors of owned city bakeries. Temporary cooking campfires also remain available on the main island.

Day lasts **12 minutes** and night lasts **6 minutes**, shared by everyone on the server. A clock shows the current phase and time remaining, with dusk and dawn lighting transitions. A new server starts in daytime.

At night, **John Pork** chooses the richest connected player carrying baked cookies or cakes. He pursues that player at **90% of their current movement speed**, including sprinting, training, buffs, swimming, and camel riding. Within **one metre**, he eats **eight baked items per second** from their bag and hotbar. Dough, ingredients, pantry/display stock, stored goods, and coins are unaffected. Walls block eating and he navigates around obstacles. After eating all a target's baked goods, he leaves that player alone for the rest of the night and seeks another eligible player. At dawn he sleeps in his hut in the far northeast near **(222, -239)**. His shirt carries the familiar John Pork meme portrait. He cannot enter or pursue players inside the disco cave, and racers are out of his reach in the racing ring. Portals give him a short delay before he follows into the destination world.

Within **100 metres**, a phone-call melody rings and a small **john pork is near** notice appears. If he is chasing you and gets within **10 metres**, it changes to **RUN FROM JOHN PORK**. Other nearby players receive the proximity notice. The ringing is synthesized locally, starts after a user gesture enables browser audio, and stops when John leaves range or your world; **Ring on/off** mutes it. Warnings remain visible when muted.

### The Root Room underground club

Follow the trail at the back of the northwest forest to the mossy cave entrance near **(-202, -204)**. Forty-eight steps lead **12 metres underground** to The Root Room: a colorful disco floor, mirror ball, gently pulsing lights, a bar, and chairs around cocktail tables. Press **E** at the bar, fortune wheel, or a chair. Press **E** again to stand, or move away. **L** switches to steady lighting; the setting is saved on your device and reduced-motion preferences also enable it.

The bar sells ten drinks, including Smirnoff Ice, beer, cider, cocktails, soft drinks, water, and chocolate or strawberry shakes. Purchases go into your inventory. Select a drink and use **Drink one**, or put it in the hotbar and use **Q** or right click while on foot. Shakes and other nonalcoholic refreshments can restore health or stamina; each menu entry shows its effects and price.

At night, **Disco Duke**, the club's purple-suited pimp in a feathered hat, sells funny cakes near **(-191, -245)**. Press **E** to browse: **Funny cake costs 15 coins for 45 seconds**, and **Cosmic cake costs 25 coins for 75 seconds**. Choose **Eat one** in your inventory, or eat from the hotbar with **Q** or right click, to see slowly shifting colors and a swaying world. Eating another slice refreshes the duration. The effect follows you outside the club and wears off automatically. **L** remains available while the effect is active; reduced flashing or your device's reduced-motion preference keeps colors still and removes the sway. Duke disappears and stops selling at dawn; cakes already bought stay in your inventory.

The fortune wheel uses **in-game coins**. Enter any affordable positive whole-coin stake and spin. The **10x jackpot has a 3% chance**, **5x has 7%**, **3x, 2x, and 1x each have 10%**, and **0.5x and 0.1x each have 30%**. Wheel slices show these proportions. The stake is deducted first; the payout includes the stake and is rounded down to whole coins. The 1x outcome returns the stake. The server chooses and validates every result. A stake must also leave room for the largest possible win within the browser's exact-integer wallet limit.

### Dune Dash camel racing

At the far western end of the beach, enter the aboveground cliff cave through its east-facing opening near **(-216, 21)**. Inside, press **E** at the camel-shaped lollipop near **(-227, 20)** and choose **Eat camel lollipop & enter**. The next available race starts **two minutes after sunrise**, once per **18-minute** day/night cycle. Registration closes **20 seconds before the start**, when connected entrants teleport to the grid. Eating a lollipop after that cutoff enters the following race. One taste registers one race; it does not charge coins.

The arena is a **120 × 84 metre oval** with three laps and 16 ordered checkpoints per lap. Your mounted camel comes with you; otherwise one of your owned camels is chosen, or a loan camel is provided. Follow the arrows and golden next-checkpoint strip. Use **W A S D**, mouse look, **Shift** to sprint, and **Space** to jump. **E** or **R** opens standings and the return button. The grid stays still during the countdown. Reversing cannot earn extra laps, and the inner fence prevents shortcuts.

| Place | Ordinary coins | Camel coins |
| --- | --- | --- |
| 1st | 200 × every other gathered racer | 50 |
| 2nd | 100 × every other gathered racer | 30 |
| 3rd | 50 × every other gathered racer | 10 |
| 4th and below | 0 | 0 |

The number of gathered entrants fixes the prize pool. A solo finisher earns no ordinary coins. Each result pays once. Disconnecting or withdrawing before finishing is a DNF; unfinished races time out after five minutes. Players return to their saved location and camel after leaving or the automatic return. Race camels never overwrite your camel's parking place. Registration and races are temporary server state; earned coins and camel coins persist.

### Camels and the pet store

Each camel is born with individual **speed**, **jump strength**, **stamina capacity**, and **happiness**, saved with its identity. Older saved camels receive stable traits on migration. Happiness affects pace: a fully happy camel reaches its natural speed. These abilities apply during exploration and racing.

Visit **Paws & Palms** near **(198, 153)** for treats. Move a treat into your hotbar, select it, and press **E** near your camel to feed one instead of mounting. Apples, carrots, dates, and oats appear in your hand, and the camel chews after eating. **Camel care** in Inventory or Friends also has feeding controls. Treats prepare faster training; they never increase a stat immediately. Dates boost speed work, carrots jump work, oats stamina work, and apples happiness work. Each treat doubles progress for **120 training points of actual work**, with a saved budget of up to **240 points per stat**. Idle time does not use that budget. Treats are not consumed when the stat is already capped or a full dose would exceed the boost budget, and feeding is paused while racing.

The **pink camel lollipop** near **(-231, 14)** inside Camel Cove opens the training grounds at any time. Bring a permanently owned camel; the mounted camel is selected first. Four fenced courses offer speed laps, three jump hurdles, an endurance lap requiring **40 metres of sprinting**, and a calm care circuit requiring a **one-second stop at each gate** without sprinting. Follow all four gates in order and complete the full lap. Standing still, repeatedly jumping in place, and cutting across the infield earn no progress.

A completed exercise earns **25 training points** before treat boosts. Every **100 points** permanently adds **0.1 speed**, **0.1 jump strength**, **2 stamina**, or **2 happiness**, up to the listed cap. Each camel saves its own partial progress and unused treats. **E**, **R**, or **Q** opens the training menu. Leaving, disconnecting, or gathering for a scheduled race restores the original world and keeps earned progress; training never changes a parked camel's saved location.

The **Call camel** button beside each camel's stats in Friends makes it run to you and stop nearby. Both of you must be in the same world and cave layer. Camels follow clear routes around obstacles; a blocked route needs to be opened before calling again. Calling never teleports a camel between worlds.

| Treat | Price | Training boost | Stat cap |
| --- | --- | --- | --- |
| Swift dates | 30 coins | 2x speed progress for 120 work | 16 m/s |
| Spring carrot | 25 coins | 2x jump progress for 120 work | 10 m/s takeoff |
| Stamina oats | 25 coins | 2x stamina progress for 120 work | 200 |
| Happy apple | 10 coins | 2x happiness progress for 120 work | 100% |

### Betty's flowers and the records room

**Petal & Posy**, near **(168, 153)**, is Betty's flower shop. Pick individual daisies, tulips, and lavender in the marked garden between the shop and the records room, then sell stems for **3**, **5**, and **4 coins** respectively. Picked flowers regrow after 45 seconds. Betty is an elderly lady with a silver bun, a warm welcome, and a few small-talk topics: press **E** near her to chat.

Choose **A little sunshine** (mixed flowers, **20 coins**), **Sunny daisies** (**15**), **Rosy tulips** (**30**), or **Lavender wishes** (**25**). Each variety has its own visible wrapped bouquet when selected in your hotbar, also visible to other players. Select one in Inventory, then choose a player within **four metres** to give that exact variety to. Gifts transfer one bouquet to the recipient's bag; blocked paths, different floors/worlds, full bags, and offline recipients cannot receive a gift.

The **Records Room**, near **(130, 153)**, contains five boards: **camel coins**, **time played**, **coins earned excluding gambling**, **player kills**, and **cookie ring kills**. Ring kills also count toward total player kills. Press **E** at a board for rankings. Saved offline players remain listed, ties share a rank, and your own rank is shown even outside the top ten. Connected playtime and newly recorded non-gambling earnings persist from this update onward. Gambling payouts, purchase refunds, and existing wallet balances do not inflate lifetime earnings.

### The southern Cookie Tree

Find the slender trunk near **(210, 184)** at the southern edge of the world. Wooden stairs wind around its outside in two turns, climbing **12 metres** to the same spacious **14 by 10 metre tree hut**. Start on the southwest side of the trunk and follow the spiral up to the entrance bridge. Press **E** at the **golden cookie** to enter a shared, enclosed **48 by 40 metre cookie fighting ring**. Throw the supplied sugar cookies with left click, dodge, and use the cover. Every ring defeat adds one to the winner's total kills and cookie-ring kills. Defeated players respawn inside the ring with brief protection. Supplied cookies do not consume inventory or award hit coins or coin transfers. **R** opens the score and return panel; the return cookie also takes you back to your tree-hut position.

The **silver cookie** leads to an aim range and the relocated gym. Four targets include a moving target; hits flash and increase your practice score. Gym training requires physical activity: hold **W** on the treadmill for **20 seconds** to train stamina, press **Space** for **10 controlled bench-press repetitions** to train arms, or use the leg press for **10 repetitions** to train legs. Enter a pool lane near its south end and swim the full lane out and back to train swimming. Each completed set costs **2 coins** and adds one level, up to **10**; existing levels carry over. Rest between sets, and use **E** or **Q** to leave an apparatus. The old instant-training menu is removed.

Your own walking and nearby players' walking produce quiet footsteps, with softer sounds at a distance and stereo direction. Footsteps follow actual grounded movement and stop while standing still, airborne, swimming, riding, or using gym equipment. The existing ambience mute and volume controls also control footsteps.

### Cookbook and inventory

All recipes begin in the cookbook with visible ingredients and hidden names and effects. Successfully baking a recipe records its name, damage, healing, and abilities permanently. Starting cookies and traded cookies remain unidentified until that recipe has been baked. Use the cookbook near a campfire or village oven for starter baking with dough from chests. Home and bakery ovens use the timed mixer and oven process below, with the matching dough held in your hand. Desert recipes still need a heat source; cakes need an oven. Player-placed main-island campfires disappear after **25 minutes**; home-island fires burn until a builder puts them out or destroys them.

A new account receives ten basic sugar cookies in its hotbar. They damage opponents when thrown and restore health when eaten. Ingredient recipes can add damage, slow opponents, weaken their throws, or briefly improve movement and throwing range. Discovering the protein recipe reveals cookies that grant **ten seconds of superstrength**: a hit defeats another player while the effect lasts. The brief shield after spawning still protects new arrivals. Respawning does not grant another starting batch.

Each player has **20 inventory slots** and **5 hotbar slots**. Every slot holds one item stack, and **all items survive death**. Only coins can be lost when another player defeats you.

Click a stack to reveal **Drop 1, Drop 5, Drop 10, and Drop all**. Dropped items form bundles of ten, then a bundle of five when possible, with any remainder lying separately. Press **E** near a bundle or single item to pick it up. Ground items disappear after **30 minutes**; picking them up and dropping them again starts a fresh timer.

The coin reward compares the two players' balances immediately before the defeat, before adding the killing hit reward. A killer with fewer coins receives **10% of the victim's coins, rounded up**. A killer with equal or more coins receives **5%, rounded down**. Valid hits also award **5 coins for a headshot**, **3 for a hand or foot**, and **1 for another body hit**. Practice targets and protected players do not provide player-hit rewards.

### The cave and your home island

Look for a pile of overlapping rocks and a shaded cave mouth near **(120, -75)** in Sunbaked Sands. The crown rocks share the entrance's warm brown sandstone colors. **Ninety-six steps** descend **24 metres underground** to a rough vaulted cavern with stalactites, rocky columns, and warm amber lighting. Rocks cover the outcrop above ground and the chamber walls below. The **pink lollipop in the left branch** offers your first home island for **200 coins**. Confirm the purchase, then choose whether to teleport home. The **blue lollipop in the other branch** opens friend-island travel.

Your home island starts with a shared materials chest and a permanent **return platform at (0, 28)**. Step onto the platform and interact to teleport directly back to the cave fork. Visitors can use it too. Existing islands also receive the platform; nearby saved furnishings move to clear space without losing their materials. Keep the platform's three-metre landing area free when building. Hotbar slot **6** opens four building categories:

* **Building:** walls, floors, roofs, stairs, window walls, and doors.
* **Decor:** tables, chairs, beds, potted plants, lanterns, shared materials chests, and personal storage chests.
* **Garden:** trees and colorful flower beds.
* **Appliances:** mixers, ovens, nut and berry garden beds, adoption ponds, and campfires.

Position pieces and rotate them in **45-degree steps**. Decor and furniture can overlap or stack; appliances cannot share a footprint, even at different heights. Built objects are solid. Floors and stairs provide walkable surfaces, doors can open, chairs can be sat in, and beds can be lain on. Home mixers and ovens use the same ingredients and timers as bakery equipment; making fresh dough requires owning a bakery. Place a table within **six metres** of a home oven at the same height to receive its finished bakes. Appliances and linked tables must finish their jobs and have their output collected before they can be packed or destroyed. Plant one nut or berry in a garden bed and harvest four after two minutes. Build a pond, then adopt a wounded fish or turtle from the ocean to give it a safe home.

Press **E** at the **island expansion sign at (-5, 24)** to see the next upgrade. The owner can expand an island **five times**, costing **150, 300, 500, 750, and 1,000 coins**. Each upgrade adds five metres to the building and walking radii; terrain, placement limits, and the map grow together. The level and size persist.

Build a **camel pen**, then interact with it to assign up to **four permanently owned camels** nearby. The enclosure has wooden fences and an open gate. Assigned camels return to their saved pen when you log out and remain visible to island visitors. Remove their assignments before moving, packing, or destroying their pen.

Interact with a pond to grow its capacity from **one to four animals**, costing **75, 125, then 200 coins**. Each enlargement expands the water and its footprint, so leave room around it. Existing adopted animals and pond upgrades survive saving; an empty packed pond retains its capacity.

### Sounds around the world

Beach surf fades into forest birds, while jumping dolphins whistle nearby. The disco has upbeat music, city shops and bakeries play cozy music that carries outside, and the desert has flute and plucked banjo melodies. All music is original, synthesized locally, and starts after interacting with the game. Open **Escape → World sounds** to mute or adjust volume; preferences are saved in your browser. John Pork's phone keeps its separate control. Backgrounding or disconnecting silences the ambience.

Enter **build mode** from the home menu before placing or editing anything. Look at a furnishing and press **E** to take control of it, then **R** to rotate, **T** to destroy, **F** to pack it into inventory, or **left click** to place it again. Packing is reserved for the owner and keeps the furnishing's colors. Destroying a piece returns **half of each material cost, rounded down**, to your inventory, or to the shared chest when a guest destroys it. Occupied storage and growing or inhabited garden pieces must be emptied first. Leaving build mode cancels an unfinished move. The island map shows furnishing shapes without name labels, and the home menu does not list every placed object.

Personal storage chests hold item stacks for the island owner. Shared materials chests hold construction supplies. Chest colors can be changed in their interaction panels. Plant trees and flowers from the home building catalog to decorate your garden. Island construction, stored contents, and permissions are saved.

### Friends and shared building

Every player receives a permanent friend code containing a **four-letter word and four random digits**, such as `FERN0427`. It is linked to the permanent account username; neither can be changed. Open **Escape → Friends**, enter another player's code, and send a request. The recipient receives a notification and can accept or decline. The Friends page also displays pending requests after reconnecting.

Friend codes are private by default. Enable public sharing to show yours under your call name to other players. Your call name can still change once per hour.

Taste the blue cave lollipop to unlock friend-island travel. A friend must own an island and enable **visitors** before you can teleport there. Visiting does not grant building rights: the owner must grant **you specifically** building permission. Owners can close visits or revoke an individual builder's permission.

Everyone with access to an island may deposit building materials in its shared chest. **Only the owner may withdraw them.** Guests with building permission spend supplies directly from that chest when placing structures; they do not need to withdraw the supplies first.

### Bakeries and camels

Your first city bakery costs **250 coins**. Its plot becomes available immediately when you disconnect, while your bakery purchase, upgrades, equipment, colors, staff, display stock, pantry, and production remain saved. When you return, claim any available plot **for free** to restore your bakery there. Twelve plots are available, each occupied by one connected baker at a time. Ovens bake cookies and cakes, mixers prepare dough, fridges store chef ingredients, and displays hold baked goods for visiting NPC customers. The plot sign manages upgrades, paint, and your bakery name in the same panel. The name stays with your bakery when you claim a different plot. The first upgrade costs **50 coins**, followed by **75, 100, 125, 150**, and so on, adding 25 per level. Upgrades improve sale prices with diminishing gains at higher levels. Sale bonuses are calculated over the whole transaction before rounding down: at level one, five basic cookies sell for 11 coins, compared with 10 at level zero. Each customer order shows the actual payment for its goods.

Buy the four basics at Baker's Pantry, then use your mixer: **one egg, one flour, one sugar, and one butter make 40 dough in 40 seconds**. Collect the finished dough, put it in your selected hotbar slot using **Hold**, and load it back into the mixer with recipe add-ins. Add-in mixing takes **one second per dough used**. Collect and hold the prepared dough, then load the oven. Baking takes **five seconds per cookie or cake**; plain dough makes basic sugar cookies. Finished bakes go onto the oven's **linked cooling table**, where you collect them before stocking the display. Each output station holds up to 200 items. Upgrades do not change these recipe quantities or timers. Production pauses while you are offline and resumes after you claim a bakery again. Before owning a bakery, chests are the only world source of dough; traders no longer sell it.

Mixers and ovens now accept **queues**, including home appliances. Add another batch while one is working; its required ingredients or held dough are reserved immediately, and batches run in order. Each appliance supports one active batch and **32 waiting batches**, saved across reconnects. If its output storage fills, a completed batch waits safely until you collect goods. Appliances and linked tables cannot be packed or destroyed while they contain jobs, queued batches, or output.

Buy **additional ovens for 120 coins** and **mixers for 90 coins** from the bakery building catalog. Place them in the kitchen in build mode. Each appliance has its own queue and runs independently; all bakery ovens deliver to the shared cooling table. Your chef uses the available appliances to prepare more batches in parallel. Moving or reopening a bakery preserves every appliance's work.

Stock cookies or cakes in your **display** before selling them. Its wider shelves hold **300 baked goods**. Customers approach from the street roughly every **20 seconds**, order up to five stocked items, and wait up to a minute. **Click a waiting customer, then click Serve** in their compact order popup to exchange displayed goods for coins. Stand within **four metres** with a clear view, outside build mode. Clicking a customer opens their order without throwing a cookie. The display menu still offers service controls. Stocking the shelf alone does not pay coins, and customers never take goods from your personal inventory.

Hire a **chef for 300 coins**, a **server for 250 coins**, and a **cashier for 200 coins**, each paid once at your bakery sign. Fill the physical **1,000-item kitchen fridge** with the four basic ingredients and recipe add-ins, then choose a discovered recipe at your oven. The chef mixes and bakes using those ingredients and leaves the finished cookies on the cooling table. The server walks to collect batches of up to **20** baked goods and carries them on a tray to the display. The cashier serves customers from actual display stock. Full or inaccessible stations pause the relevant work; goods being carried stay saved if delivery is interrupted. Staff work while you are connected with a claimed plot, even while you explore. Hires, fridge contents, carried goods, display stock, recipes and unfinished production persist between plots and reconnects. Existing pantry supplies migrate into the fridge.

Bakeries measure **20 by 16 metres**, with a separate tiled kitchen at the back and a cafe room containing a display, two tables, and four usable chairs. Inside your own bakery, **B** toggles bakery build mode and **6** opens its furnishing catalog. Look at a furnishing and press **E** to select it, **R** to rotate, **T** to destroy, **F** to pack it into inventory, and **left click** to place it. **Escape** cancels an edit. Buy bakery tables, chairs, counters, bread racks, plants, pendant lights, rugs, and menu boards with coins; paint a selected furnishing from the Bakery panel using a matching paint tin. Packing preserves its color. Layouts stay saved when you reopen on another plot. Oven, mixer, and linked cooling table belong in the kitchen; displays and seating belong in the cafe. Keep doors and station access clear. Moving a station also moves its working interaction, staff, and customer route. Finish work and collect outputs before packing or destroying appliances or the linked table. An absent station pauses work that needs it while preserving pantry and display contents. Visitors can sit but cannot redecorate your bakery.

Every claimed bakery includes a basic oven, mixer, cooling table, fridge, and display. Buy a **mixer finish for 40 coins** and an **oven finish for 60** as part of the paintable equipment collection. A **display upgrade for 35** earns one extra coin per cookie or cake sold. Owning all three unlocks paint purchases at the kitchen and paint shops. A paint tin costs **12 coins** and recolors the bakery walls or a purchased piece of equipment from the bakery panel. Seven colors are available.

Visit **Abu Fanous**, near **(226, -126)** on the far eastern side of Sunbaked Sands, to rent a camel for **20 minutes for 50 coins** or buy one permanently for **350 coins**. There is no limit on how many camels you can own. Give him one of every cookie and cake recipe to unlock a permanent purchase discount to **175 coins**. Press **Q** to dismount and park a camel, then **E** near it to get back on. Each camel's location is saved individually. The camel you are riding comes with you when teleporting between your home island and the mainland or cave; your other camels stay parked. Camels speed up land travel; rentals use real elapsed time and expire even while you are away.

Name each camel at the **bottom of the Friends panel**. Its name follows it when riding, parking, traveling, and reconnecting. An existing single-camel save migrates to the new herd automatically; that camel initially waits beside Abu Fanous.

### Ocean life and identity

The ocean starts with **57 animals**, 1.5 times its previous population: nine turtles, nine dolphins, eighteen fish, nine sea lions, six sharks, and six whales. Small groups leave open water to explore. Whales travel in ones or twos, sharks alone, turtles in groups of one to three, dolphins and sea lions in twos or threes, and fish in schools of three to six. A small fraction need help; some turtles are trapped in plastic.

Each animal has its own swimming pace and heading. A member of a group stays within **one metre of another member**, without marching in a straight line. Dolphins jump individually and slow down during their jump. Larger animals have more health. Gym swimming training improves your ability to keep up.

An untreated animal disappears after **three minutes**; treatment stops that deadline. Animals also eventually die of old age. A replacement appears as an egg after a short delay. Eggs hatch only after **two uninterrupted minutes**: approaching within 1.5 metres, interacting, or hitting the egg resets its timer. Reputation belongs to the species and remains when an individual is replaced.

Each of the six ocean species has one reputation score for you, shown in the inventory. **Below zero**, sharks and sea lions attack while the other species swim away. **Above zero**, animals of that species approach and greet you, occasionally bringing a shell, pearl, or treasure. Zero is neutral. Helping an animal raises your reputation with its whole species; hurting or killing one lowers it. Old individual opinions migrate to their species' average score.

Every account has a permanent username and a call name that can change **once per hour**. Customize five skin tones, T-shirts, hoodies or tank tops, trousers or shorts, and seven clothing colors. There are **15 achievements and matching hats**. Alongside the original five goals, earn rewards for recipe mastery, more recycling and rescues, a camel herd, racing prizes, time played, non-gambling earnings, island expansion, bakery staffing, and bakery upgrades. The Achievements tab shows exact goals and which rewards you have earned. Earned rewards stay unlocked permanently. Visit your bakery to hang or remove achievement frames on its cafe wall. Each distinct earned frame adds **10% of the original customer price**: four frames give a **1.4x** achievement factor. Bakery level and equipment bonuses are added separately; the combined achievement bonus rounds down once per sale. Appearance and identity changes are validated and saved by the server.

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

All twenty inventory slots and five hotbar slots, recipe discoveries, coins, gym levels, bakery purchases and upgrades, equipment and paint, home islands and their buildings/chests/permissions, friends and requests, permanent friend codes and visibility, camels and discounts, avatar appearance, permanent usernames, call names and their cooldowns, earned hats and achievements, hanging bakery frames, kills/deaths, and animal reputation are saved every five seconds and during graceful shutdown. Each player has a separate `data/players/<player-id>.json` file by default, containing a version number and that player's profile. Only changed profiles are rewritten, using a synced temporary file before replacing the previous file. A malformed save fails startup without overwriting it. Back up the data directory. Abrupt process or machine failure can lose progress since the last successful save, normally up to five seconds; player files are replaced independently, so an interrupted save may update some players before others.

Existing combined `players.json` saves (versions one, two, and three) migrate automatically on the first successful save. After all player files have been written, the original is retained as `players.json.<unique-suffix>.bak` in the data directory. If migration is interrupted, individual files take precedence over matching legacy records and remaining players still load from the combined file. Items from the retired three safe slots move into the twenty-slot inventory. Anything beyond its capacity remains in **Saved item recovery** until there is room to claim it. Previously owned bakeries receive individual plots. Account progress remains linked to the same email address. The server reads save versions one, two, and three and writes version three in the per-player format. Earlier binaries cannot read this directory format; keep a pre-upgrade backup if you need to return to an earlier binary.

Old mainland tents and cabins migrate into home-island ownership. Their original building materials are returned through saved item recovery, so a full inventory cannot erase the investment. Mainland shelters and their safe zones are removed.

Email login codes expire after ten minutes and can be used once; five incorrect attempts invalidate a code. The service rate-limits requests and verification attempts using the actual connection address. Select **Keep me signed in on this device** on a device you own to stay signed in for **30 days** after verifying your email once, including across browser and server restarts. Signing out revokes that device's session. Without this option, the browser gets a session cookie and the server expires the session after seven days at most; server restarts require signing in again. All session cookies are HttpOnly and SameSite, and Secure in production. Remembered sessions are saved atomically to `data/sessions.json` (under the configured data directory), containing only token hashes, player IDs/names, and expiration times. Pending codes and ordinary sessions remain in memory. Stable account IDs restore saved progress for the same normalized email address. The public game state does not contain email addresses.

Resources, ocean animals and eggs, world time, player positions, transient buffs, and main-island campfires are recreated when the server restarts. Home-island buildings, including campfires and adopted pond animals, persist. Signing in after a server restart spawns a saved account in the forest. Reconnecting to the same running server preserves position, health, buffs, and action cooldowns; disconnecting cannot refill health or bypass bakery production timers. Disconnected players remain in the world for ten seconds to prevent immediately escaping incoming attacks; their bakery plot is released immediately. Restarting the server also clears plot occupancy without removing saved bakery purchases or upgrades.

## Development and verification

```sh
go test ./...
go vet ./...
go build ./...
node --test scripts/state-sync.test.cjs
node --test scripts/cave-scenery.test.cjs
node --test scripts/club.test.cjs
node --test scripts/funny-cake-panels.test.cjs scripts/funny-cake-effects.test.cjs
node --test scripts/bakery-panels.test.cjs scripts/bakery-scene.test.cjs scripts/bakery-room.test.cjs scripts/bakery-furnishings.test.cjs
node --test scripts/production-panels.test.cjs scripts/production-scenery.test.cjs scripts/roof-scenery.test.cjs
node --test scripts/night-scene.test.cjs
node --test scripts/camel-race-ui.test.cjs scripts/town.test.cjs
node --test scripts/camel-companions-ui.test.cjs scripts/ambient-audio.test.cjs
node --test scripts/camel-training-ui.test.cjs scripts/achievements.test.cjs scripts/bouquets.test.cjs
node --test scripts/home-upgrades.test.cjs
node --test scripts/motion.test.cjs scripts/scenery-batches.test.cjs
node scripts/island-panels.test.cjs
```

On a machine with the C compiler needed by Go's race detector:

```sh
go test -race ./...
```

The GitHub Actions workflow runs race-enabled tests, vet, the build, browser JavaScript syntax validation, and state reconstruction tests on Linux. Its Windows job tests the service lifecycle and builds an installable deployment ZIP with a checksum as a downloadable artifact. Backend tests cover configuration paths, graceful shutdown, game rules, login security, persistence, real authenticated WebSocket connections, delta reconstruction, queue drops, and compression negotiation. `/healthz` provides a basic process health check.

Run `go test ./internal/server -run TestDeltaSteadyStateBandwidth -v` to compare complete snapshots, deltas, and compressed deltas over 100 simulated updates. This measures payload sizes in a stock local world, excluding the initial snapshot and network overhead; actual hosting traffic depends on player activity and connection count.

Run `go test ./internal/game -run '^$' -bench 'BenchmarkWorld' -benchmem` to measure simulation and snapshot construction at 1, 16, and 64 players. The fixture includes saved accounts, friend relationships, home islands, and the complete procedural map. Compare steady-state `SnapshotUpdate` batches separately from first-connection snapshots, which include the static layout. Server regressions also exercise blocked disk writes, ordered final saves, reconnects, mailbox pressure, and tick timing.

For the optional browser smoke check, keep a development server running and install Playwright locally with Node.js/npm:

```sh
npm install --prefix .tools --no-save playwright
node scripts/browser-smoke.cjs
```

The script uses installed Google Chrome at `C:/Program Files/Google/Chrome/Application/chrome.exe` on Windows. Set `CK_CHROME` to another installed Chrome/Chromium executable if needed; for example, in PowerShell: `$env:CK_CHROME = 'C:\path\to\chrome.exe'`. On other systems, set `CK_CHROME` or install Playwright's Chromium with `node .tools/node_modules/playwright/cli.js install chromium`. Set `CK_TEST_URL` if the development server uses a different local address.

The smoke check creates a local test account and checks rendering, login, authoritative state updates, movement, throwing, inventory, and the map. Screenshots are written to the ignored `test-results/` directory. It defaults to software rendering; set `CK_SMOKE_HARDWARE=1` to use Chrome's default graphics backend. It does not test real email delivery, certificate issuance, or production deployment. Node.js and Playwright are needed only for this optional check.

`node scripts/auth-browser-smoke.cjs` uses the same `CK_TEST_URL` server to check both login steps at laptop sizes, the opt-in checkbox, browser-session versus 30-day cookies, remembered sign-in after reopening, and sign-out revocation.

`node scripts/camel-browser-smoke.cjs` starts its own isolated server with a saved herd and home island, checks naming in Friends, **E/Q** mounting controls, mounted travel between home and cave, and saved names and parking locations after reconnecting. It chooses a free local port automatically; `CK_CAMEL_TEST_PORT` can override it.

`node scripts/home-companions-browser-smoke.cjs` exercises held-treat feeding, camel recall, pen assignment and offline housing, pond upgrades, and the home island expansion sign in Chrome. It uses an isolated saved home and writes screenshots under `test-results/`.

`node scripts/ambient-audio-browser-smoke.cjs` renders every environmental sound and footsteps to stereo WAV previews with Chrome's Web Audio engine and checks dolphin timing, quiet step levels, stopping, and disconnect silence. `node scripts/ambient-game-smoke.cjs` checks the actual game menu's volume and mute controls, saved preferences after reload, gesture unlock, and disconnect cleanup.

`node scripts/inventory-browser-smoke.cjs` starts an isolated server and checks the protected twenty-slot inventory and five-slot hotbar, Drop 1/5/10/all, ground bundles, and picking items back up through normal controls.

`node scripts/island-building-browser-smoke.cjs` starts an isolated server and checks explicit build mode, aimed **E** selection, **R/T/F** and click placement, packed furnishings, chest colors, personal storage, gardens, and the uncluttered island map.

`node scripts/cave-browser-smoke.cjs` checks the rocky entrance, the full descent to both lollipop chambers and the return climb, and captures forest wildflowers, cactus blossoms, and city flowerbeds in Chrome.

`node scripts/club-browser-smoke.cjs` starts an isolated server on a free port, walks the forest trail and underground stairs, checks lighting controls, bar purchases, drinking, seating, and exact payouts including a whole-wallet stake, then returns above ground and captures the desert cave rocks. Screenshots and logs go under `test-results/` and the server stops afterward.

`node scripts/funny-cake-browser-smoke.cjs` checks Disco Duke appearing at night, cake purchases, inventory and hotbar eating, color and motion effects, reduced flashing, refresh and expiry, dawn closing an open menu, and disconnect cleanup. It builds an isolated test server with temporary Go overlays for a shorter day/night cycle, effect duration, and nearby spawn; production timing is unchanged. Screenshots and logs go under `test-results/`.

`node scripts/fall-browser-smoke.cjs` starts an isolated server, checks alcoholic and nonalcoholic drink effects, walks until a guaranteed fall, verifies the two-second recovery lock and low camera, then stands and reconnects with the same alcohol effect. It uses seeded test inventory and ordinary game actions; its screenshots and logs stay under `test-results/`.

`node scripts/render-benchmark.cjs` builds an isolated server on port 8094 and records draw calls, frame timings, and a fixed forest screenshot. Run again with `--unbatched` to compare the same scene without plant batching. It defaults to Chrome's software renderer; set `CK_RENDER_HARDWARE=1` to use Chrome's default graphics backend. Treat software frame timings separately from hardware FPS. `CK_RENDER_TEST_PORT` and `CK_CHROME` override the port and browser. Results are saved under `test-results/` and the test server stops afterward.

`node scripts/island-browser-smoke.cjs` builds and runs its own isolated development server with two seeded test accounts. It checks home travel, the sixth hotbar slot, placement and rotation, moving and destroying builds, chest transfers, friend requests and permissions, and both cave branches in Chrome. It saves screenshots under `test-results/` and stops its server afterward. Its default port is 8091; set `CK_ISLAND_TEST_PORT` to use another free port.

`node scripts/bakery-browser-smoke.cjs` checks bakery purchase, upgrades, paint and naming, separate station actions, stocked-display NPC orders, manual service, staff hiring, fridge-only chef production and server delivery, automatic cashier sales, release on disconnect, another player's purchase, and free restoration on a different plot. It runs an isolated server on port 8093 (`CK_BAKERY_TEST_PORT` overrides this), uses two test accounts, saves screenshots, and stops the server afterward.

`node scripts/bakery-logistics-browser-smoke.cjs` checks the physical fridge, chef ingredient use, server pickup and visible tray delivery, the larger display, and exact cashier sales.

`node scripts/bakery-browser-smoke.cjs --building-only` checks the larger two-room bakery, furniture controls, buying and painting decorations, packing and replacing working stations, customer service at a relocated display, and restoring a saved layout on another plot.

`node scripts/bakery-browser-smoke.cjs --production-only` walks to Baker's Pantry, buys the four ingredients, mixes and collects 40 dough over 40 seconds, holds dough to add ingredients, bakes a timed batch onto the linked cooling table, stocks and sells it, and reconnects with an unfinished oven job. `--staff-only` checks the same cooking stations with chef, server, and cashier automation.

`node scripts/bakery-browser-smoke.cjs --queues-only` buys and places extra appliances, runs queued and simultaneous batches, collects the shared cooling output, and directly clicks a customer to serve an order.

`node scripts/night-browser-smoke.cjs` checks nighttime lighting, John Pork's visible chase, unlocked phone audio, both distance warnings, eight baked items eaten per second, and leaving a depleted target. `--dawn-only` checks returning to his hut at dawn and restored daylight; `--hut-only` inspects the hut, sleeping pose, and portrait shirt. These scripts build isolated test servers with a Go overlay that changes only the initial world clock; no debug endpoint or production timing change is added.

`node scripts/camel-race-browser-smoke.cjs` checks the cliff cave, lollipop registration, 20-second gathering countdown, three laps through ordered checkpoints, exact rewards, owned camel traits, and returning mounted to the beach. It runs an isolated server with a temporary initial-clock overlay and drives normal movement inputs around the course.

`node scripts/camel-training-browser-smoke.cjs` checks the pink training portal, all four course views, no idle or feeding-only stat gains, two real boosted laps, saved improvement, the training map, and return to the original beach position.

`node scripts/achievements-visual-smoke.cjs` checks all 15 hat models and cafe achievement frames in Chrome.

`node scripts/town-browser-smoke.cjs` checks city doorways, flower picking and selling, Betty's small talk, bouquet purchases and gifting between two players, all five leaderboards, and buying and feeding camel treats.

`node scripts/cookie-ring-browser-smoke.cjs` climbs the exterior spiral stairs with ordinary movement, uses both portal cookies, checks a real two-player projectile kill and both kill boards, respawning, inventory preservation, footsteps, and return to the elevated hut. `node scripts/cookie-practice-browser-smoke.cjs` checks target hits, physical treadmill and weight workouts, swimming lanes, completed-level charges, and return through the real UI. Each starts an isolated local server and saves screenshots and logs under `test-results/`.

Add `--tour` to walk from the forest through the desert village and city to Peace's beach stand, checking entrances and interaction screens. `--tour-only` runs that walkthrough without repeating the main UI checks. Use an isolated development world for test accounts:

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

The server limits the world to 64 simultaneous connections, validates actions and movement, bounds input rates, and disconnects superseded account connections. This is a playable alpha with one shared world, procedural graphics, fixed discoverable recipes, twelve bakery plots occupied during a player session, player homes, and a compact economy. Swimming changes speed and height; free diving is not implemented. World geometry blocks movement and projectiles, with door openings for accessible buildings. The connection limit is a guardrail, not a measured capacity guarantee; public launch would benefit from load testing, playtesting/balance, monitoring, and a transactional database for multiple server instances.

One goroutine owns the world and receives typed commands through a bounded in-memory channel. Connection readers validate messages before submitting them; connection writers encode and send snapshots independently. A separate disk worker saves detached profiles, keeps at most one newer pending save, and completes the final save before shutdown. Disk writes and socket backpressure do not pause the simulation. Initial snapshots include the static map; periodic updates omit that copy and construct only entities visible in the player's world.

The browser interpolates confirmed movement samples with a 120 ms local-position buffer and a 150 ms buffer for other players and animals. Mouse look remains immediate; stalled connections hold the last confirmed position. Teleports and reconnects reset the buffer. Stationary plants and ground decorations share spatial rendering batches while retaining individual chopping and regrowth visibility. These changes reduce update jitter and rendering work; they do not guarantee a particular frame rate on every device.

Three.js is vendored under its MIT license in `web/vendor/THREE-LICENSE.txt`.
