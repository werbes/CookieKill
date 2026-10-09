const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),{test}=require('node:test');
const root=path.resolve(__dirname,'..'),load=file=>import('data:text/javascript;base64,'+Buffer.from(fs.readFileSync(path.join(root,file),'utf8')).toString('base64'));
const modules=Promise.all([load('web/vendor/three.core.js'),load('web/night-scene.js')]);
test('night lighting begins in daylight and fades at night and each dawn',async()=>{
  const [, {nightFactor}]=await modules;
  assert.equal(nightFactor({phase:'day',elapsed:0,dayNumber:0}),0);
  assert.equal(nightFactor({phase:'day',elapsed:719,dayNumber:0}),0);
  assert.equal(nightFactor({phase:'night',elapsed:0}),0);
  assert.equal(nightFactor({phase:'night',elapsed:6}),.5);
  assert.equal(nightFactor({phase:'night',elapsed:359}),1);
  assert.equal(nightFactor({phase:'day',elapsed:0,dayNumber:1}),1);
  assert.equal(nightFactor({phase:'day',elapsed:12,dayNumber:1}),0);
});
test('John warnings use actual range, target and world with exact 100/10 metre boundaries',async()=>{
  const [,{johnPorkWarning}]=await modules,s={dayCycle:{phase:'night'},me:{id:'me',x:0,z:0},johnPork:{x:100,z:0,target:'other'}};
  assert.equal(johnPorkWarning(s).text,'john pork is near');s.johnPork.x=100.01;assert.equal(johnPorkWarning(s),null);
  s.johnPork.x=10;s.johnPork.target='me';assert.equal(johnPorkWarning(s).text,'RUN FROM JOHN PORK');
  s.johnPork.target='other';assert.equal(johnPorkWarning(s).urgent,false);
  s.johnPork.target='me';s.johnPork.y=2;assert.equal(johnPorkWarning(s).urgent,false);
  s.johnPork.island='different';assert.equal(johnPorkWarning(s),null);s.johnPork.island='';s.johnPork.inCave=true;assert.equal(johnPorkWarning(s),null);
  s.johnPork.inCave=false;s.dayCycle.phase='day';assert.equal(johnPorkWarning(s),null);
  s.dayCycle.phase='night';s.johnPork.sleeping=true;assert.equal(johnPorkWarning(s),null,'sleeping John must never trigger an alarm');
});
test('home owners and visitors never receive John warnings even from matching stale island data',async()=>{
  const [,{johnPorkWarning}]=await modules;
  for(const island of ['me','friend','camel_race','camel_casual:one','camel_training','cookie_ring','cookie_practice']){
    const s={dayCycle:{phase:'night'},me:{id:'me',island,x:0,z:0},johnPork:{island,x:0,z:.5,target:'me',eating:true}};
    assert.equal(johnPorkWarning(s),null,`${island} must suppress stale pursuit warnings`);
  }
});
test('phone melody requires audio unlock, loops only in range, and stops on mute/departure',async()=>{
  const [,{createPhoneRinging,PHONE_MELODY}]=await modules,voices=[],contexts=[];
  const param=()=>({value:0,setValueAtTime(){},setTargetAtTime(){},cancelScheduledValues(){},exponentialRampToValueAtTime(){}});
  class AudioContext {constructor(){this.currentTime=0;this.state='suspended';contexts.push(this);}resume(){this.state='running';return Promise.resolve();}close(){this.state='closed';return Promise.resolve();}createGain(){return {gain:param(),connect(){},disconnect(){}};}createOscillator(){const voice={frequency:param(),connect(){},disconnect(){},start(){this.started=true;},stop(){this.stopped=true;}};voices.push(voice);return voice;}}
  const audio=createPhoneRinging({AudioContext});audio.setRange(50);audio.update();assert.equal(contexts.length,0);assert.equal(voices.length,0);
  audio.unlock();audio.update();assert.equal(audio.playing,true);assert.equal(voices.length,PHONE_MELODY.length*3);
  audio.update();assert.equal(voices.length,PHONE_MELODY.length*3,'one ring loop, not one per frame');contexts[0].currentTime=5;audio.update();assert.equal(voices.length,PHONE_MELODY.length*6);
  audio.setRange(101);assert.equal(audio.playing,false);assert.ok(voices.every(v=>v.stopped));
  audio.setRange(5);audio.setMuted(true);audio.update();assert.equal(audio.playing,false);audio.setMuted(false);audio.setRange(5);audio.update();assert.equal(audio.playing,true);
  audio.dispose();assert.equal(contexts[0].state,'closed');
});
test('John model wears the portrait, follows pursuit, and sleeps quietly at dawn',async()=>{
  const [THREE,{createNightScene}]=await modules,parent=new THREE.Group(),primitive=(geometry,c,x,y,z,p)=>{const mesh=new THREE.Mesh(geometry,new THREE.MeshBasicMaterial({color:c}));mesh.position.set(x,y,z);p.add(mesh);return mesh;};
  const portraitTexture=new THREE.Texture(),audio={unlock(){},setMuted(){},setRange(distance,enabled){this.enabled=enabled;},update(){},stop(){this.stopped=true;},dispose(){this.disposed=true;}},view=createNightScene({THREE,parent,audio,document:null,portraitTexture,box:(w,h,d,c,x,y,z,p)=>primitive(new THREE.BoxGeometry(w,h,d),c,x,y,z,p),sphere:(r,c,x,y,z,p)=>primitive(new THREE.IcosahedronGeometry(r,1),c,x,y,z,p),cylinder:(a,b,h,c,x,y,z,p)=>primitive(new THREE.CylinderGeometry(a,b,h,8),c,x,y,z,p),disposeObject:o=>o.traverse(c=>c.geometry?.dispose())});
  const s={dayCycle:{phase:'night'},me:{id:'me',x:0,z:0},johnPork:{x:12,y:0,z:0,yaw:0,target:'me'}};view.sync(s);assert.equal(view.root.visible,true);assert.equal(view.root.position.x,12);assert.ok(view.root.getObjectByName('pig-head'));assert.ok(view.root.getObjectByName('ringing-phone'));
  s.johnPork.x=10;s.johnPork.eating=true;view.sync(s);view.update(.1,.05);assert.ok(view.root.position.x<12);assert.notEqual(view.root.getObjectByName('pig-head').rotation.x,0);
  const portrait=view.root.getObjectByName('john-pork-shirt-portrait');assert.equal(portrait.material.map,portraitTexture);assert.equal(portraitTexture.colorSpace,THREE.SRGBColorSpace);assert.equal(portrait.userData.source,'/assets/john-pork.jpg');assert.equal(portrait.rotation.y,Math.PI);
  view.sync({...s,johnPork:{...s.johnPork,target:'other',x:18}});assert.equal(view.root.position.x,18,'retargeting snaps instead of sliding through walls');
  s.dayCycle.phase='day';s.johnPork={x:220.2,y:1,z:-238.8,yaw:Math.PI,sleeping:true};view.sync(s);view.update(1,.05);assert.equal(view.root.visible,true);assert.equal(view.root.position.x,220.2);assert.equal(view.root.getObjectByName('john-pork-body').rotation.x,Math.PI/2);assert.equal(view.root.getObjectByName('john-pork-sleeping-zzz').visible,true);assert.equal(audio.enabled,false);assert.equal(view.root.getObjectByName('pig-head').rotation.x,0);
  s.dayCycle.phase='night';s.johnPork={x:12,y:0,z:0,yaw:0,target:'me'};view.sync(s);view.update(2,.05);assert.equal(view.root.getObjectByName('john-pork-body').rotation.x,0);assert.equal(view.root.getObjectByName('john-pork-sleeping-zzz').visible,false);assert.equal(audio.enabled,true);
  for(const island of ['me','friend','camel_casual:one']){
    s.me.island=island;s.johnPork.island=island;view.sync(s);view.update(3,.05);
    assert.equal(view.root.visible,false,'matching stale island data cannot render John');assert.equal(audio.enabled,false,'island arrival stops ringing');
    s.dayCycle.phase='day';s.johnPork.sleeping=true;view.sync(s);assert.equal(view.root.visible,false,'island cannot show the sleeping mainland model');
    s.dayCycle.phase='night';s.johnPork.sleeping=false;
  }
  s.me.island='';s.johnPork.island='';view.sync(s);assert.equal(view.root.visible,true,'mainland pursuit can resume after an island visit');
  view.clear();assert.equal(audio.stopped,true);view.dispose();assert.equal(parent.children.length,0);assert.equal(audio.disposed,true);
});

