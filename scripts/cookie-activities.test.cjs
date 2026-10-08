const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),{test}=require('node:test');
const root=path.resolve(__dirname,'..'),read=f=>fs.readFileSync(path.join(root,f),'utf8'),url=s=>'data:text/javascript;base64,'+Buffer.from(s).toString('base64'),art=url(read('web/cookie-activity-art.js'));
const modules=Promise.all([import(url(read('web/vendor/three.core.js'))),import(url(read('web/tree-ring-scene.js').replace('./cookie-activity-art.js',art))),import(url(read('web/cookie-practice-scene.js').replace('./cookie-activity-art.js',art))),import(url(read('web/cookie-activities-panels.js')))]);
function fixture(THREE){const scene=new THREE.Scene(),worldParent=new THREE.Group();scene.add(worldParent);return {THREE,scene,worldParent,flatLabel:text=>{const g=new THREE.Group();g.userData.text=text;return g;}};}
test('slender trunk has 64 open spiral treads, two complete turns and the unchanged full hut deck',async()=>{
  const [THREE,{createTreeRingScene}]=await modules,f=fixture(THREE),api=createTreeRingScene(f),steps=api.tree.userData.steps;
  assert.equal(steps.length,64);for(let i=0;i<64;i++){const step=steps[i],angle=13*Math.PI/16+(i+.5)*2*Math.PI/32;assert.equal(step.top,(i+1)*.1875);assert.equal(step.x,210+3.4*Math.cos(angle));assert.equal(step.z,184+3.4*Math.sin(angle));assert.equal(step.rotation,-angle);assert.equal(step.width,2.2);assert.equal(step.depth,1.05);}
  f.scene.updateMatrixWorld(true);const ray=new THREE.Raycaster(),trunk=api.tree.getObjectByName('cookie-tree-trunk'),trunkBounds=new THREE.Box3().setFromObject(trunk);assert.equal(trunk.position.x,210);assert.equal(trunk.position.z,184);assert.equal(trunkBounds.min.y,0);assert.equal(trunkBounds.max.y,12);assert.equal(trunk.scale.x,1.5);assert.equal(trunk.scale.z,1.5);assert.ok(trunkBounds.max.x-trunkBounds.min.x<=3.001);
  for(const step of steps){ray.set(new THREE.Vector3(step.x,step.top+.06,step.z),new THREE.Vector3(0,-1,0));ray.far=.12;const hit=ray.intersectObject(api.tree,true)[0];assert.ok(hit,'visible tread missing at '+step.top);assert.ok(Math.abs(hit.point.y-step.top)<.001,'visible tread must match physical top');ray.set(new THREE.Vector3(step.x,step.top+.2,step.z),new THREE.Vector3(0,1,0));ray.far=1.5;assert.equal(ray.intersectObject(api.tree,true).length,0,'spiral needs standing headroom at '+step.top);}
  for(const [x,z]of [[207,177.9],[213,177.5],[215,181.5],[207,183],[207.6,186.65],[203.2,174.2],[216.8,183.8]]){ray.set(new THREE.Vector3(x,12.08,z),new THREE.Vector3(0,-1,0));ray.far=.15;const hit=ray.intersectObject(api.tree,true)[0];assert.ok(hit,'continuous hut/bridge slab missing at '+x+','+z);assert.ok(Math.abs(hit.point.y-12)<.001);}
  for(const [x,z,dx,dz]of [[203,184,1,0],[217,184,-1,0],[210,193,0,-1]]){ray.set(new THREE.Vector3(x,2,z),new THREE.Vector3(dx,0,dz));ray.far=1.5;assert.equal(ray.intersectObject(api.tree,true).length,0,'exterior must remain open instead of the old broad wall enclosure');}
  let batches=0;api.tree.traverse(o=>{if(o.isInstancedMesh)batches++;});assert.ok(batches>0&&batches<40);api.sync({me:{island:'cookie_ring'}});assert.equal(api.ring.visible,true);api.sync({me:{island:'cookie_practice'}});assert.equal(api.ring.visible,false);api.dispose();assert.equal(f.worldParent.children.length,0);
});
test('ring cover top surfaces and perimeter netting match physical arena heights',async()=>{
  const [THREE,{createTreeRingScene,COOKIE_RING_COVER}]=await modules,f=fixture(THREE),api=createTreeRingScene(f);api.ring.updateMatrixWorld(true);const ray=new THREE.Raycaster();
  for(const c of COOKIE_RING_COVER){ray.set(new THREE.Vector3(c.x,c.height+.2,c.z),new THREE.Vector3(0,-1,0));ray.far=.5;assert.ok(Math.abs(ray.intersectObject(api.ring,true)[0].point.y-c.height)<.001);}
  assert.equal(api.ring.children.filter(o=>o.isLineSegments).length,4);assert.ok(api.ring.getObjectByName('cookie-ring-exit'));api.dispose();
});
test('practice target motion/hit glow, apparatus animation and swimming guide follow authoritative snapshot state',async()=>{
  const [THREE,,{createCookiePracticeScene}]=await modules,f=fixture(THREE),api=createCookiePracticeScene(f),snapshot={time:5,me:{island:'cookie_practice'},cookiePractice:{targets:[{id:'practice_target_0',x:-16,y:1.8,z:-21,flashUntil:6},{id:'practice_target_1',x:-8,y:1.8,z:-21},{id:'practice_target_3',x:12+Math.sin(4)*5,y:1.8,z:-21,moving:true}],stations:[{id:'practice_treadmill',active:true},{id:'practice_bench_press',active:true},{id:'practice_leg_press',active:true}],exercise:{station:'',nextGate:{x:0,z:0}}}};
  api.sync(snapshot);api.update(.1);assert.equal(api.group.visible,true);assert.equal(api.group.getObjectByName('practice-swim-next-gate').visible,false,'zero nextGate is not a swimming session');
  const first=api.group.getObjectByName('practice-target-0').children[0],second=api.group.getObjectByName('practice-target-1').children[0];assert.ok(first.material.emissiveIntensity>0);assert.equal(second.material.emissiveIntensity,0,'one target hit must not flash other targets');
  assert.ok(Math.abs(api.group.getObjectByName('practice-target-3').position.x-(12+Math.sin(5.1*.8)*5))<1e-8);
  const bar=api.group.getObjectByName('practice-bench-bar');assert.notEqual(bar.position.y,1.38);snapshot.cookiePractice.stations[1].active=false;snapshot.cookiePractice.exercise={station:'practice_pool',nextGate:{x:14,z:-5}};api.sync(snapshot);api.update(.2);assert.equal(bar.position.y,1.38);const gate=api.group.getObjectByName('practice-swim-next-gate');assert.equal(gate.visible,true);assert.deepEqual(gate.position.toArray(),[14,.08,-5]);
  api.group.updateMatrixWorld(true);const water=api.group.getObjectByName('practice-pool-water');assert.equal(water.position.y,.003);
  snapshot.cookiePractice.exercise={station:'practice_treadmill'};api.sync(snapshot);assert.equal(api.group.getObjectByName('practice_treadmill-label').visible,false,'occupied apparatus must not put a huge sign in the local first-person view');assert.equal(api.group.getObjectByName('practice_bench_press-label').visible,true);snapshot.cookiePractice.exercise={};api.sync(snapshot);assert.equal(api.group.getObjectByName('practice_treadmill-label').visible,true);
  snapshot.me.island='';api.sync(snapshot);assert.equal(api.group.visible,false);api.dispose();assert.equal(f.scene.children.length,1);
});
test('activity menus use exact portal reach and offer physical-exercise instructions without instant training buttons',async()=>{
  const [,,,{createCookieActivityPanels}]=await modules,buttons=[],state={connected:true,snapshot:{me:{x:213,y:12,z:177.5,grounded:true}}},api=createCookieActivityPanels({state,escapeHTML:String,button:(label,action,data,disabled)=>{buttons.push({label,action,data,disabled});return label;}});
  api.shop('tree_golden_cookie');assert.deepEqual(buttons[0],{label:'Eat golden cookie · Fighting ring',action:'cookie_ring_enter',data:{target:'tree_golden_cookie'},disabled:false});
  for(const update of [{ridingCamel:true},{grounded:false},{y:0},{x:217}]){const before={...state.snapshot.me};Object.assign(state.snapshot.me,update);buttons.length=0;api.shop('tree_golden_cookie');assert.equal(buttons[0].disabled,true);state.snapshot.me=before;}
  state.snapshot={me:{island:'cookie_practice',levels:{}},cookiePractice:{stations:[{id:'practice_bench_press',name:'Bench press',stat:'arms',instructions:'Press Space for each rep.'}],exercise:{station:'practice_bench_press',progress:4,goal:10,unit:'reps',status:'Lift'},hits:3}};buttons.length=0;const html=api.render();assert.match(html,/4 \/ 10 reps/);assert.deepEqual(buttons.map(b=>b.action),['practice_stop','cookie_arena_leave']);assert.doesNotMatch(html,/Train ·/);
});
test('local map draws arena-specific cover and practice lanes instead of the mainland',async()=>{
  const [,,{paintCookieActivityMap}]=await modules,calls=[],ctx=new Proxy({}, {get:(o,k)=>o[k]||((...args)=>calls.push([k,...args])),set:(o,k,v)=>(o[k]=v,true)}),canvas={width:500,height:500,getContext:()=>ctx};
  paintCookieActivityMap(canvas,{me:{island:'cookie_ring',x:0,z:0,yaw:0},players:[]});assert.ok(calls.some(c=>c[0]==='fillText'&&c[1]==='COOKIE FIGHTING RING'));
  calls.length=0;paintCookieActivityMap(canvas,{me:{island:'cookie_practice',x:0,z:20},cookiePractice:{stations:[],targets:[],exercise:{station:'',nextGate:{x:0,z:0}}}});assert.ok(calls.some(c=>c[0]==='fillText'&&c[1]==='AIM PRACTICE & PHYSICAL GYM'));
});

