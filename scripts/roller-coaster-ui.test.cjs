const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),{test}=require('node:test');
const root=path.resolve(__dirname,'..'),read=file=>fs.readFileSync(path.join(root,file),'utf8'),url=source=>'data:text/javascript;base64,'+Buffer.from(source).toString('base64');
const motionURL=url(read('web/motion.js'));
const modules=Promise.all([import(url(read('web/vendor/three.core.js'))),import(url(read('web/roller-coaster-scene.js').replace("'./motion.js'",JSON.stringify(motionURL)))),import(url(read('web/roller-coaster-panels.js')))]);
const sample=(t,x,y,z,yaw=0,pitch=0,bank=0)=>({t,x,y,z,yaw,pitch,bank});
const car=(id,playerId,x,y=1.1)=>({id,playerId,x,y,z:-165,yaw:0,pitch:0,bank:0,startedAt:0,progress:0,remaining:48});
const snapshot=(time,cars)=>({time,me:{id:'one',island:''},rollerCoaster:{station:{x:165,y:0,z:-161},duration:48,track:[sample(0,165,1.1,-165),sample(12,123,24,-192,Math.PI/2,.3,.1),sample(24,165,5,-219,Math.PI),sample(36,207,14,-192,-Math.PI/2),sample(48,165,1.1,-165)],cars}});

test('coaster rails and elevated supports use the server track; station leaves the boarding path open',async()=>{
  const [THREE,{createRollerCoasterScene}]=await modules,scene=new THREE.Scene(),coaster=createRollerCoasterScene({THREE,scene,flatLabel:()=>new THREE.Group()}),state=snapshot(0,[car('empty','',165)]);
  coaster.sync(state);scene.updateMatrixWorld(true);
  assert.equal(coaster.group.visible,true);assert.equal(coaster.group.userData.trackSamples,5);
  const rails=coaster.group.getObjectByName('coaster-rail-0');assert.ok(rails.isInstancedMesh);assert.equal(rails.count,4);
  const ties=coaster.group.getObjectByName('coaster-track-ties'),matrix=new THREE.Matrix4(),position=new THREE.Vector3();ties.getMatrixAt(1,matrix);position.setFromMatrixPosition(matrix);assert.deepEqual(position.toArray(),[123,23.75,-192],'elevated sleepers follow server floor height');
  const station=coaster.group.getObjectByName('coaster-station');assert.deepEqual(station.position.toArray(),[165,0,-161]);
  const entrance=new THREE.Raycaster(new THREE.Vector3(165,1.7,-154),new THREE.Vector3(0,0,-1),0,8);assert.equal(entrance.intersectObject(station,true).length,0,'boarding corridor is clear at eye height');
  const firstRails=rails;coaster.sync(snapshot(.05,[car('empty','',165.2)]));assert.equal(coaster.group.getObjectByName('coaster-rail-0'),firstRails,'unchanged geometry is reused');
  coaster.dispose();assert.equal(scene.children.includes(coaster.group),false);
});

test('cars interpolate authoritative poses on local and remote timelines and never extrapolate',async()=>{
  const [THREE,{createRollerCoasterScene}]=await modules,scene=new THREE.Scene(),coaster=createRollerCoasterScene({THREE,scene,flatLabel:()=>new THREE.Group()});
  const first=snapshot(1,[{...car('mine','one',165),camelId:'my_camel'},car('peer','two',170)]);coaster.sync(first);
  const next=snapshot(1.1,[{...car('mine','one',167,3.1),camelId:'my_camel',pitch:.4,bank:.2,yaw:.6},{...car('peer','two',174),yaw:.8}]);coaster.sync(next);coaster.update(1025,1050);
  const mine=coaster.group.getObjectByName('coaster-car-mine'),peer=coaster.group.getObjectByName('coaster-car-peer'),pose=coaster.poseForPlayer('one');
  assert.ok(Math.abs(mine.position.x-166)<1e-8);assert.ok(Math.abs(mine.position.y-2.1)<1e-8);assert.ok(Math.abs(peer.position.x-171)<1e-8);
  assert.ok(Math.abs(pose.yaw-.3)<1e-8);assert.ok(Math.abs(pose.pitch-.2)<1e-8);assert.ok(Math.abs(pose.bank-.1)<1e-8);assert.equal(mine.userData.camelId,'my_camel');
  const forward=new THREE.Vector3(0,0,-1).applyQuaternion(mine.quaternion);assert.ok(forward.y>0,'positive pitch tips the front of the car uphill');
  coaster.update(5000);assert.equal(mine.position.x,167);assert.equal(peer.position.x,174,'a stalled connection must not invent motion');
  const floor=mine.getObjectByName('coaster-car-floor');assert.equal(floor.geometry.parameters.width,3.8);assert.equal(floor.geometry.parameters.depth,5.6);assert.equal(floor.position.y+floor.geometry.parameters.height/2,0,'car floor matches rider feet');
  coaster.sync(snapshot(1.6,[car('mine','one',173),car('peer','two',176)]));coaster.update(1450);assert.equal(mine.position.x,173,'network gaps reset on the same threshold as avatar motion');
  coaster.sync({...snapshot(1.65,[]),me:{id:'one',island:'friend'}});assert.equal(coaster.group.visible,false);assert.equal(coaster.poseForPlayer('two'),null,'a home-island avatar must not inherit a stale ride tilt');
  coaster.sync(snapshot(.1,[car('mine','one',165)]));coaster.update(100);assert.equal(coaster.poseForPlayer('one').x,165,'new server time resets buffered car motion');
  coaster.sync(snapshot(1.2,[car('empty','',165)]));assert.equal(coaster.poseForPlayer('one'),null);assert.equal(coaster.group.getObjectByName('coaster-car-mine'),undefined);coaster.dispose();
});

