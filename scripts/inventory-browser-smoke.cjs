// Real browser inventory/drop checks against an automatically isolated server.
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const net=require('node:net');
const {spawn,spawnSync}=require('node:child_process');
const {chromium}=require(require.resolve('playwright',{paths:[path.resolve(__dirname,'../.tools')]}));

(async()=>{
  const root=path.resolve(__dirname,'..');
  const output=path.join(root,'test-results',`inventory-browser-${Date.now()}`),data=path.join(output,'data');
  fs.mkdirSync(data,{recursive:true});
  const executable=path.join(output,process.platform==='win32'?'server.exe':'server');
  const build=spawnSync(process.env.CK_GO||(process.platform==='win32'?'C:/Program Files/Go/bin/go.exe':'go'),['build','-o',executable,'.'],{cwd:root,windowsHide:true,encoding:'utf8'});
  assert.equal(build.status,0,build.stderr||build.stdout);
  const port=await new Promise((resolve,reject)=>{const probe=net.createServer();probe.once('error',reject);probe.listen(0,'127.0.0.1',()=>{const selected=probe.address().port;probe.close(error=>error?reject(error):resolve(selected));});});
  const baseURL=`http://127.0.0.1:${port}`;
  const server=spawn(executable,['-dev','-addr',`127.0.0.1:${port}`,'-data',data],{cwd:root,windowsHide:true,stdio:['ignore','pipe','pipe']});
  let serverLog='';server.stdout.on('data',chunk=>serverLog+=chunk);server.stderr.on('data',chunk=>serverLog+=chunk);
  const started=Date.now();
  try{while(!await fetch(baseURL+'/api/world').then(r=>r.ok).catch(()=>false)){assert.ok(Date.now()-started<20000,'isolated server did not start: '+serverLog);await new Promise(resolve=>setTimeout(resolve,100));}}
  catch(error){server.kill();throw error;}
  const source=fs.readFileSync(path.resolve(__dirname,'../web/state-sync.js'),'utf8');
  const {createSnapshotDecoder}=await import(`data:text/javascript;base64,${Buffer.from(source).toString('base64')}`);
  let browser;
  try{browser=await chromium.launch({executablePath:process.env.CK_CHROME||(process.platform==='win32'?'C:/Program Files/Google/Chrome/Application/chrome.exe':undefined),headless:true,args:['--use-angle=swiftshader','--enable-unsafe-swiftshader','--enable-webgl']});}catch(error){server.kill();throw error;}
  const page=await browser.newPage({viewport:{width:1440,height:1000}}),errors=[];
  // Test movement still passes through the normal server-authoritative input
  // path; no coordinates or inventory are changed directly in the game.
  await page.addInitScript(()=>{const send=WebSocket.prototype.send;WebSocket.prototype.send=function(payload){try{const m=JSON.parse(payload);if(m.type==='input'&&window.__inventoryMovement)payload=JSON.stringify({...m,...window.__inventoryMovement});}catch{}return send.call(this,payload);};});
  let latest;
  page.on('pageerror',error=>errors.push(error.message));
  page.on('websocket',socket=>{const decoder=createSnapshotDecoder();socket.on('framereceived',({payload})=>{const next=decoder.apply(JSON.parse(payload.toString()));if(next)latest=next;});});
  const until=async(predicate,label)=>{const start=Date.now();while(!await predicate()){assert.ok(Date.now()-start<12000,label);await page.waitForTimeout(100);}};
  const drops=()=>latest?.nodes?.filter(n=>n.kind==='item_drop')||[];
  const openInventory=async()=>{await page.keyboard.press('i');await page.locator('#modal').waitFor({state:'visible'});};
  const resume=async()=>{await page.locator('#close-modal').click();if(await page.locator('#capture-prompt').isVisible())await page.locator('#capture-prompt').click();await page.waitForFunction(()=>!!document.pointerLockElement);await page.waitForTimeout(180);};
  const approach=async n=>{const started=Date.now();try{while(Math.hypot(n.x-latest.me.x,n.z-latest.me.z)>.18){assert.ok(Date.now()-started<15000,'route to dropped item blocked');const dx=n.x-latest.me.x,dz=n.z-latest.me.z;await page.evaluate(input=>{window.__inventoryMovement=input;},{x:dx/3,z:dz/3,sprint:false});await page.waitForTimeout(100);}}finally{await page.evaluate(()=>{window.__inventoryMovement={x:0,z:0,sprint:false};});}await page.waitForTimeout(180);};
  try{
    await page.goto(baseURL,{waitUntil:'networkidle'});
    await page.locator('#email').fill(`inventory${Date.now()}@example.com`);
    await page.locator('#email-submit').click();await page.locator('#code-stage').waitFor({state:'visible'});
    assert.match(await page.locator('#login-code').inputValue(),/^\d{6}$/,'isolated development server must provide a code');
    await page.locator('#code-submit').click();await page.locator('#start-button').waitFor({state:'visible'});await until(()=>latest?.me,'snapshot missing');
    assert.equal(latest.me.inventoryVersion,3);assert.equal(latest.me.bagSlots.length,20);assert.equal(latest.me.hotbar.length,5);assert.equal(latest.me.safeSlots,undefined);
    await page.locator('#start-button').click();await page.waitForFunction(()=>!!document.pointerLockElement);
    await openInventory();assert.equal(await page.locator('.storage-bag .storage-slot').count(),20);assert.equal(await page.locator('.storage-hotbar .storage-slot').count(),5);assert.equal(await page.locator('.storage-safe').count(),0);
    await page.locator('.storage-hotbar .storage-slot').first().click();
    for(const label of ['Drop 1','Drop 5','Drop 10','Drop all'])assert.equal(await page.getByRole('button',{name:label,exact:true}).isVisible(),true);
    assert.equal(await page.locator('.reputation-pill').count(),6);
    await page.screenshot({path:path.join(output,'inventory-protected-drop-controls.png')});
    await page.getByRole('button',{name:'Drop 10',exact:true}).click();await until(()=>latest.me.inventory.sugar===undefined&&drops().length===1,'Drop10 did not create a ground bundle');
    assert.equal(drops()[0].amount,10);assert.equal(drops()[0].item,'sugar');
    await resume();await approach(drops()[0]);await page.waitForFunction(()=>document.getElementById('interaction-label').textContent.startsWith('Pick up'));
    await page.keyboard.press('e');await until(()=>latest.me.inventory.sugar===10&&drops().length===0,'E pickup did not restore the actual items');
    await openInventory();const index=latest.me.bagSlots.findIndex(s=>s.item==='sugar');assert.ok(index>=0);
    await page.locator('.storage-bag .storage-slot').nth(index).click();await page.getByRole('button',{name:'Drop 1',exact:true}).click();await until(()=>latest.me.inventory.sugar===9,'Drop1 failed');
    await page.getByRole('button',{name:'Drop 5',exact:true}).click();await until(()=>latest.me.inventory.sugar===4,'Drop5 failed');
    assert.equal(await page.getByRole('button',{name:'Drop 10',exact:true}).isDisabled(),true);
    await page.getByRole('button',{name:'Drop all',exact:true}).click();await until(()=>latest.me.inventory.sugar===undefined&&drops().length===6,'Dropall did not scatter four singles');
    assert.deepEqual(drops().map(n=>n.amount).sort((a,b)=>a-b),[1,1,1,1,1,5]);
    await until(()=>page.locator('.storage-slot.picked').count().then(count=>count===0),'dropping the whole stack left a stale selection');
    assert.equal(await page.getByRole('button',{name:'Drop all',exact:true}).count(),0);
    await resume();await page.evaluate(dy=>{const event=new MouseEvent('mousemove');Object.defineProperty(event,'movementY',{value:dy});document.dispatchEvent(event);},(latest.me.pitch+1.2)/.002);
    await until(()=>latest.me.pitch< -1.1,'camera did not turn toward the dropped items');await page.waitForTimeout(180);await page.screenshot({path:path.join(output,'inventory-ground-singles-bundle.png')});
    for(let i=0;i<6;i++){const before=drops().length,target=drops().sort((a,b)=>Math.hypot(a.x-latest.me.x,a.z-latest.me.z)-Math.hypot(b.x-latest.me.x,b.z-latest.me.z))[0];await approach(target);await page.waitForFunction(()=>document.getElementById('interaction-label').textContent.startsWith('Pick up'));await page.keyboard.press('e');await until(()=>drops().length<before,'E pickup missed a scattered item');}
    assert.equal(latest.me.inventory.sugar,10);assert.equal(drops().length,0);assert.deepEqual(errors,[]);
    console.log('Inventory Chrome smoke passed: protected20+5slots, Drop1/5/10/all, grouping, E pickup, item conservation.');
    console.log('Screenshots: '+output);
  }catch(error){await page.screenshot({path:path.join(output,'inventory-smoke-failure.png')}).catch(()=>{});throw error;}
  finally{await browser.close();server.kill();await Promise.race([new Promise(resolve=>server.once('exit',resolve)),new Promise(resolve=>setTimeout(resolve,3000))]);fs.writeFileSync(path.join(output,'server.log'),serverLog);}
})().catch(error=>{console.error(error);process.exitCode=1;});
