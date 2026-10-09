// Chrome gameplay with isolated saves and initial wildlife/position fixtures only.
const assert=require('node:assert/strict'), fs=require('node:fs'), path=require('node:path');
const crypto=require('node:crypto'), net=require('node:net'), {spawn,spawnSync}=require('node:child_process');
const {chromium}=require(require.resolve('playwright',{paths:[path.resolve(__dirname,'../.tools')]}));

(async()=>{
  const root=path.resolve(__dirname,'..'), out=path.join(root,'test-results','rodents-'+Date.now()), data=path.join(out,'data');
  fs.mkdirSync(data,{recursive:true});
  const account=email=>({email,id:'u_'+crypto.createHash('sha256').update(email).digest('hex').slice(0,32)});
  const correct=account('rodents-correct@example.com'), wrong=account('rodents-rainbow@example.com');
  const slots=n=>Array.from({length:n},()=>({item:'',count:0}));
  const profiles={};
  for(const [index,owner] of [correct,wrong].entries()){
    const p={id:owner.id,name:index?'Rainbow Pal':'Wildlife Pal',callName:index?'Rainbow Pal':'Wildlife Pal',username:'wildlife_'+index,
      coins:400,inventoryVersion:3,hotbar:slots(5),bagSlots:slots(20),selectedSlot:0,levels:{},discovered:{sugar:true,nut_cookie:true},
      nutIsland:{owner:owner.id,nextId:0,pets:[],cages:[]}};
    p.hotbar[0]={item:'funny_cake',count:8};p.hotbar[1]={item:'nut_cookie',count:8};p.hotbar[2]={item:'cosmic_cake',count:8};
    profiles[owner.id]=p;
  }
  fs.writeFileSync(path.join(data,'players.json'),JSON.stringify({version:3,profiles}));
  const worldFile=path.join(root,'internal/game/world.go'),source=fs.readFileSync(worldFile,'utf8');
  const anchor=/(\tw\.spawn\(p\)\r?\n)(\tw\.players\[id\] = p)/;
  assert.match(source,anchor,'initial wildlife fixture anchor');
  const fixture=`$1\tp.x, p.y, p.z, p.yaw = -236.5, 0, -256.5, 1.57\n$2
\tif id == "${correct.id}" || id == "${wrong.id}" {
\t\tif r := w.spawnRodent("otter", "", -239.3, -256.5); r == nil { panic("otter initial fixture unavailable") }
\t}
\tif id == "${correct.id}" { if r := w.spawnRodent("wood_mouse", "", -239.3, -259.3); r == nil { panic("mouse initial fixture unavailable") } }
`;
  const changed=source.replace('\tw.seedMap()','\tw.time = 730\n\tw.seedMap()').replace(anchor,fixture);
  const replacement=path.join(out,'world.go.txt'),overlay=path.join(out,'overlay.json'),exe=path.join(out,process.platform==='win32'?'server.exe':'server');
  fs.writeFileSync(replacement,changed);fs.writeFileSync(overlay,JSON.stringify({Replace:{[worldFile]:replacement}}));
  const built=spawnSync(process.env.CK_GO||(process.platform==='win32'?'C:/Program Files/Go/bin/go.exe':'go'),['build','-overlay',overlay,'-o',exe,'.'],{cwd:root,windowsHide:true,encoding:'utf8'});
  assert.equal(built.status,0,built.stderr||built.stdout);
  const port=await new Promise((resolve,reject)=>{const s=net.createServer();s.once('error',reject);s.listen(0,'127.0.0.1',()=>{const port=s.address().port;s.close(()=>resolve(port));});});
  const base='http://127.0.0.1:'+port;
  let browser,log='';const clients=[],errors=[],httpErrors=[],actions=[];
  const server=spawn(exe,['-dev','-addr','127.0.0.1:'+port,'-data',data],{cwd:root,windowsHide:true,stdio:['ignore','pipe','pipe']});
  server.stdout.on('data',c=>log+=c);server.stderr.on('data',c=>log+=c);
  const {createSnapshotDecoder}=await import('data:text/javascript;base64,'+Buffer.from(fs.readFileSync(path.join(root,'web/state-sync.js'),'utf8')).toString('base64'));
  async function until(check,label,timeout=20000){const started=Date.now();while(!await check()){assert.ok(Date.now()-started<timeout,label+'; '+errors.join('; '));await new Promise(r=>setTimeout(r,70));}}
  async function resume(c){
    const p=c.page;await p.bringToFront();
    if(await p.locator('#modal').isVisible())await p.locator('#close-modal').click();
    if(await p.locator('#play-overlay').isVisible())await p.locator('#play-button').click();
    if(await p.locator('#capture-prompt').isVisible())await p.locator('#capture-prompt').click();
    await p.waitForFunction(()=>!!document.pointerLockElement&&window.__wildDebug().locked);
  }
  async function face(c,x,y,z){
    const me=c.snapshot.me,yaw=Math.atan2(-(x-me.x),-(z-me.z));
    const pitch=Math.atan2(y-(me.y+1.45),Math.hypot(x-me.x,z-me.z));
    await c.page.evaluate(({yaw,pitch})=>{const view=window.__wildDebug(),event=new MouseEvent('mousemove');
      Object.defineProperty(event,'movementX',{value:(view.yaw-yaw)/.002});Object.defineProperty(event,'movementY',{value:(view.pitch-pitch)/.002});document.dispatchEvent(event);},{yaw,pitch});
    await c.page.waitForTimeout(150);
  }
  const species=(c,id)=>c.snapshot.rodents.find(r=>r.species===id&&Math.hypot(r.x-c.snapshot.me.x,r.z-c.snapshot.me.z)<20);
  const screenshot=(c,name)=>c.page.screenshot({path:path.join(out,name+'.png')});
  async function fire(c,slot,target){
    await resume(c);await c.page.keyboard.press(String(slot));
    await until(()=>c.snapshot.me.selectedSlot===slot-1,'real hotbar selection');
    const r=typeof target==='string'?species(c,target):target;assert.ok(r,'live projectile target exists');
    await face(c,r.x,r.y+.3,r.z);
    await c.page.mouse.down();await c.page.mouse.up();
    await c.page.waitForTimeout(650);
  }
  async function login(owner){
    const context=await browser.newContext({viewport:{width:1440,height:1000}}),page=await context.newPage(),c={context,page,snapshot:null,history:[],owner};clients.push(c);
    await page.route('**/game.js',async route=>{const response=await route.fetch(),source=await response.text();
      const anchor='function receiveSnapshot(snapshot){';assert.ok(source.includes(anchor));
      const debug='window.__wildDebug=()=>({locked:state.locked,yaw:state.yaw,pitch:state.pitch});';
      await route.fulfill({response,body:source.replace(anchor,debug+anchor).replace('renderer.render(scene,camera);','window.__wildFrames=(window.__wildFrames||0)+1;if(window.__wildFrames%3===0)renderer.render(scene,camera);')});
    });
    page.on('pageerror',e=>errors.push(e.message));page.on('response',r=>{if(r.status()>=400)httpErrors.push(r.status()+' '+r.url());});
    page.on('websocket',socket=>{const decoder=createSnapshotDecoder();
      socket.on('framesent',({payload})=>{const m=JSON.parse(payload.toString());if(m.type==='action')actions.push({...m,account:owner.id});});
      socket.on('framereceived',({payload})=>{try{const m=JSON.parse(payload.toString()),next=decoder.apply(m);if(next){c.snapshot=next;c.history.push({time:next.time,rodents:next.rodents,world:next.rodentWorld,coins:next.me.coins,inventory:next.me.inventory});}if(m.type==='error')errors.push(m.message);}catch(e){errors.push(e.message);}});
    });
    await page.goto(base,{waitUntil:'networkidle'});await page.locator('#email').fill(owner.email);await page.locator('#email-submit').click();
    await page.locator('#code-stage').waitFor({state:'visible'});
    if(!await page.locator('#login-code').inputValue())await page.locator('#login-code').fill((await page.locator('#development-code').innerText()).match(/\b\d{6}\b/)[0]);
    await page.locator('#code-submit').click();await page.locator('#start-button').waitFor({state:'visible'});await until(()=>c.snapshot?.me,'player snapshot');
    await page.locator('#start-button').click();await resume(c);return c;
  }
  try{
    await until(()=>{if(server.exitCode!==null)throw Error(log);return fetch(base+'/healthz').then(r=>r.ok).catch(()=>false);},'server start');
    browser=await chromium.launch({headless:true,executablePath:process.env.CK_CHROME||(process.platform==='win32'?'C:/Program Files/Google/Chrome/Application/chrome.exe':undefined),args:['--use-angle=swiftshader','--enable-unsafe-swiftshader','--enable-webgl']});
    const c=await login(correct);
    assert.equal(c.snapshot.me.coins,400);assert.ok(species(c,'wood_mouse'));assert.ok(species(c,'otter'));
    await fire(c,1,'wood_mouse');
    await until(()=>species(c,'wood_mouse')?.state==='confused','funny cake projectile confuses mouse');
    assert.equal(c.snapshot.me.inventory.funny_cake,7);
    await screenshot(c,'funny-cake-confused-mouse');
    await until(()=>c.page.locator('#rodent-capture-hud').innerText().then(t=>/Wood mouse/.test(t)),'H prompt identifies catchable mouse');
    await c.page.keyboard.press('h');await until(()=>c.snapshot.me.inventory.pet_wood_mouse===1,'real H captures one mouse into bag');
    await c.page.keyboard.press('i');await c.page.getByRole('button',{name:/bag slot \d+: Wood mouse/}).click();
    assert.match(await c.page.locator('#modal-content').innerText(),/Wood mouse/);await screenshot(c,'captured-mouse-in-inventory');
    await resume(c);
    await fire(c,2,'otter');await until(()=>species(c,'otter')?.nutFed,'real nut cookie feeds otter first');
    await screenshot(c,'otter-nut-first');
    await fire(c,3,'otter');await until(()=>species(c,'otter')?.state==='confused','cosmic cake completes ordered otter sequence');
    assert.equal(c.snapshot.me.inventory.nut_cookie,7);assert.equal(c.snapshot.me.inventory.cosmic_cake,7);
    await screenshot(c,'otter-cosmic-confusion');
    await c.page.keyboard.press('h');await until(()=>c.snapshot.me.inventory.pet_otter===1,'real H captures prepared otter');
    assert.equal(c.snapshot.me.coins,400,'non-damaging wildlife hits and captures never farm coins');
    assert.equal(c.snapshot.me.coinsEarned||0,0);assert.equal(c.snapshot.me.kills||0,0);
    await c.page.keyboard.press('i');await c.page.getByRole('button',{name:/bag slot \d+: Otter/}).click();await screenshot(c,'captured-mythic-otter-in-inventory');
    await c.page.getByRole('button',{name:'Go to my nut island',exact:true}).first().click();await until(()=>c.snapshot.me.island==='nut:'+correct.id,'real UI enters a different world');await resume(c);
    console.log('PASS: real funny cake projectile, H mouse capture, nut-then-cosmic otter capture, inventory slots, and unchanged wallet.');
    const wrongClient=await login(wrong);
    const initial=species(wrongClient,'otter');assert.ok(initial&&!initial.nutFed);
    await fire(wrongClient,3,'otter');await until(()=>species(wrongClient,'otter')?.state==='rainbow','cosmic cake before nut sends otter skyward');
    await until(()=>wrongClient.snapshot.rodentWorld.rainbowUntil>wrongClient.snapshot.time&&c.snapshot.rodentWorld.rainbowUntil===wrongClient.snapshot.rodentWorld.rainbowUntil,'rainbow broadcasts across mainland and private nut island');
    await until(()=>wrongClient.page.locator('#otter-rainbow').evaluate(e=>getComputedStyle(e).display!=='none'&&Number(getComputedStyle(e).opacity)>.1),'visible global rainbow overlay');
    await screenshot(wrongClient,'wrong-order-otter-rainbow');
    await resume(c);await until(()=>c.page.locator('#otter-rainbow').evaluate(e=>getComputedStyle(e).display!=='none'&&Number(getComputedStyle(e).opacity)>.1),'same rainbow visible on nut island');
    await screenshot(c,'rainbow-on-another-island');
    assert.equal(wrongClient.snapshot.me.inventory.cosmic_cake,7);assert.equal(wrongClient.snapshot.me.inventory.pet_otter||0,0);assert.equal(wrongClient.snapshot.me.coins,400);
    assert.ok(wrongClient.history.some(s=>s.rodents.some(r=>r.species==='otter'&&r.state==='rainbow'&&r.y>1)),'wrong-order otter physically ascends');
    assert.equal(c.snapshot.me.inventory.pet_otter,1);assert.equal(c.snapshot.me.inventory.pet_wood_mouse,1);
    assert.deepEqual(errors,[]);assert.deepEqual(httpErrors,[]);
    console.log('PASS: wrong-order cosmic hit causes skyward otter and global rainbow visible across worlds, with no economic rewards.');console.log('Screenshots: '+out);
  }catch(error){
    for(const [i,c] of clients.entries()){await screenshot(c,'failure-'+i).catch(()=>{});fs.writeFileSync(path.join(out,'failure-'+i+'.json'),JSON.stringify({snapshot:c.snapshot,errors,actions:actions.slice(-15)},null,2));}
    throw error;
  }finally{
    await browser?.close();server.kill();if(server.exitCode===null)await Promise.race([new Promise(r=>server.once('exit',r)),new Promise(r=>setTimeout(r,3000))]);
    fs.writeFileSync(path.join(out,'server.log'),log);fs.writeFileSync(path.join(out,'actions.json'),JSON.stringify(actions,null,2));
    clients.forEach((c,i)=>fs.writeFileSync(path.join(out,'history-'+i+'.json'),JSON.stringify(c.history)));
  }
})().catch(error=>{console.error(error);process.exitCode=1;});
