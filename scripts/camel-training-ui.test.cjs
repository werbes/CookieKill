const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),{test}=require('node:test');
const raceStateURL='data:text/javascript;base64,'+Buffer.from(fs.readFileSync(path.join(__dirname,'../web/camel-race-state.js'),'utf8')).toString('base64');
const load=file=>import('data:text/javascript;base64,'+Buffer.from(fs.readFileSync(path.join(__dirname,'..',file),'utf8').replaceAll('./camel-race-state.js',raceStateURL)).toString('base64'));
const modules=Promise.all([load('web/vendor/three.core.js'),load('web/camel-training-scene.js'),load('web/camel-training-panels.js'),load('web/camel-care-panels.js')]);
const courses=['speed','jump','stamina','happiness'].map((id,i)=>({id,name:id,centerX:-42+i*28,halfWidth:8,halfDepth:24,checkpoints:[{x:-34+i*28,z:24},{x:-50+i*28,z:24},{x:-50+i*28,z:-24},{x:-34+i*28,z:-24}],pointsPerGain:100,cap:100}));

test('pink portal and isolated four-course scenery follow authoritative gate state',async()=>{
  const [THREE,{buildCamelTrainingEntrance,createCamelTrainingScene}]=await modules,scene=new THREE.Scene();
  const portal=buildCamelTrainingEntrance({THREE,parent:scene,flatLabel:()=>new THREE.Group()});
  assert.deepEqual(portal.group.position.toArray(),[-231,0,14]);
  const training=createCamelTrainingScene({THREE,scene,flatLabel:()=>new THREE.Group()});
  assert.equal(training.group.visible,false);
  const snapshot={me:{island:'camel_training'},camelTraining:{exercise:'jump',checkpoint:2,courses}};training.sync(snapshot);
  assert.equal(training.group.visible,true);
  const gate=training.group.getObjectByName('camel-training-next-gate');assert.deepEqual(gate.position.toArray(),[-22,.1,-24]);
  for(const c of courses)assert.ok(training.group.getObjectByName('training-course-'+c.id));
  const hurdles=[];training.group.traverse(o=>{if(o.name==='training-hurdle')hurdles.push(o);});
  assert.equal(hurdles.length,3);assert.deepEqual(hurdles.map(h=>h.position.z),[-12,0,12]);
  assert.ok(hurdles.every(h=>h.position.x===-6&&h.position.y===.5));
  snapshot.me.island='';training.sync(snapshot);assert.equal(training.group.visible,false);training.dispose();assert.equal(training.group.parent,null);
});

test('training entry rejects rentals and distance; treat buttons reserve a complete work budget',async()=>{
  const [,,{createCamelTrainingPanels}]=await modules,buttons=[];
  const camel={id:'c1',name:'Daisy',speed:10,jump:7,stamina:130,happiness:75,training:{speed:50},trainingBoost:{speed:120}};
  const state={connected:true,snapshot:{me:{x:-231,z:14,camels:[camel,{id:'rental',rentalUntil:100}],inventory:{swift_dates:3}},camelTraining:{courses},town:{treatCatalog:[{id:'swift_dates',name:'Dates',stat:'speed',cap:16,work:120,maxWork:240}]}}};
  const panels=createCamelTrainingPanels({state,escapeHTML:String,button:(label,action,data,disabled)=>{buttons.push({label,action,data,disabled});return label;}});
  assert.match(panels.render(),/by itself never changes a stat/);assert.equal(buttons.length,1);assert.equal(buttons[0].disabled,false);assert.equal(buttons[0].data.item,'c1');
  state.snapshot.me.x=-215;buttons.length=0;panels.render();assert.equal(buttons[0].disabled,true);
  state.snapshot.me.island='camel_training';Object.assign(state.snapshot.camelTraining,{exercise:'speed',checkpoint:1,ownCamel:camel});
  buttons.length=0;assert.match(panels.render(),/50 \/ 100 training points/);assert.equal(buttons.find(b=>b.action==='feed_camel').disabled,false);
  camel.trainingBoost.speed=121;buttons.length=0;panels.render();assert.equal(buttons.find(b=>b.action==='feed_camel').disabled,true,'never offer a treat that cannot fully fit');
  camel.trainingBoost.speed=0;camel.speed=16;buttons.length=0;panels.render();assert.equal(buttons.find(b=>b.action==='feed_camel').disabled,true);
  assert.ok(buttons.some(b=>b.action==='training_leave'));
});

test('Friends feeds the active training camel without allowing a world recall',async()=>{
  const [,,,{createCamelCarePanels}]=await modules,buttons=[];
  const parked={id:'c1',name:'Daisy',x:200,z:-200,speed:10},active={...parked,speed:10.1,trainingBoost:{speed:140}};
  const state={connected:true,snapshot:{me:{x:-34,z:-24,island:'camel_training',camels:[parked],inventory:{swift_dates:2}},camelTraining:{ownCamel:active},town:{treatCatalog:[{id:'swift_dates',name:'Dates',stat:'speed',cap:16,work:120,maxWork:240}]}}};
  const panels=createCamelCarePanels({state,escapeHTML:String,button:(label,action,data,disabled)=>{buttons.push({action,disabled});return label;}});
  assert.match(panels.herd(),/Speed: 10.10/);assert.equal(buttons.find(b=>b.action==='camel_call').disabled,true);assert.equal(buttons.find(b=>b.action==='feed_camel').disabled,true);
  active.trainingBoost.speed=100;buttons.length=0;panels.herd();assert.equal(buttons.find(b=>b.action==='feed_camel').disabled,false);
});
