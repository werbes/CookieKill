const loading = document.getElementById('loading');
const loadingMessage = loading?.querySelector('span:last-child');
const retryDelays = [1000, 2500, 5000, 10000];

function isTransientModuleFailure(error) {
  const message = String(error?.message || error);
  return /failed to fetch dynamically imported module|error loading dynamically imported module|importing a module script failed|networkerror|load failed/i.test(message);
}

function setLoadingMessage(message) {
  if (loadingMessage) loadingMessage.textContent = message;
}

async function startWorld() {
  for (let attempt = 0; ; attempt++) {
    try {
      await import(attempt === 0 ? './game.js' : `./game.js?retry=${attempt}`);
      return;
    } catch (error) {
      console.error('CookieKill module loading failed', error);
      const delay = retryDelays[attempt];
      if (!isTransientModuleFailure(error) || delay === undefined) {
        setLoadingMessage('The world could not load. Please check your connection and reload.');
        return;
      }
      setLoadingMessage(`The server is busy. Retrying in ${Math.ceil(delay / 1000)} seconds...`);
      await new Promise(resolve => setTimeout(resolve, delay));
      setLoadingMessage('Warming up the world...');
    }
  }
}

startWorld();
