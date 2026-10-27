const assert = require('node:assert/strict');
const fs = require('node:fs');
const http = require('node:http');
const path = require('node:path');
const test = require('node:test');

const playwrightPath = path.join(__dirname, '..', '.tools', 'node_modules', 'playwright');

test('browser retries one proxy 429 during the module graph load', { skip: !fs.existsSync(playwrightPath) }, async () => {
  const { chromium } = require(playwrightPath);
  const startScript = fs.readFileSync(path.join(__dirname, '..', 'web', 'start.js'));
  let gameRequests = 0;
  let moduleRequests = 0;
  const server = http.createServer((request, response) => {
    const url = new URL(request.url, 'http://localhost');
    if (url.pathname === '/') {
      response.setHeader('Content-Type', 'text/html');
      response.end('<!doctype html><div id="loading"><span></span><span>Warming up</span></div><script type="module" src="/start.js"></script>');
      return;
    }
    if (url.pathname === '/start.js') {
      response.setHeader('Content-Type', 'text/javascript');
      response.end(startScript);
      return;
    }
    if (url.pathname === '/game.js') {
      gameRequests++;
      response.setHeader('Content-Type', 'text/javascript');
      response.end("import './night-scene.js'; document.body.dataset.loaded = 'yes';");
      return;
    }
    if (url.pathname === '/night-scene.js') {
      moduleRequests++;
      if (moduleRequests === 1) {
        response.statusCode = 429;
        response.setHeader('Content-Type', 'text/plain');
        response.end('Too Many Requests');
      } else {
        response.setHeader('Content-Type', 'text/javascript');
        response.end('export const loaded = true;');
      }
      return;
    }
    response.statusCode = 404;
    response.end();
  });
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  const browser = await chromium.launch({
    headless: true,
    executablePath: process.env.CK_CHROME || 'C:/Program Files/Google/Chrome/Application/chrome.exe',
  });
  try {
    const page = await browser.newPage();
    await page.goto(`http://127.0.0.1:${server.address().port}/`);
    await page.waitForFunction(() => document.body.dataset.loaded === 'yes', null, { timeout: 5000 });
    assert.equal(gameRequests, 2);
    assert.equal(moduleRequests, 2);
    assert.equal(await page.locator('#loading span:last-child').textContent(), 'Warming up the world...');
  } finally {
    await browser.close();
    await new Promise(resolve => server.close(resolve));
  }
});
