const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),{test}=require('node:test');
const root=path.resolve(__dirname,'..'),read=file=>fs.readFileSync(path.join(root,file),'utf8'),url=s=>'data:text/javascript;base64,'+Buffer.from(s).toString('base64');
const raceStateURL=url(read('web/camel-race-state.js')),raceModule=file=>import(url(read(file).replace(/(['"])\.\/camel-race-state\.js\1/g,JSON.stringify(raceStateURL))));
const modules=Promise.all([import(url(read('web/vendor/three.core.js'))),raceModule('web/camel-race-scene.js'),raceModule('web/camel-race-panels.js'),raceModule('web/camel-care-panels.js')]);

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

const escapeHTML=value=>String(value).replace(/[&<>"']/g,char=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[char]));

test('casual menu hosts anywhere, joins only open lobbies, escapes names and blocks unavailable players',async()=>{
  const [,,{createCamelRacePanels}]=await modules,buttons=[],state={connected:true,snapshot:{time:100,me:{id:'one',island:'friend',x:20,z:20},camelRace:{casualLobbies:[{id:'open',hostName:'<img src=x onerror=alert(1)>',entrantCount:2,phase:'lobby'},{id:'started',hostName:'Already racing',entrantCount:4,phase:'racing'}]}}};
  const panels=createCamelRacePanels({state,escapeHTML,button:(label,action,data,disabled)=>{buttons.push({label,action,data,disabled});return label;}});
  const html=panels.casualMenu();assert.match(html,/Free to host or join/);assert.match(html,/Playful races are for fun; coin prizes and camel coins are earned in scheduled races\./);assert.match(html,/&lt;img/);assert.doesNotMatch(html,/<img|Already racing/);
  assert.equal(buttons[0].action,'race_casual_host');assert.equal(buttons[0].disabled,false,'friend islands can host a race');assert.deepEqual(buttons[1].data,{target:'open'});assert.equal(buttons[1].action,'race_casual_join');
  for(const blocked of ['offline','fallen']){state.connected=blocked!=='offline';state.snapshot.me.fallen=blocked==='fallen';buttons.length=0;panels.casualMenu();assert.ok(buttons.every(b=>b.disabled),blocked+' must disable hosting and joining');}
  state.connected=true;state.snapshot.me.fallen=false;
  for(const island of ['camel_race','camel_casual:open']){state.snapshot.me.island=island;buttons.length=0;panels.casualMenu();assert.equal(buttons.length,1);assert.equal(buttons[0].action,'tab');assert.deepEqual(buttons[0].data,{tab:'race'});}
});

test('casual race panel gives only the host a lobby start, countdown and prize-free finish standings',async()=>{
  const [,,{createCamelRacePanels}]=await modules,buttons=[],state={connected:true,snapshot:{time:100,me:{id:'one',island:'camel_casual:open'},camelRace:{inArena:true,mode:'casual',hostId:'one',phase:'lobby',entrantCount:2,entrants:[{id:'one',name:'<Racer>',status:'racing',lap:0}]}}};
  const panels=createCamelRacePanels({state,escapeHTML,button:(label,action,data,disabled)=>{buttons.push({label,action,data,disabled});return label;}});
  let html=panels.render();assert.match(html,/ten-second countdown/);assert.equal(buttons.filter(b=>b.action==='race_casual_start').length,1);assert.ok(buttons.some(b=>b.action==='tab'&&b.data.tab==='teleport'));assert.match(html,/&lt;Racer&gt;/);assert.match(html,/coin prizes and camel coins are earned in scheduled races/);assert.doesNotMatch(html,/coins this race|Prizes use|other racers/);
  state.snapshot.camelRace.hostId='other';buttons.length=0;html=panels.render();assert.match(html,/Waiting for the host/);assert.equal(buttons.some(b=>b.action==='race_casual_start'),false);
  state.snapshot.camelRace.hostId='one';state.snapshot.me.fallen=true;buttons.length=0;panels.render();assert.equal(buttons.find(b=>b.action==='race_casual_start').disabled,true);state.snapshot.me.fallen=false;
  Object.assign(state.snapshot.camelRace,{phase:'gathering',startAt:110});buttons.length=0;html=panels.render();assert.match(html,/0:10 to go/);assert.equal(buttons.some(b=>b.action==='race_casual_start'),false);
  Object.assign(state.snapshot.camelRace,{phase:'finished',entrants:[{id:'one',name:'Racer',status:'finished',rank:2,finishTime:91,coins:999,camelCoins:999}]});html=panels.render();assert.match(html,/Place 2 · 1:31/);assert.match(html,/Return to previous location/);assert.doesNotMatch(html,/999|Prizes use|coins this race/);
});

function fakeRaceDocument(){
  const elements=[],element=()=>({hidden:false,listeners:{},addEventListener(name,fn){this.listeners[name]=fn;},remove(){this.removed=true;}});
  return {elements,createElement:element,body:{appendChild(el){elements.push(el);}},head:{appendChild(el){elements.push(el);}}};
}

test('two-minute scheduled reminder reaches unregistered players and obeys visibility and click rules',async()=>{
  const [,,{createCamelRaceHUD}]=await modules,{camelRaceReminder,isCamelRaceIsland}=await import(raceStateURL),document=fakeRaceDocument(),opened=[],state={connected:true,hasPlayed:true,snapshot:{time:500,me:{id:'one',island:'friend'},camelRace:{reminderStartAt:621,registeredStartAt:0}}},hud=createCamelRaceHUD({document,state,openModal:tab=>opened.push(tab)});
  hud.update();assert.equal(hud.root.hidden,true);assert.equal(camelRaceReminder(state.snapshot),0);
  state.snapshot.camelRace.reminderStartAt=620;hud.update();assert.equal(hud.root.hidden,false);assert.match(hud.root.textContent,/DUNE DASH STARTS IN 2:00/);hud.root.listeners.click();assert.deepEqual(opened,['race']);
  for(const field of ['modal','hasPlayed','connected']){const old=state[field];state[field]=field==='modal'?'teleport':false;hud.update();assert.equal(hud.root.hidden,true,field+' should hide the reminder');state[field]=old;}
  state.snapshot.time=601;hud.update();assert.match(hud.root.textContent,/starting grid/);assert.doesNotMatch(hud.root.textContent,/lollipop to enter/,'the closed current race must not advertise late entry');
  state.snapshot.time=620;hud.update();assert.equal(hud.root.hidden,true,'reminder ends at start, without a negative clock');
  state.snapshot.time=660;state.snapshot.camelRace.registeredStartAt=720;hud.update();assert.match(hud.root.textContent,/GATHER IN 0:40/);
  assert.equal(isCamelRaceIsland('camel_casual:open'),true);assert.equal(isCamelRaceIsland('friend'),false);hud.dispose();assert.ok(document.elements.every(el=>el.removed));
});

test('casual HUD handles host, guest, countdown and finish without scheduled rewards',async()=>{
  const [,,{createCamelRaceHUD}]=await modules,document=fakeRaceDocument(),state={connected:true,hasPlayed:true,snapshot:{time:100,me:{id:'one',island:'camel_casual:open'},camelRace:{inArena:true,mode:'casual',phase:'lobby',hostId:'one',entrantCount:2,entrants:[{id:'one',status:'racing',lap:0}],reminderStartAt:200}}},hud=createCamelRaceHUD({document,state,openModal(){}});
  hud.update();assert.match(hud.root.textContent,/YOUR LOBBY IS OPEN/);assert.match(hud.root.textContent,/E to start/);assert.match(hud.root.textContent,/Scheduled Dune Dash in 1:40/);
  state.snapshot.camelRace.hostId='other';hud.update();assert.match(hud.root.textContent,/WAITING FOR HOST/);
  Object.assign(state.snapshot.camelRace,{phase:'gathering',startAt:110});hud.update();assert.match(hud.root.textContent,/START IN 0:10/);
  Object.assign(state.snapshot.camelRace,{phase:'racing'});state.snapshot.camelRace.entrants[0].lap=1;hud.update();assert.match(hud.root.textContent,/PLAYFUL RACE · LAP 2\/3/);
  Object.assign(state.snapshot.camelRace,{phase:'finished',entrants:[{id:'one',status:'finished',rank:1,finishTime:89,coins:999,camelCoins:999}]});hud.update();assert.match(hud.root.textContent,/PLACE 1 · TIME 1:29/);assert.doesNotMatch(hud.root.textContent,/COINS|999/);hud.dispose();
});
