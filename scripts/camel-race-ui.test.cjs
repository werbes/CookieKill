const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),{test}=require('node:test');
const root=path.resolve(__dirname,'..'),read=file=>fs.readFileSync(path.join(root,file),'utf8'),url=s=>'data:text/javascript;base64,'+Buffer.from(s).toString('base64');
const modules=Promise.all([import(url(read('web/vendor/three.core.js'))),import(url(read('web/camel-race-scene.js'))),import(url(read('web/camel-race-panels.js'))),import(url(read('web/camel-care-panels.js')))]);

test('beach cliff has an unobstructed surface entrance and a visible camel lollipop',async()=>{
  const [THREE,{buildBeachRaceCave}]=await modules,parent=new THREE.Group(),{group}=buildBeachRaceCave({THREE,parent,flatLabel:()=>new THREE.Group()});
  assert.deepEqual(group.getObjectByName('camel-race-lollipop').position.toArray(),[-227,0,20]);
  group.updateMatrixWorld(true);const ray=new THREE.Raycaster(new THREE.Vector3(-214,1.7,21),new THREE.Vector3(-1,0,0),0,8);
  assert.equal(ray.intersectObject(group,true).length,0,'east-facing doorway must stay open');
  ray.set(new THREE.Vector3(-214,1.7,15),new THREE.Vector3(-1,0,0));assert.ok(ray.intersectObject(group,true).length>0,'cliff face should look solid outside the doorway');
});

test('race arrows follow server direction, gate uses next checkpoint, and arena hides on return',async()=>{
  const [THREE,{createCamelRaceScene}]=await modules,scene=new THREE.Scene(),race=createCamelRaceScene({THREE,scene,flatLabel:()=>new THREE.Group()});
  assert.equal(race.group.visible,false);
  const snap={me:{id:'one',island:'camel_race'},camelRace:{phase:'racing',entrants:[{id:'one',status:'racing',checkpoint:4}],track:{checkpoints:16}}};race.sync(snap);
  assert.equal(race.group.visible,true);const gate=race.group.getObjectByName('camel-race-next-checkpoint');assert.ok(Math.abs(gate.position.x)<1e-8);assert.equal(gate.position.z,33.5);
  for(let i=0;i<16;i++){const a=i*Math.PI*2/16,arrow=race.group.getObjectByName('camel-race-direction-'+i),tip=new THREE.Vector3(0,1,0).applyEuler(arrow.rotation),tangent=new THREE.Vector3(-51.5*Math.sin(a),0,33.5*Math.cos(a)).normalize();assert.ok(tip.dot(tangent)>.99999,'arrow points the wrong way at '+i);}
  snap.camelRace.entrants[0].status='finished';race.sync(snap);assert.equal(gate.visible,false);
  snap.me.island='';race.sync(snap);assert.equal(race.group.visible,false);
  assert.ok(race.group.getObjectByName('camel-race-fences').isInstancedMesh,'fencing should share draw calls');
});

test('entry panel respects lollipop reach, cancellation, gathered purse and race status',async()=>{
  const [,,{createCamelRacePanels,raceClock}]=await modules,buttons=[];
  const state={connected:true,snapshot:{time:60,me:{id:'one',x:-227,z:20},camelRace:{nextStartAt:120,registeredStartAt:0,entrantCount:0,entrants:[]}}};
  const panel=createCamelRacePanels({state,escapeHTML:String,button:(label,action,data,disabled)=>{buttons.push({label,action,data,disabled});return label;}});
  assert.equal(raceClock(61),'1:01');assert.match(panel.render(),/Next race in 1:00/);assert.equal(buttons[0].action,'race_enter');assert.equal(!!buttons[0].disabled,false);
  buttons.length=0;state.snapshot.me.x=-220;panel.render();assert.equal(!!buttons[0].disabled,true);
  state.snapshot.camelRace.registeredStartAt=120;buttons.length=0;assert.match(panel.render(),/Gather in 0:40/);assert.equal(buttons[0].action,'race_leave');
  Object.assign(state.snapshot.me,{island:'camel_race'});Object.assign(state.snapshot.camelRace,{phase:'gathering',startAt:120,entrantCount:5,entrants:[{id:'one',name:'Racer',status:'racing',lap:0}]});
  const markup=panel.render();assert.match(markup,/800 coins this race/);assert.match(markup,/400 coins this race/);assert.match(markup,/200 coins this race/);
});

test('camel care shows individual stats and only offers usable nearby uncapped treats',async()=>{
  const [,,,{createCamelCarePanels}]=await modules,buttons=[];
  const state={connected:true,snapshot:{me:{x:1,z:2,inventory:{happy_apple:2},camels:[{id:'one_camel_1',name:'Daisy',x:1,z:2,speed:10.25,jump:6.5,stamina:130,happiness:80}]},town:{treatCatalog:[{id:'happy_apple',name:'Happy apple',stat:'happiness',gain:0,cap:100,multiplier:2,work:120,maxWork:240}]}}};
  const care=createCamelCarePanels({state,escapeHTML:String,button:(label,action,data,disabled)=>{if(action==='feed_camel')buttons.push({label,action,data,disabled});return label;}});
  assert.match(care.herd(),/Speed: 10.25m\/s/);assert.deepEqual(buttons[0].data,{target:'one_camel_1',item:'happy_apple'});assert.equal(buttons[0].disabled,false);
  for(const change of [()=>state.snapshot.me.camels[0].happiness=100,()=>{state.snapshot.me.camels[0].happiness=80;state.snapshot.me.x=100;},()=>{state.snapshot.me.x=1;state.snapshot.me.island='camel_race';}]){change();buttons.length=0;care.herd();assert.equal(buttons[0].disabled,true);}
});

test('race and town snapshots replace atomically while unrelated player state survives',async()=>{
  const {createSnapshotDecoder}=await import(url(read('web/state-sync.js'))),decoder=createSnapshotDecoder();
  const first=decoder.apply({type:'snapshot',seq:1,me:{id:'one',coins:10},camelRace:{phase:'registration',entrants:[]},town:{leaderboards:[{id:'kills',entries:[]}]}});
  const next=decoder.apply({type:'delta',seq:2,base:1,camelRace:{phase:'gathering',entrants:[{id:'one'}]},town:{leaderboards:[]}});
  assert.equal(next.camelRace.phase,'gathering');assert.equal(next.me.coins,10);assert.deepEqual(next.town.leaderboards,[]);assert.equal(first.camelRace.phase,'registration');
});
