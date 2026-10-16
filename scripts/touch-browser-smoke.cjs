// Real Chrome/Go integration: mobile multi-touch controls and desktop regression.
// Uses temporary fixture accounts/data; never modifies an existing player profile.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const net = require('node:net');
const {spawn, spawnSync} = require('node:child_process');
const {chromium} = require(require.resolve('playwright', {paths: [path.resolve(__dirname, '../.tools')]}));

(async () => {
  const root = path.resolve(__dirname, '..');
  const out = path.join(root, 'test-results', 'touch-browser-' + Date.now());
  const data = path.join(out, 'data');
  fs.mkdirSync(data, {recursive: true});
  const emails = ['touch-baker@example.com', 'keyboard-baker@example.com', 'hover-baker@example.com', 'windows-touch-baker@example.com'];
  const slots = n => Array.from({length: n}, () => ({item: '', count: 0}));
  const profiles = Object.fromEntries(emails.map((email, index) => {
    const id = 'u_' + crypto.createHash('sha256').update(email).digest('hex').slice(0, 32);
    const hotbar = slots(5);
    hotbar[0] = {item: 'sugar', count: 20};
    hotbar[1] = {item: 'berry_cookie', count: 10};
    return [id, {id, name: index ? 'Keyboard Baker' : 'Touch Baker', callName: index ? 'Keyboard Baker' : 'Touch Baker',
      username: index ? 'keyboard_baker_' + index : 'touch_baker', coins: 1000, inventoryVersion: 3,
      hotbar, bagSlots: slots(20), selectedSlot: 0, levels: {}, discovered: {sugar: true, berry_cookie: true}}];
  }));
  fs.writeFileSync(path.join(data, 'players.json'), JSON.stringify({version: 3, profiles}));
  const executable = path.join(out, process.platform === 'win32' ? 'server.exe' : 'server');
  const go = process.env.CK_GO || (process.platform === 'win32' ? 'C:/Program Files/Go/bin/go.exe' : 'go');
  const build = spawnSync(go, ['build', '-o', executable, '.'], {cwd: root, windowsHide: true, encoding: 'utf8'});
  assert.equal(build.status, 0, build.stderr || build.stdout);
  const port = await new Promise(resolve => {
    const probe = net.createServer();
    probe.listen(0, '127.0.0.1', () => {const p = probe.address().port; probe.close(() => resolve(p));});
  });
  const base = 'http://127.0.0.1:' + port;
  let browser, page, log = '';
  const errors = [], requestsFailed = [], sent = [], report = [];
  const server = spawn(executable, ['-dev', '-addr', '127.0.0.1:' + port, '-data', data],
    {cwd: root, windowsHide: true, stdio: ['ignore', 'pipe', 'pipe']});
  server.stdout.on('data', x => log += x);
  server.stderr.on('data', x => log += x);
  async function until(check, label, timeout = 20000) {
    const started = Date.now();
    while (!await check()) {
      assert.ok(Date.now() - started < timeout, label + ' ' + errors.join('; '));
      await new Promise(resolve => setTimeout(resolve, 50));
    }
  }
  const read = () => page.evaluate(() => window.__touchGame());
  const stopped = s => !s.movement.x && !s.movement.z && !s.movement.sprint && !s.movement.jump;
  const actionButton = name => page.locator('[data-touch-action="' + name + '"]');
  async function resume() {
    if (await page.locator('#modal').isVisible()) await page.locator('#close-modal').tap();
    if (await page.locator('#play-overlay').isVisible()) await page.locator('#play-button').tap();
    if (await page.locator('#start-overlay').isVisible()) await page.locator('#start-button').tap();
    await until(async () => {const s = await read(); return s.locked && !s.modal && !s.menuOpen;}, 'touch play resumes');
    assert.equal((await read()).pointerLock, false, 'touch play must not require pointer lock');
    assert.equal(await page.locator('#touch-controls').isVisible(), true);
  }
  async function checkDesktopControls(label) {
    assert.equal(await page.locator('body').evaluate(el => el.classList.contains('touch-controls-enabled')), false, 'desktop is not touch mode');
    assert.equal(await page.locator('#touch-controls').isVisible(), false, 'desktop controls hidden');
    assert.equal(await page.locator('#touch-mode-settings').evaluate(el => el.hidden), true, 'desktop touch settings hidden');
    assert.equal(await page.locator('#touch-mode-toggle').isDisabled(), true, 'desktop touch toggle disabled');
    await page.locator('#start-button').click();
    await until(async () => (await read()).pointerLock && (await read()).locked, 'desktop pointer lock');
    await page.keyboard.down('w');
    assert.ok(Math.hypot((await read()).movement.x, (await read()).movement.z) > .9, 'desktop W movement');
    const desktopMovement = (await read()).movement;
    for (const pointerType of ['touch', 'pen']) for (const target of ['document', 'world']) {
      await page.evaluate(({pointerType, target}) => {
        const element = target === 'document' ? document : document.getElementById(target);
        element.dispatchEvent(new PointerEvent('pointerdown', {pointerType, pointerId: 51,
          clientX: 50, clientY: 50, bubbles: true, cancelable: true}));
        document.dispatchEvent(new PointerEvent('pointerup', {pointerType, pointerId: 51, bubbles: true}));
      }, {pointerType, target});
      const current = await read(), label = 'desktop ' + pointerType + ' on ' + target;
      assert.equal(current.touchControls, false, label + ' cannot enable mobile controls');
      assert.equal(current.pointerLock && current.locked, true, label + ' preserves pointer lock');
      assert.deepEqual(current.movement, desktopMovement, label + ' preserves held keyboard movement');
    }
    await page.locator('#touch-mode-toggle').dispatchEvent('click');
    const afterToggle = await read();
    assert.equal(afterToggle.touchControls, false, 'forced desktop toggle click cannot enable mobile controls');
    assert.equal(afterToggle.pointerLock && afterToggle.locked, true, 'forced desktop toggle preserves pointer lock');
    assert.deepEqual(afterToggle.movement, desktopMovement, 'forced desktop toggle preserves held keyboard movement');
    await page.keyboard.down('Space');
    assert.equal((await read()).movement.jump, true, 'desktop keyboard jump');
    await page.keyboard.up('Space');
    await page.keyboard.up('w');
    assert.equal(stopped(await read()), true, 'desktop keyboard release');
    await page.screenshot({path: path.join(out, label + '-play.png')});
    await page.setViewportSize({width: 390, height: 844});
    assert.equal(await page.locator('#touch-controls').isVisible(), false, 'narrow desktop does not enable touch');
  }
  async function setup(context, email) {
    const next = await context.newPage();
    next.on('pageerror', error => errors.push(error.message));
    next.on('response', response => {if (response.status() >= 400) requestsFailed.push(response.status() + ' ' + response.url());});
    next.on('websocket', socket => socket.on('framesent', ({payload}) => {
      try {sent.push({...JSON.parse(payload.toString()), at: Date.now(), email});} catch {}
    }));
    // Accessors and context fixtures are injected only into this test's response.
    await next.route('**/game.js', async route => {
      const response = await route.fetch(), source = await response.text();
      assert.ok(source.includes('function receiveSnapshot(snapshot){'), 'debug injection anchor');
      const debug = `window.__touchGame=()=>({movement:movement(),touchControls:state.touchControls,
        locked:state.locked,playing:state.playing,modal:state.modal,menuOpen:state.menuOpen,
        connected:state.connected,yaw:state.yaw,pitch:state.pitch,selectedSlot:state.selectedSlot,
        me:state.snapshot?.me,pointerLock:!!document.pointerLockElement});
        window.__touchDisconnect=()=>state.ws.close(4001,'test disconnect');
        window.__touchLabels=(patch={},building=null)=>{const saved=state.snapshot,savedBuilding=state.building;
          try{state.snapshot={...saved,...patch,me:{...saved.me,...patch.me}};state.building=building;updateTouchUI();
            return {eat:$('touch-eat').textContent,throw:$('touch-throw').textContent,
              activity:!$('touch-activity').hidden,build:!$('touch-build-controls').hidden};
          }finally{state.snapshot=saved;state.building=savedBuilding;updateTouchUI();}};
        function receiveSnapshot(snapshot){`;
      await route.fulfill({response, body: source.replace('function receiveSnapshot(snapshot){', debug)});
    });
    await next.goto(base, {waitUntil: 'networkidle'});
    await next.waitForFunction(() => window.__touchGame);
    await next.locator('#email').fill(email);
    await next.locator('#email-submit').click();
    await next.locator('#code-stage').waitFor({state: 'visible'});
    if (!await next.locator('#login-code').inputValue()) {
      await next.locator('#login-code').fill((await next.locator('#development-code').innerText()).match(/\b\d{6}\b/)[0]);
    }
    await next.locator('#code-submit').click();
    await next.locator('#start-button').waitFor({state: 'visible'});
    return next;
  }
  async function center(selector) {
    const b = await page.locator(selector).boundingBox();
    assert.ok(b, selector + ' is visible');
    return {x: b.x + b.width / 2, y: b.y + b.height / 2};
  }
  async function geometry(label) {
    const result = await page.evaluate(() => {
      const selectors = ['#touch-joystick', '#touch-sprint', '#touch-jump', '#touch-controls button', '#hotbar [data-slot]'];
      const controls = [...new Set(selectors.flatMap(selector => [...document.querySelectorAll(selector)]))]
        .filter(element => element.getClientRects().length && getComputedStyle(element).visibility !== 'hidden')
        .map(element => {
          const box = element.getBoundingClientRect();
          const top = document.elementFromPoint(box.x + box.width / 2, box.y + box.height / 2);
          return {id: element.id || element.dataset.touchAction || 'hotbar-' + element.dataset.slot,
            x: box.x, y: box.y, width: box.width, height: box.height, disabled: element.disabled,
            reachable: top === element || element.contains(top)};
        });
      return {viewport: {width: innerWidth, height: innerHeight}, scrollWidth: document.documentElement.scrollWidth, controls};
    });
    assert.ok(result.controls.length >= 10, label + ': expected visible touch controls');
    assert.ok(result.scrollWidth <= result.viewport.width + 1, label + ': no horizontal document overflow');
    for (const c of result.controls) {
      assert.ok(c.width >= 44 && c.height >= 44, label + ': 44px touch target ' + JSON.stringify(c));
      assert.ok(c.x >= -1 && c.y >= -1 && c.x + c.width <= result.viewport.width + 1 && c.y + c.height <= result.viewport.height + 1,
        label + ': control within viewport ' + JSON.stringify(c));
      assert.ok(c.disabled || c.reachable, label + ': unobstructed touch target ' + JSON.stringify(c));
    }
    report.push({label, ...result});
    await page.screenshot({path: path.join(out, label + '.png')});
  }
  async function panelGeometry(label) {
    const result = await page.evaluate(() => {
      const box = document.querySelector('.modal-shell').getBoundingClientRect();
      const content = document.getElementById('modal-content');
      return {x: box.x, y: box.y, width: box.width, height: box.height,
        viewport: {width: innerWidth, height: innerHeight},
        contentWidth: content.clientWidth, contentScrollWidth: content.scrollWidth,
        squeezedTabs: [...document.querySelectorAll('.modal-tab')].filter(tab => tab.scrollWidth > tab.clientWidth + 1).map(tab => tab.textContent)};
    });
    assert.ok(result.x >= -1 && result.x + result.width <= result.viewport.width + 1,
      label + ': full modal fits viewport ' + JSON.stringify(result));
    assert.ok(result.y >= -1 && result.y + result.height <= result.viewport.height + 1,
      label + ': modal fits viewport height ' + JSON.stringify(result));
    assert.ok(result.contentScrollWidth <= result.contentWidth + 1,
      label + ': content has no horizontal overflow ' + JSON.stringify(result));
    assert.deepEqual(result.squeezedTabs, [], label + ': panel tab labels do not overlap');
    report.push({label, ...result});
    await page.screenshot({path: path.join(out, label + '.png')});
  }
  try {
    await until(() => fetch(base + '/healthz').then(r => r.ok).catch(() => false), 'server starts');
    browser = await chromium.launch({executablePath: process.env.CK_CHROME || (process.platform === 'win32' ? 'C:/Program Files/Google/Chrome/Application/chrome.exe' : undefined),
      headless: true, args: ['--use-angle=swiftshader', '--enable-unsafe-swiftshader', '--enable-webgl']});
    const mobile = await browser.newContext({viewport: {width: 390, height: 844}, deviceScaleFactor: 1, isMobile: true, hasTouch: true,
      userAgent: 'Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Mobile Safari/537.36'});
    await mobile.addInitScript(() => {
      Object.defineProperties(navigator, {
        platform: {value: 'Linux armv8l'}, maxTouchPoints: {value: 5},
        userAgentData: {value: {platform: 'Android', mobile: true}},
      });
    });
    page = await setup(mobile, emails[0]);
    assert.equal(await page.locator('body').evaluate(el => el.classList.contains('touch-controls-enabled')), true);
    assert.equal(await page.locator('#touch-controls').isVisible(), false, 'controls hidden before play');
    assert.equal(await page.locator('#touch-mode-settings').evaluate(el => el.hidden), false, 'phone offers touch settings');
    assert.equal(await page.locator('#touch-mode-toggle').isEnabled(), true, 'phone touch toggle enabled');
    await page.screenshot({path: path.join(out, 'phone-start.png')});
    await resume();
    assert.equal((await read()).touchControls, true);
    await geometry('phone-390x844');
    for (const viewport of [{width: 844, height: 390}, {width: 320, height: 568}, {width: 1024, height: 768}]) {
      await page.setViewportSize(viewport);
      await geometry('phone-' + viewport.width + 'x' + viewport.height);
      await actionButton('menu').tap();
      await page.locator('#play-overlay').waitFor({state: 'visible'});
      assert.equal(await page.locator('#touch-mode-toggle').isVisible(), true, 'phone/tablet menu offers touch toggle');
      assert.equal(await page.locator('#touch-mode-toggle').isEnabled(), true, 'phone/tablet touch toggle enabled');
      await page.screenshot({path: path.join(out, 'menu-' + viewport.width + 'x' + viewport.height + '.png')});
      await resume();
      await actionButton('inventory').tap();
      await page.locator('#modal').waitFor({state: 'visible'});
      await panelGeometry('inventory-' + viewport.width + 'x' + viewport.height);
      await resume();
    }
    await page.setViewportSize({width: 390, height: 844});

    const cdp = await mobile.newCDPSession(page), points = new Map();
    async function touch(type, id, position) {
      const ending = type === 'touchEnd' ? points.get(id) : null;
      if (type === 'touchEnd') points.delete(id);
      else if (type === 'touchCancel') points.clear();
      else points.set(id, {id, ...position, radiusX: 4, radiusY: 4, force: 1});
      await cdp.send('Input.dispatchTouchEvent', {type, touchPoints: ending ? [ending] : [...points.values()]});
    }
    const joystick = await center('#touch-joystick');
    const jump = await center('#touch-jump');
    const start = await read();
    await page.locator('#touch-sprint').tap();
    assert.equal((await read()).movement.sprint, true, 'sprint toggle on');
    await touch('touchStart', 1, joystick);
    await touch('touchMove', 1, {x: joystick.x, y: joystick.y - 42});
    await until(async () => Math.hypot((await read()).movement.x, (await read()).movement.z) > .4, 'joystick forward');
    // Find an uncovered canvas point so the gesture tests actual world look input.
    const look = await page.evaluate(() => {
      for (const y of [.42, .5, .35]) for (const x of [.6, .7, .5]) {
        const p = {x: innerWidth * x, y: innerHeight * y};
        if (document.elementFromPoint(p.x, p.y)?.id === 'world') return p;
      }
      return null;
    });
    assert.ok(look, 'world has an uncovered look area');
    await touch('touchStart', 2, look);
    await touch('touchMove', 2, {x: look.x + 40, y: look.y + 20});
    await touch('touchStart', 3, jump);
    let current = await read();
    assert.ok(Math.abs(current.yaw - start.yaw) > .02, 'look changes yaw while walking');
    assert.ok(Math.abs(current.pitch - start.pitch) > .01, 'look changes pitch while walking');
    assert.equal(current.movement.jump, true, 'third finger holds jump');
    assert.ok(Math.hypot(current.movement.x, current.movement.z) > .4, 'moving while looking and jumping');
    await until(async () => {const s = await read(); return Math.hypot(s.me.x - start.me.x, s.me.z - start.me.z) > .3;}, 'server receives touch movement');
    await touch('touchEnd', 3);
    assert.equal((await read()).movement.jump, false, 'jump releases independently');
    await touch('touchEnd', 2);
    assert.ok(Math.hypot((await read()).movement.x, (await read()).movement.z) > .4, 'look release preserves joystick');
    await touch('touchCancel');
    await until(async () => {const m = (await read()).movement; return !m.x && !m.z && !m.jump;}, 'cancel releases every held touch');
    await page.locator('#touch-sprint').tap();
    assert.equal(stopped(await read()), true, 'sprint toggle off after cancellation');
    await geometry('phone-multitouch');

    // A panel opened with another finger must send a stopped input immediately.
    for (const name of ['inventory', 'recipes', 'map']) {
      const count = sent.length;
      await touch('touchStart', 1, joystick);
      await touch('touchMove', 1, {x: joystick.x, y: joystick.y - 42});
      const target = await center('[data-touch-action="' + name + '"]');
      await touch('touchStart', 2, target);
      await touch('touchEnd', 2);
      await page.locator('#modal').waitFor({state: 'visible'});
      assert.equal((await read()).modal, name);
      assert.equal(stopped(await read()), true, name + ' stops movement');
      assert.equal(await page.locator('#touch-controls').isVisible(), false, name + ' hides controls');
      await until(() => sent.slice(count).some(m => m.type === 'input' && !m.x && !m.z && !m.jump && !m.sprint), name + ' sends stop');
      await touch('touchEnd', 1);
      await panelGeometry('phone-' + name);
      await resume();
      assert.equal(stopped(await read()), true, name + ' resume has no stuck touch');
    }
    await touch('touchStart', 1, joystick);
    await touch('touchMove', 1, {x: joystick.x + 42, y: joystick.y});
    await touch('touchStart', 2, await center('[data-touch-action="menu"]'));
    await touch('touchEnd', 2);
    await page.locator('#play-overlay').waitFor({state: 'visible'});
    assert.equal(stopped(await read()), true, 'menu stops movement');
    assert.equal(await page.locator('#touch-controls').isVisible(), false, 'menu hides controls');
    await touch('touchEnd', 1);
    await page.screenshot({path: path.join(out, 'phone-menu.png')});
    await resume();

    const beforeSelect = sent.length;
    await touch('touchStart', 1, joystick);
    await touch('touchMove', 1, {x: joystick.x, y: joystick.y - 42});
    await touch('touchStart', 2, await center('#hotbar [data-slot="1"]'));
    await touch('touchEnd', 2);
    await until(async () => (await read()).me.selectedSlot === 1, 'tap hotbar selects server slot');
    assert.equal(sent.slice(beforeSelect).filter(m => m.action === 'select' && m.slot === 1).length, 1, 'one hotbar selection per touch');
    assert.ok(Math.hypot((await read()).movement.x, (await read()).movement.z) > .4, 'hotbar selection preserves joystick movement');
    await touch('touchStart', 2, await center('#hotbar [data-slot="0"]'));
    await touch('touchEnd', 2);
    await until(async () => (await read()).me.selectedSlot === 0, 'tap hotbar selects first slot');
    await touch('touchEnd', 1);
    const beforeThrow = (await read()).me.inventory.sugar, beforeAction = sent.length;
    await touch('touchStart', 1, joystick);
    await touch('touchMove', 1, {x: joystick.x, y: joystick.y - 42});
    await touch('touchStart', 2, await center('[data-touch-action="throw"]'));
    await touch('touchEnd', 2);
    await until(() => sent.slice(beforeAction).some(m => m.action === 'throw' && m.item === 'sugar'), 'touch throw action');
    await until(async () => (await read()).me.inventory.sugar === beforeThrow - 1, 'touch throw consumes real cookie');
    const beforeEat = (await read()).me.inventory.sugar, beforeEatAction = sent.length;
    await touch('touchStart', 2, await center('[data-touch-action="eat"]'));
    await touch('touchEnd', 2);
    await until(async () => (await read()).me.inventory.sugar === beforeEat - 1, 'touch eat consumes real cookie');
    assert.equal(sent.slice(beforeEatAction).filter(m => m.action === 'eat').length, 1, 'one eat per touch without compatibility click duplication');
    assert.ok(Math.hypot((await read()).movement.x, (await read()).movement.z) > .4, 'actions preserve joystick movement');
    await touch('touchEnd', 1);
    assert.equal(stopped(await read()), true, 'joystick releases after walking throw/eat');
    const labels = await page.evaluate(() => ({
      mounted: window.__touchLabels({me: {ridingCamel: true}}),
      activity: window.__touchLabels({me: {island: 'camel_training'}}),
      ride: window.__touchLabels({rollerCoaster: {ride: {}}}),
      exercise: window.__touchLabels({cookiePractice: {exercise: {station: 'practice_treadmill'}}}),
      placing: window.__touchLabels({}, {scope: 'bakery', item: 'chair'}),
    }));
    assert.equal(labels.mounted.eat, 'Dismount');
    assert.equal(labels.activity.eat, 'Activity');
    assert.equal(labels.activity.activity, true);
    assert.equal(labels.ride.eat, 'Ride');
    assert.equal(labels.ride.activity, true);
    assert.equal(labels.exercise.eat, 'Stop');
    assert.equal(labels.placing.throw, 'Place');
    assert.equal(labels.placing.build, true);

    await touch('touchStart', 1, joystick);
    await touch('touchMove', 1, {x: joystick.x, y: joystick.y - 42});
    await touch('touchStart', 3, jump);
    await page.evaluate(() => window.dispatchEvent(new Event('blur')));
    assert.equal(stopped(await read()), true, 'blur clears movement and jump');
    await touch('touchCancel');
    await resume();
    const resizedJoystick = await center('#touch-joystick');
    await touch('touchStart', 1, resizedJoystick);
    await touch('touchMove', 1, {x: resizedJoystick.x, y: resizedJoystick.y - 35});
    await page.evaluate(() => window.__touchDisconnect());
    await until(async () => !(await read()).connected, 'disconnect observed');
    assert.equal(stopped(await read()), true, 'disconnect clears movement');
    assert.equal(await page.locator('#touch-controls').isVisible(), false, 'disconnect hides controls');
    await touch('touchCancel');
    await page.screenshot({path: path.join(out, 'phone-disconnected.png')});

    const desktop = await browser.newContext({viewport: {width: 1280, height: 800}, hasTouch: false});
    page = await setup(desktop, emails[1]);
    await checkDesktopControls('desktop');
    await desktop.close();

    // Windows touch hardware can natively report the same input media as a phone.
    const windowsTouch = await browser.newContext({viewport: {width: 1920, height: 1080}, isMobile: false, hasTouch: true,
      userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36'});
    await windowsTouch.addInitScript(() => {
      Object.defineProperties(navigator, {
        platform: {value: 'Win32'}, maxTouchPoints: {value: 10},
        userAgentData: {value: {platform: 'Windows', mobile: false}},
      });
    });
    page = await setup(windowsTouch, emails[3]);
    assert.equal(await page.evaluate(() => matchMedia('(pointer: coarse) and (hover: none)').matches), true,
      'Windows touchscreen reproduces the phone-like media query without overriding matchMedia');
    await checkDesktopControls('windows-touch');
    await windowsTouch.close();

    // Model coarse input with hover, then a device changing between touch and
    // mouse input on an iPad using its desktop browser identity.
    const hybrid = await browser.newContext({viewport: {width: 1024, height: 768}, hasTouch: true,
      userAgent: 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Safari/605.1.15'});
    await hybrid.addInitScript(() => {
      Object.defineProperties(navigator, {
        platform: {value: 'MacIntel'}, maxTouchPoints: {value: 5}, userAgentData: {value: undefined},
      });
      let profile = {pointer: 'coarse', hover: 'hover'};
      const nativeMatchMedia = window.matchMedia.bind(window), queries = new Set();
      window.matchMedia = query => {
        const features = [...query.matchAll(/\((pointer|hover):\s*(\w+)\)/g)];
        const media = nativeMatchMedia(query);
        if (!features.length) return media;
        Object.defineProperty(media, 'matches', {get: () => features.every(([, name, value]) => profile[name] === value)});
        queries.add(media);
        return media;
      };
      window.__setPointerProfile = next => {
        const before = new Map([...queries].map(media => [media, media.matches]));
        profile = next;
        for (const media of queries) if (media.matches !== before.get(media)) {
          media.dispatchEvent(new MediaQueryListEvent('change', {matches: media.matches, media: media.media}));
        }
      };
    });
    page = await setup(hybrid, emails[2]);
    assert.equal((await read()).touchControls, false, 'coarse input with hover stays in desktop mode');
    assert.equal(await page.locator('#touch-mode-settings').evaluate(el => el.hidden), true, 'hover device hides touch settings');
    assert.equal(await page.locator('#touch-mode-toggle').isDisabled(), true, 'hover device disables touch toggle');
    await page.locator('#touch-mode-toggle').dispatchEvent('click');
    assert.equal((await read()).touchControls, false, 'hover device cannot force touch mode');
    await page.evaluate(() => window.__setPointerProfile({pointer: 'coarse', hover: 'none'}));
    assert.equal((await read()).touchControls, true, 'changing to touch input enables mobile controls');
    assert.equal(await page.locator('#touch-mode-settings').evaluate(el => el.hidden), false, 'touch input exposes its settings');
    assert.equal(await page.locator('#touch-mode-toggle').isEnabled(), true, 'touch input enables its toggle');
    await resume();
    await page.locator('#touch-sprint').tap();
    const hybridStick = await center('#touch-joystick');
    await page.locator('#touch-joystick').dispatchEvent('pointerdown', {pointerType: 'touch', pointerId: 71,
      clientX: hybridStick.x, clientY: hybridStick.y - 40});
    assert.equal((await read()).movement.sprint, true, 'touch sprint active before changing input');
    assert.ok(Math.hypot((await read()).movement.x, (await read()).movement.z) > .4, 'touch movement active before changing input');
    await page.evaluate(() => window.__setPointerProfile({pointer: 'fine', hover: 'hover'}));
    const afterInputChange = await read();
    assert.equal(afterInputChange.touchControls, false, 'changing to mouse input disables mobile controls');
    assert.equal(afterInputChange.locked || afterInputChange.playing, false, 'changing input waits for desktop pointer lock');
    assert.equal(stopped(afterInputChange), true, 'changing input clears held touch movement and sprint');
    assert.equal(await page.locator('#touch-controls').isVisible(), false, 'changing to mouse input hides touch controls');
    assert.equal(await page.locator('#touch-mode-settings').evaluate(el => el.hidden), true, 'changing to mouse input hides touch settings');
    assert.equal(await page.locator('#touch-mode-toggle').isDisabled(), true, 'changing to mouse input disables touch toggle');
    assert.deepEqual(errors, [], 'no browser runtime errors');
    assert.deepEqual(requestsFailed, [], 'no failed HTTP requests');
    console.log('PASS: Android multi-touch controls, desktop-mode iPad controls, phone/tablet layouts, Windows coarse/no-hover touchscreen rejection, desktop keyboard/pointer-lock preservation, and input-device changes.');
    console.log('Screenshots and geometry: ' + out);
  } catch (error) {
    if (page) {
      await page.screenshot({path: path.join(out, 'failure.png')}).catch(() => {});
      fs.writeFileSync(path.join(out, 'failure.json'), JSON.stringify({errors, requestsFailed, state: await read().catch(() => null), sent: sent.slice(-30)}, null, 2));
    }
    throw error;
  } finally {
    fs.writeFileSync(path.join(out, 'geometry.json'), JSON.stringify(report, null, 2));
    fs.writeFileSync(path.join(out, 'server.log'), log);
    await browser?.close();
    server.kill();
    await new Promise(resolve => {if (server.exitCode !== null) return resolve(); server.once('exit', resolve); setTimeout(resolve, 3000);});
  }
})().catch(error => {console.error(error); process.exitCode = 1;});
