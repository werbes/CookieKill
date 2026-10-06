// Real Chrome integration: npm install --prefix .tools --no-save playwright
// Start an isolated development server, then set CK_TEST_URL to its URL.
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {chromium}=require(require.resolve('playwright',{paths:[path.resolve(__dirname,'../.tools')]}));
(async()=>{
  const syncSource=fs.readFileSync(path.join(__dirname,'../web/state-sync.js'),'utf8');
  const {createSnapshotDecoder}=await import(`data:text/javascript;base64,${Buffer.from(syncSource).toString('base64')}`);
  const baseURL=process.env.CK_TEST_URL||'http://127.0.0.1:8080';
  const executablePath=process.env.CK_CHROME||(process.platform==='win32'?'C:/Program Files/Google/Chrome/Application/chrome.exe':undefined);
  const browser=await chromium.launch({executablePath,headless:true,args:['--use-angle=swiftshader','--enable-unsafe-swiftshader','--enable-webgl']});
  const context=await browser.newContext({viewport:{width:1440,height:1000}}),page=await context.newPage();
  // Tour input remains ordinary movement, validated against server collisions.
  await page.addInitScript(()=>{const send=WebSocket.prototype.send;WebSocket.prototype.send=function(payload){window.__smokeSocket=this;try{const m=JSON.parse(payload);if(m.type==='input'&&window.__smokeInput)payload=JSON.stringify({...m,...window.__smokeInput});}catch{}return send.call(this,payload);};});
  const errors=[],failed=[];let latest,layout,activeSocket,connectionCount=0,deltaCount=0,retainedRecipes=false,retainedNodes=false;
  page.on('pageerror',e=>errors.push(e.message));
  page.on('response',r=>{if(r.status()>=400)failed.push(`${r.status()} ${r.url()}`);});
  page.on('websocket',socket=>{
    activeSocket=socket;latest=undefined;connectionCount++;
    const decoder=createSnapshotDecoder();
    if(new URL(socket.url()).searchParams.get('updates')!=='delta-v1')errors.push('Browser did not request delta updates');
    socket.on('framereceived',({payload})=>{
      if(socket!==activeSocket)return;
      try{
        const m=JSON.parse(payload.toString()),before=latest,next=decoder.apply(m);
        if(!next)return;
        latest=next;if(next.layout)layout=next.layout;
        if(m.type==='delta'){
          deltaCount++;
          if(!Object.hasOwn(m,'recipes')){assert.strictEqual(next.recipes,before.recipes);retainedRecipes=next.recipes.length>0;}
          if(!Object.hasOwn(m,'nodes')){assert.strictEqual(next.nodes,before.nodes);retainedNodes=next.nodes.length>0;}
        }
      }catch(error){errors.push('WebSocket decoding: '+error.message);}
    });
  });
  const output=path.resolve(__dirname,'../test-results');fs.mkdirSync(output,{recursive:true});
  const screenshot=name=>page.screenshot({path:path.join(output,name+'.png')});
  async function until(predicate,label){const start=Date.now();while(!predicate()){assert.ok(Date.now()-start<7000,label);await page.waitForTimeout(100);}}
  async function resume(){if(await page.locator('#modal').isVisible())await page.locator('#close-modal').click();if(await page.locator('#capture-prompt').isVisible())await page.locator('#capture-prompt').click();await page.waitForFunction(()=>!!document.pointerLockElement,null,{timeout:5000});assert.equal(await page.locator('#play-overlay').isVisible(),false,'panel close opened Escape menu');}
  try{
    await page.goto(baseURL,{waitUntil:'networkidle'});await page.locator('#email').waitFor({state:'visible'});
    assert.equal(await page.locator('#render-error').isVisible(),false,'WebGL failed');
    const publicLayout=await page.evaluate(async()=>await(await fetch('/api/world')).json());
    assert.equal(publicLayout.layout.version,4);assert.equal(publicLayout.layout.plots.length,12);assert.equal(publicLayout.layout.zones.length,0);assert.ok(new Set(publicLayout.layout.coast.map(p=>p.z.toFixed(1))).size>10,'coast is not curved');
    await screenshot('landing');
    await page.locator('#email').fill(`smoke${Date.now()}@example.com`);await page.locator('#email-submit').click();await page.locator('#code-stage').waitFor({state:'visible'});
    if(!await page.locator('#login-code').inputValue()){const code=(await page.locator('#development-code').innerText()).match(/\b\d{6}\b/);assert.ok(code,'local code missing');await page.locator('#login-code').fill(code[0]);}
    await page.locator('#code-submit').click();await page.locator('#start-button').waitFor({state:'visible'});await until(()=>latest?.me,'missing snapshot');
    assert.equal(latest.me.inventory.sugar,10);assert.equal(latest.me.safeSlots.length,3);assert.equal(latest.me.bagSlots.length,15);assert.equal(latest.me.hotbar.length,5);assert.equal(latest.me.discovered?.sugar||false,false);
    await until(()=>deltaCount>0&&retainedRecipes&&retainedNodes,'delta stream did not retain unchanged recipes and nodes');
    assert.equal(await page.locator('#play-overlay').isVisible(),false,'Escape menu shown before Escape');
    if(!process.argv.includes('--tour-only')){
    await page.locator('#start-button').click();await page.waitForFunction(()=>!!document.pointerLockElement);
    assert.equal(await page.locator('#hotbar .hotbar-slot').count(),5);assert.match(await page.locator('#selected-cookie-label').innerText(),/UNIDENTIFIED COOKIE/);
    const initial={x:latest.me.x,z:latest.me.z};await page.keyboard.down('w');await page.waitForTimeout(900);await page.keyboard.up('w');await page.waitForTimeout(200);
    assert.ok(Math.hypot(latest.me.x-initial.x,latest.me.z-initial.z)>.5,'movement did not reach server');
    await page.mouse.click(720,500);await until(()=>latest.me.inventory.sugar===9,'throw did not consume cookie');await screenshot('first-person');
    await page.keyboard.press('2');await until(()=>latest.me.selectedSlot===1,'hotbar key 2 ignored');await page.waitForFunction(()=>document.getElementById('selected-cookie-label').textContent.startsWith('EMPTY SLOT'));assert.match(await page.locator('#selected-cookie-label').innerText(),/EMPTY SLOT/);
    await page.keyboard.press('1');await until(()=>latest.me.selectedSlot===0,'hotbar key 1 ignored');
    await page.keyboard.press('i');await page.locator('#modal').waitFor({state:'visible'});
    assert.equal(await page.locator('.storage-safe .storage-slot').count(),3);assert.equal(await page.locator('.storage-bag .storage-slot').count(),15);assert.equal(await page.locator('.storage-hotbar .storage-slot').count(),5);
    await page.locator('.storage-hotbar .storage-slot').nth(0).click();await page.locator('.storage-safe .storage-slot').nth(0).click();await until(()=>latest.me.safeSlots[0]?.count===9,'hotbar to safe move failed');
    await page.locator('.storage-safe .storage-slot').nth(0).click();await page.locator('.storage-bag .storage-slot').nth(0).click();await until(()=>latest.me.bagSlots[0]?.count===9,'safe to bag move failed');
    await page.locator('.storage-bag .storage-slot').nth(0).click();await page.locator('.storage-hotbar .storage-slot').nth(2).click();await until(()=>latest.me.hotbar[2]?.count===9,'bag to hotbar move failed');await screenshot('inventory');
    await resume();await page.keyboard.press('3');await until(()=>latest.me.selectedSlot===2,'hotbar key 3 ignored');await page.keyboard.press('q');await until(()=>latest.me.inventory.sugar===8,'hotbar eat failed');
    await page.keyboard.press('c');await page.locator('#modal').waitFor({state:'visible'});assert.equal(await page.locator('.recipe-mystery').count(),8);assert.equal(await page.locator('.recipe-stats').count(),0,'undiscovered effects leaked');assert.doesNotMatch(await page.locator('#modal-content').innerText(),/10 seconds of power|Slow your rivals|Berry cookie|Sugar cookie/i);await screenshot('cookbook');await resume();
    await page.evaluate(()=>document.exitPointerLock());await page.waitForFunction(()=>!document.pointerLockElement);await page.waitForTimeout(200);assert.equal(await page.locator('#play-overlay').isVisible(),false,'unexpected pointer unlock opened Escape menu');assert.equal(await page.locator('#capture-prompt').isVisible(),true,'unexpected unlock needs small capture prompt');await page.locator('#capture-prompt').click();await page.waitForFunction(()=>!!document.pointerLockElement);
    await page.keyboard.press('Escape');await page.locator('#play-overlay').waitFor({state:'visible'});await screenshot('escape-menu');
    await page.keyboard.press('Escape');await page.locator('#play-overlay').waitFor({state:'hidden'});await page.waitForTimeout(250);assert.equal(await page.locator('#play-overlay').isVisible(),false,'Escape close reopened menu');
    if(await page.locator('#capture-prompt').isVisible())await page.locator('#capture-prompt').click();await page.waitForFunction(()=>!!document.pointerLockElement);
    await page.keyboard.press('Escape');await page.locator('#play-overlay').waitFor({state:'visible'});await page.locator('#customize-button').click();await page.locator('#customize-form').waitFor({state:'visible'});
    assert.equal(await page.locator('.skin-swatch').count(),5);assert.equal(await page.locator('#permanent-username').getAttribute('readonly'),'');assert.equal(await page.locator('.hat-option.locked button:disabled').count(),5);
    const username=latest.me.username,newName='Crumb Explorer';await page.locator('#call-name-input').fill(newName);await page.locator('.skin-swatch').nth(4).click();await page.locator('select[data-avatar="shirt"]').selectOption('hoodie');await page.locator('select[data-avatar="shirtColor"]').selectOption('purple');await page.locator('select[data-avatar="pants"]').selectOption('shorts');await page.locator('#customize-form button[type="submit"]').click();
    await until(()=>latest.me.callName===newName&&latest.me.avatar.skin===4&&latest.me.avatar.shirt==='hoodie','customization did not save');assert.equal(latest.me.username,username);assert.ok(latest.me.callNameChangedAt>0);await screenshot('customize');
    await page.locator('#call-name-input').fill('Another name');assert.equal(await page.locator('#customize-form button[type="submit"]').isDisabled(),true,'hourly call-name limit missing');await resume();
    await page.keyboard.press('m');await page.locator('#modal').waitFor({state:'visible'});assert.match(await page.locator('#modal-content').innerText(),/Wildwood/i);assert.match(await page.locator('#modal-content').innerText(),/Crumb City/i);await screenshot('map');await resume();
    assert.equal(await page.locator('#play-overlay').isVisible(),false,'closing map opened pause menu');
    await page.reload({waitUntil:'networkidle'});await page.locator('#start-button').waitFor({state:'visible'});await until(()=>latest?.me.callName===newName,'saved identity missing');assert.equal(latest.me.username,username);assert.equal(latest.me.hotbar[2].count,8);assert.equal(latest.me.avatar.skin,4);
    await page.waitForFunction(()=>window.__smokeSocket?.readyState===WebSocket.OPEN);
    const previousConnections=connectionCount;
    await page.evaluate(()=>window.__smokeSocket.dispatchEvent(new MessageEvent('message',{data:JSON.stringify({type:'delta',seq:0,base:-1})})));
    await until(()=>connectionCount>previousConnections&&latest?.seq>=2,'out-of-sequence update did not reconnect with a fresh baseline');
    assert.equal(latest.me.hotbar[2].count,8);assert.equal(latest.me.callName,newName);assert.equal(await page.locator('#disconnect-banner').isVisible(),false);
    }
    if(process.argv.includes('--tour')||process.argv.includes('--tour-only')){
      await page.locator('#start-button').click();await page.waitForFunction(()=>!!document.pointerLockElement);
      async function travel(x,z){
        const start=Date.now();let previous=Date.now(),old={x:latest.me.x,z:latest.me.z};
        while(Math.hypot(x-latest.me.x,z-latest.me.z)>.8){
          assert.ok(Date.now()-start<45000,`walking route timed out at ${latest.me.x.toFixed(1)},${latest.me.z.toFixed(1)} toward ${x},${z}`);
          const dx=x-latest.me.x,dz=z-latest.me.z,d=Math.hypot(dx,dz);
          await page.evaluate(input=>window.__smokeInput=input,{x:dx/d,z:dz/d,sprint:false});await page.waitForTimeout(120);
          if(Date.now()-previous>4000){assert.ok(Math.hypot(latest.me.x-old.x,latest.me.z-old.z)>.2,`route blocked near ${latest.me.x.toFixed(1)},${latest.me.z.toFixed(1)} toward ${x},${z}`);old={x:latest.me.x,z:latest.me.z};previous=Date.now();}
        }
        await page.evaluate(()=>window.__smokeInput={x:0,z:0,sprint:false});await page.waitForTimeout(180);
      }
      async function face(x,z){const yaw=Math.atan2(-(x-latest.me.x),-(z-latest.me.z));await page.evaluate(({dx,dy})=>document.dispatchEvent(new MouseEvent('mousemove',{movementX:dx,movementY:dy})),{dx:(latest.me.yaw-yaw)/.002,dy:latest.me.pitch/.002});await page.waitForTimeout(250);}
      async function visit(key,file,pattern){await page.keyboard.press('e');await page.locator('#modal').waitFor({state:'visible'});assert.match(await page.locator('#modal-content').innerText(),pattern);await screenshot(file+'-shop');await resume();console.log('PASS: route and interaction with '+key);}
      for(const p of [[-35,-35],[-35,-9],[28,-9],[28,-20],[32,-20]])await travel(...p);
      await face(32,-30);await screenshot('desert');await travel(32,-27);await visit('desert barter','desert',/Village barter|berries/i);
      for(const p of [[32,-20],[28,-20],[28,-9],[8,-9],[8,34],[45,34]])await travel(...p);
      await face(45,20);await screenshot('city');await travel(45,23);await visit('city gym','city',/workout|Train/i);
      for(const p of [[45,34],[8,34],[8,-9],[-30,-9],[-30,10]])await travel(...p);
      await face(-80,65);await screenshot('beach');await face(-30,8);await visit('Peace','beach',/kilogram|trash/i);
      await page.evaluate(()=>window.__smokeInput=null);
    }
    assert.deepEqual(errors,[],'browser runtime errors');assert.deepEqual(failed,[],'failed asset/API requests');
    console.log(process.argv.includes('--tour-only')?'PASS: Chrome route tour; forest, desert barter, city gym, Peace beach; no browser errors.':'PASS: Chrome WebGL; curved authoritative world; 12 bakery plots; login; movement; throw/eat; 3/15/5 inventory; protected transfers; 5-slot hotbar; undiscovered cookbook; Escape-only menu; direct panel resume; five-tone avatar; earned hats; hourly call name; reload persistence; no browser errors.');
    console.log(`PASS: ${deltaCount} delta frames decoded; unchanged recipes and nodes retained${process.argv.includes('--tour-only')?'':'; fresh baseline after reconnect'}.`);
    console.log('Screenshots: '+output);
  }catch(error){await screenshot('failure').catch(()=>{});console.error('Browser diagnostics',{errors,failed,me:latest?.me&&{x:latest.me.x,z:latest.me.z,selectedSlot:latest.me.selectedSlot,callName:latest.me.callName},focus:await page.evaluate(()=>({active:document.activeElement?.id,locked:!!document.pointerLockElement,menu:!document.querySelector('#play-overlay').hidden,modal:!document.querySelector('#modal').hidden})).catch(()=>null)});throw error;}
  finally{await browser.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
