// Real Chrome and Go integration checks. Build overlays shorten only this
// isolated server's day/night clock and effect timers and spawn by the vendor.
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const crypto=require('node:crypto');
const net=require('node:net');
const {spawn,spawnSync}=require('node:child_process');
const {chromium}=require(require.resolve('playwright',{paths:[path.resolve(__dirname,'../.tools')]}));

(async()=>{
  const root=path.resolve(__dirname,'..');
  const out=path.join(root,'test-results','funny-cake-browser-'+Date.now()),data=path.join(out,'data');
  fs.mkdirSync(data,{recursive:true});
  const email='disco-cakes@example.com',id='u_'+crypto.createHash('sha256').update(email).digest('hex').slice(0,32);
  const slots=n=>Array.from({length:n},()=>({item:'',count:0}));
  fs.writeFileSync(path.join(data,'players.json'),JSON.stringify({version:3,profiles:{[id]:{
    id,name:'Cake Explorer',callName:'Cake Explorer',username:'cake_explorer',coins:1000,
    inventoryVersion:3,hotbar:slots(5),bagSlots:slots(20),selectedSlot:0,levels:{}
  }}}));

  const replacements={};
  function overlay(relative,transform){
    const file=path.join(root,relative),source=fs.readFileSync(file,'utf8'),changed=transform(source);
    assert.notEqual(changed,source,'overlay anchor missing in '+relative);
    const target=path.join(out,path.basename(file)+'.txt');fs.writeFileSync(target,changed);replacements[file]=target;
  }
  overlay('internal/game/world.go',source=>source.replace(/(\tw\.spawn\(p\)\r?\n)(\tw\.players\[id\] = p)/,
    '$1\tp.x, p.y, p.z, p.inCave = -189, -12, -241.5, true\n$2'));
  overlay('internal/game/john_pork.go',source=>{
    assert.match(source,/const DayDuration = 12 \* 60\.0/);assert.match(source,/const NightDuration = 6 \* 60\.0/);
    return source.replace('const DayDuration = 12 * 60.0','const DayDuration = 20.0')
      .replace('const NightDuration = 6 * 60.0','const NightDuration = 45.0');
  });
  overlay('internal/game/club_cakes.go',source=>source.replace('p.buffs["funny_cake"] = w.time + cake.Duration',
    'p.buffs["funny_cake"] = w.time + cake.Duration/8'));
  const overlayFile=path.join(out,'overlay.json');fs.writeFileSync(overlayFile,JSON.stringify({Replace:replacements}));
  const executable=path.join(out,process.platform==='win32'?'server.exe':'server');
  const go=process.env.CK_GO||(process.platform==='win32'?'C:/Program Files/Go/bin/go.exe':'go');
  const build=spawnSync(go,['build','-overlay',overlayFile,'-o',executable,'.'],{cwd:root,windowsHide:true,encoding:'utf8'});
  assert.equal(build.status,0,build.stderr||build.stdout);
  const port=await new Promise((resolve,reject)=>{
    const probe=net.createServer();probe.once('error',reject);probe.listen(0,'127.0.0.1',()=>{
      const selected=probe.address().port;probe.close(error=>error?reject(error):resolve(selected));
    });
  });
  const base='http://127.0.0.1:'+port;
  let browser,page,snapshot,activeSocket,log='';
  const errors=[],failed=[],actionErrors=[],actions=[],history=[];
  const server=spawn(executable,['-dev','-addr','127.0.0.1:'+port,'-data',data],{cwd:root,windowsHide:true,stdio:['ignore','pipe','pipe']});
  server.stdout.on('data',chunk=>log+=chunk);server.stderr.on('data',chunk=>log+=chunk);
  const {createSnapshotDecoder}=await import('data:text/javascript;base64,'+Buffer.from(fs.readFileSync(path.join(root,'web/state-sync.js'),'utf8')).toString('base64'));
  async function until(check,label,timeout=20000){
    const start=Date.now();while(!await check()){
      assert.ok(Date.now()-start<timeout,label+'; errors: '+[...errors,...actionErrors].join('; '));
      await new Promise(resolve=>setTimeout(resolve,75));
    }
  }
  const shot=name=>page.screenshot({path:path.join(out,name+'.png')});
  const seller=()=>snapshot?.nodes?.find(node=>node.id==='club_cake_seller');
  const remaining=()=>snapshot?.me?.buffs?.funny_cake||0;
  const effects=()=>page.evaluate(()=>{const s=document.querySelector('#world').style;return {filter:s.filter,transform:s.transform};});
  async function resume(){
    if(await page.locator('#modal').isVisible())await page.locator('#close-modal').click();
    if(await page.locator('#capture-prompt').isVisible())await page.locator('#capture-prompt').click();
    await page.waitForFunction(()=>!!document.pointerLockElement&&window.__cakeDebug().locked);
  }
  async function inventory(){await page.keyboard.press('i');await page.locator('#modal').waitFor({state:'visible'});}
  async function pickBag(item){
    const index=snapshot.me.bagSlots.findIndex(stack=>stack.item===item&&stack.count>0);assert.ok(index>=0,item+' in bag');
    await page.locator('.storage-bag .storage-slot').nth(index).click();
  }
  async function faceSeller(){
    await page.waitForFunction(()=>!!document.pointerLockElement&&window.__cakeDebug().locked);
    const yaw=Math.atan2(-(-191-snapshot.me.x),-(-245-snapshot.me.z));
    await page.evaluate(yaw=>{const d=window.__cakeDebug(),event=new MouseEvent('mousemove');Object.defineProperty(event,'movementX',{value:(d.yaw-yaw)/.002});Object.defineProperty(event,'movementY',{value:(d.pitch+.07)/.002});document.dispatchEvent(event);},yaw);
    await page.waitForTimeout(150);
  }
  try{
    await until(()=>fetch(base+'/healthz').then(response=>response.ok).catch(()=>false),'server start');
    browser=await chromium.launch({executablePath:process.env.CK_CHROME||(process.platform==='win32'?'C:/Program Files/Google/Chrome/Application/chrome.exe':undefined),headless:true,args:['--use-angle=swiftshader','--enable-unsafe-swiftshader','--enable-webgl']});
    page=await browser.newPage({viewport:{width:1440,height:1000}});
    await page.addInitScript(()=>{const send=WebSocket.prototype.send;WebSocket.prototype.send=function(payload){window.__cakeSocket=this;return send.call(this,payload);};});
    await page.route('**/game.js',async route=>{
      const response=await route.fetch(),source=await response.text();
      assert.ok(source.includes('function receiveSnapshot(snapshot){'),'read-only inspection anchor');
      await route.fulfill({response,body:source.replace('function receiveSnapshot(snapshot){',
        "window.__cakeDebug=()=>{const seller=scene.getObjectByName('club-cake-seller');let visible=!!seller;for(let o=seller;o;o=o.parent)visible=visible&&o.visible;return {yaw:state.yaw,pitch:state.pitch,connected:state.connected,locked:state.locked,selected:state.selected,sellerExists:!!seller,sellerVisible:visible};};function receiveSnapshot(snapshot){")
        .replace('renderer.render(scene,camera);','window.__cakeRender=(window.__cakeRender||0)+1;if(window.__cakeRender%3===0)renderer.render(scene,camera);')});
    });
    page.on('pageerror',error=>errors.push(error.message));
    page.on('response',response=>{if(response.status()>=400)failed.push(response.status()+' '+response.url());});
    page.on('websocket',socket=>{
      activeSocket=socket;const decoder=createSnapshotDecoder();
      socket.on('framesent',({payload})=>{const message=JSON.parse(payload.toString());if(message.type==='action')actions.push(message);});
      socket.on('framereceived',({payload})=>{
        if(socket!==activeSocket)return;
        try{const message=JSON.parse(payload.toString()),next=decoder.apply(message);
          if(next){snapshot=next;history.push({time:next.time,phase:next.dayCycle?.phase,coins:next.me.coins,inventory:next.me.inventory,buff:next.me.buffs?.funny_cake||0,seller:seller()?.available});}
          if(message.type==='error')actionErrors.push(message.message);
        }catch(error){errors.push(error.message);}
      });
    });
    await page.goto(base,{waitUntil:'networkidle'});
    await page.locator('#email').fill(email);await page.locator('#email-submit').click();
    await page.locator('#code-stage').waitFor({state:'visible'});
    if(!await page.locator('#login-code').inputValue())await page.locator('#login-code').fill((await page.locator('#development-code').innerText()).match(/\b\d{6}\b/)[0]);
    await page.locator('#code-submit').click();await page.locator('#start-button').waitFor({state:'visible'});
    await until(()=>snapshot?.me,'initial snapshot');await page.locator('#start-button').click();await page.waitForFunction(()=>!!document.pointerLockElement);
    assert.deepEqual(snapshot.clubCakes.map(({id,price,duration})=>({id,price,duration})),[
      {id:'funny_cake',price:15,duration:45},{id:'cosmic_cake',price:25,duration:75}
    ]);
    assert.equal(snapshot.me.inCave,true);assert.equal(snapshot.me.y,-12);assert.equal(snapshot.dayCycle.phase,'day');
    assert.equal(seller()?.available,false);await faceSeller();
    assert.equal((await page.evaluate(()=>window.__cakeDebug())).sellerVisible,false);await shot('daytime-empty-cake-corner');
    await until(()=>snapshot.dayCycle.phase==='night'&&seller()?.available,'seller opens at night',25000);
    await until(()=>page.evaluate(()=>window.__cakeDebug().sellerVisible),'nighttime seller visible');
    await page.waitForFunction(()=>/cake/i.test(document.getElementById('interaction-label').textContent));
    await shot('nighttime-disco-duke');await page.keyboard.press('e');await page.locator('#modal').waitFor({state:'visible'});
    assert.match(await page.locator('#modal-content').innerText(),/Disco Duke/);
    async function buy(item){
      const cake=snapshot.clubCakes.find(cake=>cake.id===item),before=snapshot.me.inventory[item]||0;
      const row=page.locator('.shop-row').filter({has:page.getByRole('heading',{name:cake.name,exact:true})});
      await row.getByRole('button',{name:/Buy/}).click();await until(()=>snapshot.me.inventory[item]===before+1,'buy '+item);
    }
    await buy('funny_cake');await buy('funny_cake');await buy('cosmic_cake');assert.equal(snapshot.me.coins,945);
    await shot('funny-cake-menu');await resume();await inventory();await pickBag('funny_cake');
    await page.getByRole('button',{name:'Eat one',exact:true}).click();
    await until(()=>remaining()>0&&snapshot.me.inventory.funny_cake===1,'consume cake from inventory');
    await resume();await page.waitForTimeout(1100);
    const initial=await effects();assert.match(initial.filter,/hue-rotate/);assert.match(initial.transform,/translate/);
    const position={x:snapshot.me.x,z:snapshot.me.z,yaw:snapshot.me.yaw,pitch:snapshot.me.pitch};
    await page.waitForTimeout(250);assert.notDeepEqual(await effects(),initial,'colors and movement should evolve');
    assert.deepEqual({x:snapshot.me.x,z:snapshot.me.z,yaw:snapshot.me.yaw,pitch:snapshot.me.pitch},position,'visual effect must not move or aim the player');
    await shot('funny-cake-colors-and-sway');
    await page.keyboard.press('l');await until(async()=>!(await effects()).transform,'reduced flashing stops sway');
    assert.match(await page.locator('#club-light-toggle').innerText(),/reduced/);
    await shot('reduced-flashing-cake');await page.keyboard.press('l');await until(async()=>!!(await effects()).transform,'sway resumes');
    await inventory();await pickBag('cosmic_cake');await page.locator('.storage-hotbar .storage-slot').first().click();
    await until(()=>snapshot.me.hotbar[0].item==='cosmic_cake','move cake to hotbar');
    await resume();await page.keyboard.press('1');
    await page.waitForFunction(()=>window.__cakeDebug().selected==='cosmic_cake'&&window.__cakeDebug().locked);
    const oldRemaining=remaining();await page.keyboard.press('q');
    await until(()=>!snapshot.me.inventory.cosmic_cake&&remaining()>7,'consume cosmic cake through hotbar');
    assert.ok(remaining()>oldRemaining,'another cake refreshes the effect');
    await until(()=>remaining()===0,'cake effect expires',13000);
    await until(async()=>{const style=await effects();return !style.filter&&!style.transform;},'expiry restores the canvas');
    await shot('cake-effect-expired');
    await faceSeller();await page.keyboard.press('e');await page.locator('#modal').waitFor({state:'visible'});
    await until(()=>snapshot.dayCycle.phase==='day','dawn closes shop',50000);
    assert.equal(seller()?.available,false);await until(()=>page.evaluate(()=>!window.__cakeDebug().sellerVisible),'dawn hides seller');
    await until(()=>page.locator('#modal-content').innerText().then(text=>/Come back after sunset/.test(text)),'open menu closes at dawn');
    const buyButtons=page.locator('.shop-row button[data-action="buy_club_cake"]');assert.equal(await buyButtons.count(),2);
    for(const button of await buyButtons.all())assert.equal(await button.isDisabled(),true);
    const balance=snapshot.me.coins,count=snapshot.me.inventory.funny_cake;
    await page.evaluate(()=>window.__cakeSocket.send(JSON.stringify({type:'action',action:'buy_club_cake',target:'club_cake_seller',item:'funny_cake',amount:1})));
    await until(()=>actionErrors.length===1,'server rejects stale daylight purchase');
    assert.match(actionErrors[0],/night|closed|sunset/i);assert.equal(snapshot.me.coins,balance);assert.equal(snapshot.me.inventory.funny_cake,count);
    await shot('dawn-closes-cake-menu');await resume();await inventory();await pickBag('funny_cake');
    await page.getByRole('button',{name:'Eat one',exact:true}).click();await until(()=>remaining()>0,'retained cake can be eaten after dawn');
    await resume();await until(async()=>!!(await effects()).transform,'final cake effect active');
    await page.evaluate(()=>window.__cakeSocket.close());
    await until(()=>page.evaluate(()=>!window.__cakeDebug().connected&&!document.querySelector('#world').style.filter&&!document.querySelector('#world').style.transform),'disconnect resets effect',2000);
    assert.deepEqual(errors,[],'browser errors');assert.deepEqual(failed,[],'failed HTTP requests');
    assert.equal(actionErrors.length,1,'only the intentional daylight denial');
    assert.ok(actions.some(action=>action.action==='eat'&&action.item==='funny_cake'));
    assert.ok(actions.some(action=>action.action==='eat'&&action.item==='cosmic_cake'));
    console.log('PASS: nighttime seller; real coin purchases; inventory and hotbar eating; moving colors and sway; reduced flashing; refresh, expiry and disconnect cleanup; dawn closes open menu and server denies purchases.');
    console.log('Screenshots: '+out);
  }catch(error){if(page)await shot('failure').catch(()=>{});console.error({errors,actionErrors,failed,me:snapshot?.me});throw error;}
  finally{
    await browser?.close();server.kill();
    if(server.exitCode===null)await Promise.race([new Promise(resolve=>server.once('exit',resolve)),new Promise(resolve=>setTimeout(resolve,3000))]);
    fs.writeFileSync(path.join(out,'server.log'),log);fs.writeFileSync(path.join(out,'actions.json'),JSON.stringify(actions,null,2));
    fs.writeFileSync(path.join(out,'history.json'),JSON.stringify(history));
  }
})().catch(error=>{console.error(error);process.exitCode=1;});
