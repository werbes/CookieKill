// Requires the same Playwright/Chrome installation as browser-smoke.cjs.
// Start an isolated development server, then set CK_TEST_URL to its URL.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { chromium } = require(require.resolve('playwright', { paths: [path.resolve(__dirname, '../.tools')] }));

(async () => {
  const baseURL = process.env.CK_TEST_URL || 'http://127.0.0.1:8080';
  const executablePath = process.env.CK_CHROME || (process.platform === 'win32' ? 'C:/Program Files/Google/Chrome/Application/chrome.exe' : undefined);
  const browser = await chromium.launch({
    executablePath,
    headless: true,
    args: process.env.CK_SMOKE_HARDWARE === '1' ? [] : ['--use-angle=swiftshader', '--enable-unsafe-swiftshader', '--enable-webgl'],
  });
  const output = path.resolve(__dirname, '../test-results');
  fs.mkdirSync(output, { recursive: true });
  const errors = [], failed = [];
  const cookieName = 'cookiekill_session';
  let activePage;

  async function newPage(cookies = []) {
    const context = await browser.newContext({ viewport: { width: 1366, height: 768 } });
    await context.addCookies(cookies);
    const page = await context.newPage();
    activePage = page;
    page.on('pageerror', error => errors.push(error.message));
    page.on('response', response => {
      if (response.status() >= 400) failed.push(`${response.status()} ${response.url()}`);
    });
    const authRequests = [];
    page.on('request', request => {
      if (['/api/login', '/api/verify'].includes(new URL(request.url()).pathname)) authRequests.push(request);
    });
    return { context, page, authRequests };
  }

  async function screenshot(page, name) {
    await page.screenshot({ path: path.join(output, `auth-${name}.png`) });
  }

  async function checkLoginLayout(page, stage) {
    for (const viewport of [{ width: 1366, height: 768 }, { width: 1280, height: 720 }]) {
      await page.setViewportSize(viewport);
      await screenshot(page, `${stage}-${viewport.width}x${viewport.height}`);
      const control = await page.locator('.remember-device').boundingBox();
      const card = await page.locator('.login-card').boundingBox();
      const footer = await page.locator('.landing-bottom').boundingBox();
      const perks = await page.locator('.landing-perks').boundingBox();
      assert.ok(control && control.y >= 0 && control.y + control.height <= viewport.height, 'keep signed in control must fit on screen');
      assert.ok(card && footer && card.y + card.height <= footer.y, 'login card overlaps the footer');
      if (perks) assert.ok(perks.y + perks.height <= footer.y, 'login perks overlap the footer');
    }
  }

  async function openLogin(page) {
    await page.goto(baseURL, { waitUntil: 'networkidle' });
    await page.locator('#email').waitFor({ state: 'visible' });
    assert.equal(await page.locator('#render-error').isVisible(), false, 'WebGL failed');
    const checkbox = page.getByRole('checkbox', { name: 'Keep me signed in on this device', exact: true });
    assert.equal(await checkbox.count(), 1, 'checkbox must have an accessible label');
    assert.equal(await checkbox.isChecked(), false, 'keep signed in must be opt-in');
    assert.match(await page.locator('#keep-signed-in-help').innerText(), /30 days.*devices you own/i);
    return checkbox;
  }

  async function login(page, remember) {
    const checkbox = page.locator('#keep-signed-in');
    await checkbox.setChecked(remember);
    await page.locator('#email').fill(`auth-smoke-${remember ? 'remembered' : 'ordinary'}-${Date.now()}@example.com`);
    await page.locator('#email-submit').click();
    await page.locator('#code-stage').waitFor({ state: 'visible' });
    assert.equal(await checkbox.isChecked(), remember, 'checkbox choice must survive the code step');
    if (!await page.locator('#login-code').inputValue()) {
      const code = (await page.locator('#development-code').innerText()).match(/\b\d{6}\b/);
      assert.ok(code, 'isolated development server must provide a local login code');
      await page.locator('#login-code').fill(code[0]);
    }
    if (remember) await checkLoginLayout(page, 'code-remembered');
    const verified = page.waitForResponse(response => new URL(response.url()).pathname === '/api/verify');
    await page.locator('#code-submit').click();
    const response = await verified;
    assert.equal(response.status(), 200, 'email verification failed');
    assert.equal(response.request().postDataJSON().keepSignedIn, remember, 'verify request must send the checkbox choice');
    await page.locator('#start-button').waitFor({ state: 'visible' });
    return (await response.json()).player;
  }

  async function session(page) {
    return page.evaluate(async () => (await fetch('/api/session')).json());
  }

  try {
    const ordinary = await newPage();
    await openLogin(ordinary.page);
    await checkLoginLayout(ordinary.page, 'login');
    await login(ordinary.page, false);
    const ordinaryCookie = (await ordinary.context.cookies()).find(cookie => cookie.name === cookieName);
    assert.ok(ordinaryCookie, 'ordinary login did not create a session cookie');
    assert.equal(ordinaryCookie.expires, -1, 'ordinary login must use a browser-session cookie');
    assert.equal(ordinaryCookie.httpOnly, true, 'session cookie must be HTTP-only');
    const ordinaryPersistentCookies = (await ordinary.context.cookies()).filter(cookie => cookie.expires > Date.now() / 1000);
    await ordinary.context.close();

    // Browser restart carries only durable cookies; a browser-session cookie is discarded.
    const remembered = await newPage(ordinaryPersistentCookies);
    await openLogin(remembered.page);
    assert.equal((await session(remembered.page)).authenticated, false, 'ordinary session survived a clean browser restart');
    const player = await login(remembered.page, true);
    const rememberedCookie = (await remembered.context.cookies()).find(cookie => cookie.name === cookieName);
    assert.ok(rememberedCookie, 'remembered login did not create a session cookie');
    assert.equal(rememberedCookie.httpOnly, true, 'remembered cookie must be HTTP-only');
    const lifetime = rememberedCookie.expires - Date.now() / 1000;
    assert.ok(Math.abs(lifetime - 30 * 24 * 60 * 60) < 120, `remembered cookie must last 30 days, got ${lifetime} seconds`);
    remembered.authRequests.length = 0;
    await remembered.page.reload({ waitUntil: 'networkidle' });
    await remembered.page.locator('#start-button').waitFor({ state: 'visible' });
    assert.equal((await session(remembered.page)).player.id, player.id, 'reload lost the remembered identity');
    assert.equal(remembered.authRequests.length, 0, 'reload requested another email verification');
    const persistentCookies = (await remembered.context.cookies()).filter(cookie => cookie.expires > Date.now() / 1000);
    await remembered.context.close();

    const resumed = await newPage(persistentCookies);
    await resumed.page.goto(baseURL, { waitUntil: 'networkidle' });
    await resumed.page.locator('#start-button').waitFor({ state: 'visible' });
    assert.equal((await session(resumed.page)).player.id, player.id, 'browser restart lost remembered identity');
    assert.equal(await resumed.page.locator('#landing').isVisible(), false, 'remembered device showed the login box');
    assert.equal(resumed.authRequests.length, 0, 'remembered device requested another email verification');

    await resumed.page.locator('#start-button').click();
    await resumed.page.waitForFunction(() => !!document.pointerLockElement);
    await resumed.page.keyboard.press('Escape');
    await resumed.page.locator('#logout-button').waitFor({ state: 'visible' });
    await resumed.page.locator('#logout-button').click();
    await resumed.page.locator('#email').waitFor({ state: 'visible' });
    assert.equal((await resumed.context.cookies()).some(cookie => cookie.name === cookieName), false, 'sign out did not clear remembered cookie');
    await resumed.context.close();

    // Replaying the old cookie must fail even though its browser expiry is still in the future.
    const revoked = await newPage(persistentCookies);
    await openLogin(revoked.page);
    assert.equal((await session(revoked.page)).authenticated, false, 'sign out did not revoke the remembered session');
    assert.deepEqual(errors, [], 'browser runtime errors');
    assert.deepEqual(failed, [], 'failed asset/API requests');
    console.log('PASS: accessible opt-in checkbox; ordinary browser-session cookie; 30-day remembered cookie; reload and fresh browser context restore identity without email verification; sign out revokes remembered session; no browser errors.');
    console.log('Screenshots: ' + output);
  } catch (error) {
    if (activePage && !activePage.isClosed()) await screenshot(activePage, 'failure').catch(() => {});
    console.error('Browser diagnostics', { errors, failed });
    throw error;
  } finally {
    await browser.close();
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
