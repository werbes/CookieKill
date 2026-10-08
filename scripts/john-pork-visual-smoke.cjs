// Real WebGL asset/render smoke; screenshots make the shirt and sleeping pose reviewable.
const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),http=require('node:http');
const {chromium}=require(require.resolve('playwright',{paths:[path.resolve(__dirname,'../.tools')]}));
(async()=>{
  const root=path.resolve(__dirname,'..'),web=path.join(root,'web'),out=path.join(root,'test-results','john-pork-visual-'+Date.now());fs.mkdirSync(out,{recursive:true});
  const fixture=`<!doctype html><html><body style="margin:0"><script type="module">
    import * as THREE from '/vendor/three.module.js';
    import {createNightScene} from '/night-scene.js';
    import {buildJohnHut} from '/john-hut-scenery.js';
    const scene=new THREE.Scene();scene.background=new THREE.Color('#c2c3af');
    const camera=new THREE.PerspectiveCamera(50,innerWidth/innerHeight,.1,800),renderer=new THREE.WebGLRenderer({antialias:true});renderer.setSize(innerWidth,innerHeight);document.body.append(renderer.domElement);
    scene.add(new THREE.HemisphereLight('#fff1dc','#968674',2));const sun=new THREE.DirectionalLight('#fff0d8',2);sun.position.set(-3,8,-4);scene.add(sun);
    const primitive=(geometry,color,x,y,z,parent)=>{const mesh=new THREE.Mesh(geometry,new THREE.MeshStandardMaterial({color}));mesh.position.set(x,y,z);parent.add(mesh);return mesh;};
    const floor=primitive(new THREE.PlaneGeometry(1000,1000),'#cebb98',0,-.06,0,scene);floor.rotation.x=-Math.PI/2;
    const label=(text,color,background)=>{const canvas=document.createElement('canvas');canvas.width=512;canvas.height=100;const ctx=canvas.getContext('2d');ctx.fillStyle=background;ctx.fillRect(0,0,512,100);ctx.font='bold 34px Arial';ctx.fillStyle=color;ctx.textAlign='center';ctx.fillText(text,256,62,490);const texture=new THREE.CanvasTexture(canvas);texture.colorSpace=THREE.SRGBColorSpace;return new THREE.Mesh(new THREE.PlaneGeometry(1,1),new THREE.MeshBasicMaterial({map:texture,side:THREE.DoubleSide}));};
    const hut=buildJohnHut({THREE,parent:scene,flatLabel:label}),audio={unlock(){},setMuted(){},setRange(d,enabled){window.ringing=enabled;},update(){},stop(){},dispose(){}};
    const john=createNightScene({THREE,parent:scene,audio,document:null,box:(w,h,d,c,x,y,z,p)=>primitive(new THREE.BoxGeometry(w,h,d),c,x,y,z,p),sphere:(r,c,x,y,z,p)=>primitive(new THREE.IcosahedronGeometry(r,2),c,x,y,z,p),cylinder:(a,b,h,c,x,y,z,p)=>primitive(new THREE.CylinderGeometry(a,b,h,8),c,x,y,z,p)});
    window.view=mode=>{const sleeping=mode!=='shirt';john.sync({dayCycle:{phase:sleeping?'day':'night'},me:{id:'me',x:sleeping?222:0,z:sleeping?-236:-3},johnPork:sleeping?{x:220.2,y:1,z:-238.8,yaw:Math.PI,sleeping:true}:{x:0,y:0,z:0,yaw:0,target:'me'}});john.update(1,.05);if(mode==='shirt'){camera.position.set(0,1.35,-3.2);camera.lookAt(0,1.22,0);}else if(mode==='bed'){camera.position.set(223.2,2.5,-236);camera.lookAt(220.1,1,-239.8);}else{camera.position.set(231,7,-223);camera.lookAt(222,1.8,-239);}renderer.render(scene,camera);};
    window.john=john;window.view('shirt');window.ready=true;
  </script></body></html>`;
  const server=http.createServer((req,res)=>{if(req.url==='/fixture'){res.setHeader('content-type','text/html');res.end(fixture);return;}const relative=decodeURIComponent(req.url.split('?')[0]).replace(/^\/+/,''),file=path.resolve(web,relative);if(!file.startsWith(web+path.sep)||!fs.existsSync(file)){res.writeHead(404);res.end();return;}res.setHeader('content-type',file.endsWith('.js')?'text/javascript':file.endsWith('.jpg')?'image/jpeg':'application/octet-stream');fs.createReadStream(file).pipe(res);});
  await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));let browser;const errors=[];
  try{
    browser=await chromium.launch({executablePath:process.env.CK_CHROME||(process.platform==='win32'?'C:/Program Files/Google/Chrome/Application/chrome.exe':undefined),headless:true,args:['--use-angle=swiftshader','--enable-unsafe-swiftshader','--enable-webgl']});const page=await browser.newPage({viewport:{width:1100,height:800}});page.on('pageerror',error=>errors.push(error.message));
    await page.goto('http://127.0.0.1:'+server.address().port+'/fixture');await page.waitForFunction(()=>window.ready&&window.john.root.getObjectByName('john-pork-shirt-portrait').material.map.image?.naturalWidth>0);
    for(const mode of ['shirt','bed','hut']){await page.evaluate(mode=>window.view(mode),mode);await page.screenshot({path:path.join(out,mode+'.png')});assert.equal(await page.evaluate(()=>window.ringing),mode==='shirt');}
    assert.deepEqual(errors,[]);console.log('PASS: actual local portrait texture loads, sleeping John remains silent, and hut/bed render without errors.');console.log('Screenshots: '+out);
  }finally{await browser?.close();await new Promise(resolve=>server.close(resolve));}
})().catch(error=>{console.error(error);process.exitCode=1;});
