// Real Chrome integration for an interrupted delta stream and resume keyframe.
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
  const browser=await chromium.launch({executablePath,headless:true,args:process.env.CK_SMOKE_HARDWARE==='1'?[]:['--use-angle=swiftshader','--enable-unsafe-swiftshader','--enable-webgl']});
  const context=await browser.newContext({viewport:{width:1280,height:900}});
  const page=await context.newPage();
  await page.addInitScript(()=>{const send=WebSocket.prototype.send;WebSocket.prototype.send=function(payload){window.__reconnectSocket=this;return send.call(this,payload);};});

  const errors=[],failed=[],resumeIDs=[];
  let latest,activeSocket,connections=0;
  let before;
  page.on('pageerror',error=>errors.push(error.message));
  page.on('response',response=>{if(response.status()>=400)failed.push(`${response.status()} ${response.url()}`);});
  page.on('websocket',socket=>{
    activeSocket=socket;latest=undefined;connections++;
    resumeIDs.push(new URL(socket.url()).searchParams.get('resume')||'');
    const decoder=createSnapshotDecoder();
    socket.on('framereceived',({payload})=>{
      if(socket!==activeSocket)return;
      try{latest=decoder.apply(JSON.parse(payload.toString()))||latest;}catch(error){errors.push('WebSocket decoding: '+error.message);}
    });
  });

  async function until(predicate,label,timeout=7000){
    const started=Date.now();
    while(!await predicate()){
      assert.ok(Date.now()-started<timeout,label);
      await page.waitForTimeout(50);
    }
  }

  try{
    await page.goto(baseURL,{waitUntil:'networkidle'});
    await page.locator('#email').fill(`reconnect${Date.now()}@example.com`);
    await page.locator('#email-submit').click();
    await page.locator('#code-stage').waitFor({state:'visible'});
    if(!await page.locator('#login-code').inputValue()){
      const code=(await page.locator('#development-code').innerText()).match(/\b\d{6}\b/);
      assert.ok(code,'development login code missing');
      await page.locator('#login-code').fill(code[0]);
    }
    await page.locator('#code-submit').click();
    await page.locator('#start-button').waitFor({state:'visible'});
    await until(()=>!!latest?.me,'missing initial keyframe');

    await page.locator('#start-button').click();
    await page.waitForFunction(()=>!!document.pointerLockElement);
    const firstPosition={x:latest.me.x,z:latest.me.z};
    for(const key of ['w','a','s','d']){
      await page.keyboard.down(key);
      await page.waitForTimeout(350);
      await page.keyboard.up(key);
      if(Math.hypot(latest.me.x-firstPosition.x,latest.me.z-firstPosition.z)>.01)break;
    }
    assert.ok(Math.hypot(latest.me.x-firstPosition.x,latest.me.z-firstPosition.z)>.01,'server did not accept pre-interruption movement');
    await page.waitForTimeout(250);
    before={id:latest.me.id,x:latest.me.x,z:latest.me.z,yaw:latest.me.yaw,pitch:latest.me.pitch};
    const previousConnections=connections;

    await page.evaluate(()=>window.__reconnectSocket.dispatchEvent(new MessageEvent('message',{data:JSON.stringify({type:'delta',seq:0,base:-1})})));
    await until(()=>connections>previousConnections&&latest?.seq>=1,'client did not request a resume keyframe');
    await until(async()=>!(await page.locator('#disconnect-banner').isVisible()),'reconnect banner did not clear');

    assert.equal(latest.me.id,before.id,'reconnect changed the player');
    assert.ok(Math.hypot(latest.me.x-before.x,latest.me.z-before.z)<.5,'reconnect reset player position');
    assert.equal(latest.me.yaw,before.yaw,'reconnect reset camera yaw');
    assert.equal(latest.me.pitch,before.pitch,'reconnect reset camera pitch');
    assert.ok(resumeIDs.length>=2&&resumeIDs[0]&&resumeIDs[0]===resumeIDs[1],'browser did not reuse one tab resume identifier');
    assert.equal(await page.locator('#landing').isVisible(),false,'reconnect reopened the login page');
    assert.equal(await page.locator('#hud').isVisible(),true,'reconnect reset the in-game UI');
    assert.deepEqual(errors,[],'browser runtime errors');
    assert.deepEqual(failed,[],'failed asset/API requests');
    console.log('PASS: forced delta interruption reconnects with a full keyframe and resumes player, position and camera without reloading login.');
  }catch(error){
    console.error('Reconnect browser diagnostics',{errors,failed,connections,resumeIDs,before,me:latest?.me&&{id:latest.me.id,x:latest.me.x,z:latest.me.z,yaw:latest.me.yaw,pitch:latest.me.pitch}});
    throw error;
  }finally{
    await browser.close();
  }
})().catch(error=>{console.error(error);process.exitCode=1;});