test('John hut matches physical walls and leaves the south doorway and bed aisle open',async()=>{
  const [THREE]=await modules,{buildJohnHut,JOHN_HUT_X,JOHN_HUT_Z}=await load('web/john-hut-scenery.js');
  const parent=new THREE.Group(),hut=buildJohnHut({THREE,parent,flatLabel:()=>new THREE.Group()});assert.deepEqual(hut.position.toArray(),[222,0,-239]);assert.equal(hut.parent,parent);
  assert.deepEqual(hut.getObjectByName('john-hut-west').scale.toArray(),[.3,3,8.3]);assert.deepEqual(hut.getObjectByName('john-hut-door-lintel').position.toArray(),[0,2.75,4]);assert.deepEqual(hut.getObjectByName('john-hut-bed').position.toArray(),[-1.8,.25,-.9]);
  hut.updateMatrixWorld(true);const ray=new THREE.Raycaster(new THREE.Vector3(JOHN_HUT_X,1.7,JOHN_HUT_Z+6),new THREE.Vector3(0,0,-1),0,7);
  assert.equal(ray.intersectObject(hut,true).length,0,'doorway blocks the path from the desert');
  ray.set(new THREE.Vector3(JOHN_HUT_X+2.6,1.7,JOHN_HUT_Z+6),new THREE.Vector3(0,0,-1));assert.ok(ray.intersectObject(hut,true).length>0,'solid front wall should be visible');
  assert.ok(hut.getObjectByName('john-hut-sign'));assert.ok(hut.getObjectByName('john-hut-hours'));
});

