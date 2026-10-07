// Optional fixed-camera Chrome benchmark. Uses an isolated world and reports
// draw calls separately from software-renderer frame times. --unbatched runs
// the same scenery without batching for an apples-to-apples comparison.
const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path');
const {spawn,spawnSync}=require('node:child_process');
const root=path.resolve(__dirname,'..');
const {chromium}=require(require.resolve('playwright',{paths:[path.join(root,'.tools')]}));
(async()=>{
  const unbatched=process.argv.includes('--unbatched'),hardware=process.env.CK_RENDER_HARDWARE==='1';
  const out=path.join(root,'test-results','render-benchmark-'+Date.now()),data=path.join(out,'data');fs.mkdirSync(data,{recursive:true});
  const port=Number(process.env.CK_RENDER_TEST_PORT||8094),exe=path.join(out,process.platform==='win32'?'server.exe':'server'),base='http://127.0.0.1:'+port;
  assert.equal(await fetch(base+'/healthz').then(()=>true).catch(()=>false),false,port+' must be unused');
  const built=spawnSync('go',['build','-o',exe,'.'],{cwd:root,windowsHide:true,encoding:'utf8'});assert.equal(built.status,0,built.stderr);
  const server=spawn(exe,['-dev','-addr','127.0.0.1:'+port,'-data',data],{cwd:root,windowsHide:true,stdio:['ignore','pipe','pipe']});
  let log='',browser,page;const errors=[];server.stdout.on('data',x=>log+=x);server.stderr.on('data',x=>log+=x);
  async function until(test,message,timeout=30000){const start=Date.now();while(!await test()){assert.ok(Date.now()-start<timeout,message);await new Promise(r=>setTimeout(r,100));}}
  try{
    await until(()=>fetch(base+'/healthz').then(r=>r.ok).catch(()=>false),'server start');
    browser=await chromium.launch({executablePath:process.env.CK_CHROME||(process.platform==='win32'?'C:/Program Files/Google/Chrome/Application/chrome.exe':undefined),headless:true,args:hardware?[]:['--use-angle=swiftshader','--enable-unsafe-swiftshader','--enable-webgl']});
    page=await browser.newPage({viewport:{width:960,height:720}});page.on('pageerror',e=>errors.push(e.message));
    await page.route('**/game.js',async route=>{
      const response=await route.fetch(),original=await response.text();
      const sceneSource=unbatched?original.replace('sceneryBatches=createSceneryBatches(THREE,worldGroup,batchRoots);','sceneryBatches=null;'):original;
      if(unbatched)assert.notEqual(sceneSource,original,'batching hook changed');
      const changed=sceneSource.replace('renderer.render(scene,camera);',`camera.position.set(-35,1.7,-35);camera.rotation.set(0,0,0);window.__perfAccess={renderer,scene,worldGroup,camera,state}; const pstart=performance.now();renderer.render(scene,camera);if(window.__perfCollect){window.__perfSamples.push({at:performance.now(),cpu:performance.now()-pstart,calls:renderer.info.render.calls,triangles:renderer.info.render.triangles,geometries:renderer.info.memory.geometries});}`);
      assert.notEqual(changed,original);await route.fulfill({response,body:changed});
    });
    await page.goto(base,{waitUntil:'networkidle'});await page.locator('#email').fill('performance-review@example.com');await page.locator('#email-submit').click();await page.locator('#code-stage').waitFor({state:'visible'});
    if(!await page.locator('#login-code').inputValue()){const code=(await page.locator('#development-code').innerText()).match(/\b\d{6}\b/);assert.ok(code);await page.locator('#login-code').fill(code[0]);}
    await page.locator('#code-submit').click();await page.locator('#start-button').waitFor({state:'visible'});await page.locator('#start-button').click();await page.waitForFunction(()=>!!window.__perfAccess?.state.snapshot?.me);
    const graphics=await page.evaluate(()=>{const gl=window.__perfAccess.renderer.getContext(),info=gl.getExtension('WEBGL_debug_renderer_info');return info?gl.getParameter(info.UNMASKED_RENDERER_WEBGL):gl.getParameter(gl.RENDERER);});
    await page.waitForTimeout(4000);
    const stats=[];
    for(const variant of [unbatched?'unbatched':'batched']){
      await page.evaluate(()=>{
        window.__perfSamples=[];window.__perfCollect=true;
      });
      await page.waitForTimeout(7000);
      const result=await page.evaluate(()=>{window.__perfCollect=false;return {samples:window.__perfSamples,meshes:(()=>{let n=0;window.__perfAccess.scene.traverse(o=>{if(o.isMesh)n++;});return n;})(),lights:(()=>{const list=[];window.__perfAccess.scene.traverse(o=>{if(o.isLight)list.push({type:o.type,distance:o.distance,visible:o.visible});});return list;})()};});
      const sorted=key=>result.samples.map((s,i)=>key==='dt'?i?s.at-result.samples[i-1].at:0:s[key]).filter(n=>n>0).sort((a,b)=>a-b),mid=arr=>arr[Math.floor(arr.length*.5)],p95=arr=>arr[Math.floor(arr.length*.95)];
      assert.ok(result.samples.length>=10,'too few rendered frames');
      stats.push({variant,requestedRenderer:hardware?'hardware/default':'SwiftShader',graphics,frames:result.samples.length,meshes:result.meshes,lights:result.lights,renderCallMsMedian:mid(sorted('cpu')),renderCallMsP95:p95(sorted('cpu')),frameMsMedian:mid(sorted('dt')),frameMsP95:p95(sorted('dt')),calls:mid(sorted('calls')),triangles:mid(sorted('triangles')),geometries:mid(sorted('geometries'))});
      console.log(JSON.stringify(stats.at(-1)));
    }
    await page.screenshot({path:path.join(out,'forest.png')});assert.deepEqual(errors,[]);fs.writeFileSync(path.join(out,'metrics.json'),JSON.stringify(stats,null,2));console.log('Artifacts: '+out);
  }finally{if(browser)await browser.close();server.kill();await Promise.race([new Promise(r=>server.once('exit',r)),new Promise(r=>setTimeout(r,2500))]);fs.writeFileSync(path.join(out,'server.log'),log);}
})().catch(e=>{console.error(e);process.exitCode=1;});