test('activity HUD leaves the pointer capture prompt and menus reachable',async()=>{
  const [,,,{createCookieActivityHUD}]=await modules,elements=[],document={getElementById:id=>elements.find(e=>e.id===id),createElement:()=>({hidden:false,addEventListener(){},remove(){this.removed=true;}}),head:{appendChild:e=>elements.push(e)},body:{appendChild:e=>elements.push(e)}};
  const state={connected:true,hasPlayed:true,locked:false,menuOpen:false,modal:null,snapshot:{me:{island:'cookie_practice'},cookiePractice:{hits:2}}},api=createCookieActivityHUD({document,state,openModal(){}});
  api.update();assert.equal(api.root.hidden,true,'resume pointer capture must remain clickable');
  state.locked=true;api.update();assert.equal(api.root.hidden,false);assert.match(api.root.textContent,/2 HITS/);
  for(const field of ['menuOpen','modal']){state[field]=true;api.update();assert.equal(api.root.hidden,true);state[field]=false;}
  state.locked=false;api.update();assert.equal(api.root.hidden,true);state.locked=true;state.snapshot.me.island='';api.update();assert.equal(api.root.hidden,true);api.dispose();assert.equal(api.root.removed,true);
});

test('exercise avatar rig preserves limb positions before animation and resets resting treadmill poses',async()=>{
  const [THREE]=await modules,{createPracticeAvatarRig,animatePracticeAvatar}=await import(url(read('web/practice-avatar.js'))),body=new THREE.Group(),parts=[];
  const add=(name,x,y,z=0)=>{const mesh=new THREE.Mesh(new THREE.BoxGeometry(.1,.1,.1),new THREE.MeshBasicMaterial());mesh.name=name;mesh.position.set(x,y,z);body.add(mesh);parts.push(mesh);return mesh;};
  const head=add('head',0,1.68),torso=add('torso',0,1.05),held=new THREE.Group();held.position.set(.37,.97,-.2);body.add(held);
  for(const side of [-1,1]){add('leg',side*.16,.38);add('shorts',side*.16,.24,-.005);add('shoe',side*.16,.1,-.07);add('arm',side*.36,1.05);add('sleeve',side*.34,1.26);}
  body.updateMatrixWorld(true);const before=new Map(parts.map(p=>[p,p.getWorldPosition(new THREE.Vector3())])),rig=createPracticeAvatarRig(THREE,body,'treadmill');body.updateMatrixWorld(true);
  for(const part of parts)assert.ok(before.get(part).distanceTo(part.getWorldPosition(new THREE.Vector3()))<1e-10,'reparenting moved '+part.name);
  assert.equal(head.parent,body);assert.equal(torso.parent,body);assert.equal(held.visible,false);assert.deepEqual(rig.legs.map(l=>l.children.length),[3,3]);assert.deepEqual(rig.arms.map(a=>a.children.length),[2,2]);
  animatePracticeAvatar(rig,true,.2);assert.notEqual(rig.legs[0].rotation.x,0);assert.equal(rig.legs[0].rotation.x,-rig.legs[1].rotation.x);assert.ok(rig.legs[0].rotation.x*rig.arms[0].rotation.x<0);
  animatePracticeAvatar(rig,false,.4);for(const limb of [...rig.legs,...rig.arms])assert.equal(Math.abs(limb.rotation.x),0);assert.equal(body.position.y,0);
  for(const posture of ['bench_press','leg_press']){const posed=createPracticeAvatarRig(THREE,new THREE.Group(),posture);animatePracticeAvatar(posed,false,0);const resting=posed.arms[0].rotation.x+posed.legs[0].rotation.x;animatePracticeAvatar(posed,true,.2);assert.notEqual(posed.arms[0].rotation.x+posed.legs[0].rotation.x,resting);}
});