const escapeHTML=value=>String(value).replace(/[&<>"']/g,char=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[char]));
test('boarding menu validates station reach, world, connection and dispatch while preserving camel choice',async()=>{
  const [,,{createRollerCoasterPanels}]=await modules,buttons=[],state={connected:true,snapshot:{me:{id:'one',x:165,y:0,z:-161},rollerCoaster:{station:{x:165,y:0,z:-161},duration:48,nextBoardIn:0,ride:null}}};
  const panels=createRollerCoasterPanels({state,escapeHTML,button:(label,action,data,disabled)=>{buttons.push({label,action,data,disabled});return label;}});
  let html=panels.shop('roller_coaster');assert.match(html,/Board on foot/);assert.match(html,/0:48/);assert.deepEqual(buttons[0],{label:'Board on foot',action:'coaster_board',data:{target:'desert_coaster'},disabled:false});assert.equal(panels.shop('camel'),null);
  for(const change of [{x:171},{y:.6},{grounded:false},{island:'friend'},{inCave:true},{fallen:true}]){const old={...state.snapshot.me};Object.assign(state.snapshot.me,change);buttons.length=0;panels.render();assert.equal(buttons[0].disabled,true,JSON.stringify(change));state.snapshot.me=old;}
  state.connected=false;buttons.length=0;panels.render();assert.equal(buttons[0].disabled,true);state.connected=true;
  state.snapshot.rollerCoaster.nextBoardIn=1.2;buttons.length=0;assert.match(panels.render(),/0:02/);assert.equal(buttons[0].disabled,true);state.snapshot.rollerCoaster.nextBoardIn=0;
  Object.assign(state.snapshot.me,{ridingCamel:true,mountedCamelId:'c',camels:[{id:'c',name:'<img src=x>'}]});buttons.length=0;html=panels.render();assert.equal(buttons[0].label,'Board with camel');assert.equal(buttons[0].disabled,false);assert.match(html,/&lt;img/);assert.doesNotMatch(html,/<img/);
});

test('active coaster menu reports progress and offers safe early return instead of a second boarding',async()=>{
  const [,,{createRollerCoasterPanels}]=await modules,buttons=[],state={connected:true,snapshot:{me:{id:'one',x:200,y:24,z:-190,ridingCamel:true},rollerCoaster:{ride:{playerId:'one',camelId:'c',camelName:'<Daisy>',progress:.42,remaining:27.2}}}};
  const panels=createRollerCoasterPanels({state,escapeHTML,button:(label,action,data,disabled)=>{buttons.push({label,action,data,disabled});return label;}}),html=panels.render();
  assert.match(html,/42% around/);assert.match(html,/0:28/);assert.match(html,/&lt;Daisy&gt;/);assert.match(html,/return safely/);assert.equal(buttons.length,1);assert.equal(buttons[0].action,'coaster_leave');assert.equal(buttons[0].label,'Return to station now');assert.equal(buttons[0].disabled,false);
});

function fakeDocument(){const elements=[],element=()=>({listeners:{},addEventListener(name,fn){this.listeners[name]=fn;},remove(){this.removed=true;}});return {elements,createElement:element,body:{appendChild:e=>elements.push(e)},head:{appendChild:e=>elements.push(e)}};}
test('coaster HUD follows the authoritative ride and hides behind menus or after returning',async()=>{
  const [,,{createRollerCoasterHUD}]=await modules,document=fakeDocument(),opened=[],state={connected:true,hasPlayed:true,snapshot:{rollerCoaster:{ride:{progress:.25,remaining:36,camelId:'camel'}}}},hud=createRollerCoasterHUD({document,state,openModal:tab=>opened.push(tab)});
  hud.update();assert.equal(hud.root.hidden,false);assert.match(hud.root.textContent,/25% · 0:36/);assert.match(hud.root.textContent,/CAMEL ON BOARD/);hud.root.listeners.click();assert.deepEqual(opened,['coaster']);
  state.snapshot.time=100;state.snapshot.camelRace={reminderStartAt:220};hud.update();assert.equal(hud.root.className,'coaster-with-race-reminder','both ride and race reminder stay readable');state.snapshot.time=220;hud.update();assert.equal(hud.root.className,'');state.snapshot.camelRace.registeredStartAt=500;hud.update();assert.equal(hud.root.className,'coaster-with-race-reminder','registration countdown also keeps its space');
  for(const field of ['modal','connected','hasPlayed']){const old=state[field];state[field]=field==='modal'?'coaster':false;hud.update();assert.equal(hud.root.hidden,true);state[field]=old;}
  state.snapshot.rollerCoaster.ride=null;hud.update();assert.equal(hud.root.hidden,true);hud.dispose();assert.ok(document.elements.every(e=>e.removed));
});