test('the clock does not cover open menus, and informational HUD elements cannot intercept clicks',async()=>{
  const [THREE,{createNightScene}]=await modules,elements=new Map([['modal',{hidden:true}]]),element=()=>({hidden:false,style:{},children:[],dataset:{},appendChild(child){this.children.push(child);elements.set(child.id,child);},append(...children){children.forEach(child=>this.appendChild(child));},setAttribute(){},addEventListener(){},remove(){}}),document={hidden:false,head:element(),body:element(),getElementById:id=>elements.get(id),createElement:element},audio={unlock(){},setMuted(){},setRange(){},update(){},stop(){},dispose(){}},primitive=(...args)=>{const view=new THREE.Group();args.find(arg=>arg?.isObject3D)?.add(view);return view;};
  const scene=createNightScene({THREE,parent:new THREE.Group(),box:primitive,sphere:primitive,cylinder:primitive,document,audio});scene.sync({me:{id:'me'},dayCycle:{phase:'day',remaining:720}});
  const hud=elements.get('night-hud');assert.equal(hud.hidden,false);assert.equal(hud.style.pointerEvents,'none');assert.equal(elements.get('john-pork-warning').style.pointerEvents,'none');
  elements.get('modal').hidden=false;scene.update(0,.05);assert.equal(hud.hidden,true);elements.get('modal').hidden=true;scene.update(1,.05);assert.equal(hud.hidden,false);scene.clear();scene.update(2,.05);assert.equal(hud.hidden,true);scene.dispose();
});
