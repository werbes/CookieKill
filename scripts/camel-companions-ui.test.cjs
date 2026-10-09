const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),{test}=require('node:test');
const raceStateURL='data:text/javascript;base64,'+Buffer.from(fs.readFileSync(path.join(__dirname,'../web/camel-race-state.js'),'utf8')).toString('base64');
const load=file=>import('data:text/javascript;base64,'+Buffer.from(fs.readFileSync(path.join(__dirname,'..',file),'utf8').replaceAll('./camel-race-state.js',raceStateURL)).toString('base64'));
const modules=Promise.all([load('web/vendor/three.core.js'),load('web/camel-view.js'),load('web/camel-care-panels.js')]);

test('held food comes from the selected occupied hotbar slot and targets only a reachable own camel layer',async()=>{
  const [,{heldCamelTreat,heldTreatTarget}]=await modules;
  const s={me:{id:'one',x:0,y:0,z:0,selectedSlot:1,hotbar:[{item:'sugar',count:1},{item:'happy_apple',count:2}]},town:{treatCatalog:[{id:'happy_apple',name:'Happy apple'}]},nodes:[{id:'other',kind:'camel',owner:'two',x:1,z:0},{id:'upstairs',kind:'camel',owner:'one',x:1,y:8,z:0},{id:'underground',kind:'camel',owner:'one',x:1,z:0,inCave:true},{id:'mine',kind:'camel',owner:'one',x:3,z:0},{id:'far',kind:'camel',owner:'one',x:6,z:0}]};
  assert.equal(heldCamelTreat(s).id,'happy_apple');assert.equal(heldTreatTarget(s).id,'mine');
  s.me.hotbar[1].count=0;assert.equal(heldCamelTreat(s),null);assert.equal(heldTreatTarget(s),null);
  s.me.hotbar[1].count=1;s.me.island='camel_race';assert.equal(heldTreatTarget(s),null);
  s.me.island='';s.me.ridingCamel=true;assert.equal(heldTreatTarget(s),null);
});

test('camel care offers recall by identity with world, riding and following guards',async()=>{
  const [,,{createCamelCarePanels}]=await modules,buttons=[];
  const c={id:'c1',name:'Daisy',x:14,z:0,island:'home',speed:11,jump:7,stamina:120,happiness:80};
  const state={connected:true,snapshot:{me:{id:'one',x:0,z:0,island:'home',camels:[c]},town:{treatCatalog:[]}}};
  const panel=createCamelCarePanels({state,escapeHTML:String,button:(label,action,data,disabled)=>{buttons.push({label,action,data,disabled});return label;}});
  assert.match(panel.herd(),/Daisy/);assert.deepEqual(buttons[0],{label:'Call camel',action:'camel_call',data:{target:'c1'},disabled:false});
  c.following=true;buttons.length=0;assert.match(panel.herd(),/Running to you/);assert.equal(buttons[0].disabled,true);
  c.following=false;c.island='';buttons.length=0;panel.herd();assert.equal(buttons[0].disabled,true);
  c.island='home';state.snapshot.me.ridingCamel=true;state.snapshot.me.mountedCamelId='c1';buttons.length=0;panel.herd();assert.equal(buttons[0].disabled,true);
  state.snapshot.me.ridingCamel=false;state.connected=false;buttons.length=0;panel.herd();assert.equal(buttons[0].disabled,true);
});

test('rig animates running and eating independently and returns to a neutral pose',async()=>{
  const [THREE,{createCamelView,animateCamelView,createCamelFoodView}]=await modules;
  const mesh=(geometry,color,x,y,z,parent)=>{const m=new THREE.Mesh(geometry,new THREE.MeshBasicMaterial({color}));m.position.set(x,y,z);parent.add(m);return m;};
  const deps={THREE,sphere:(r,color,x,y,z,parent)=>mesh(new THREE.SphereGeometry(r,7,5),color,x,y,z,parent),box:(w,h,d,color,x,y,z,parent)=>mesh(new THREE.BoxGeometry(w,h,d),color,x,y,z,parent),cylinder:(a,b,h,color,x,y,z,parent)=>mesh(new THREE.CylinderGeometry(a,b,h,7),color,x,y,z,parent)};
  const camel=createCamelView(deps),rig=camel.userData.camelRig;assert.equal(rig.legs.length,4);
  animateCamelView(camel,{time:.15,moving:true});assert.ok(rig.legs[0].rotation.x>.4);assert.ok(rig.legs[1].rotation.x<-.4);
  animateCamelView(camel,{time:.15,eating:true});assert.equal(rig.legs[0].rotation.x,0);assert.ok(rig.head.rotation.x>.1);assert.notEqual(rig.jaw.position.y,-.14);
  animateCamelView(camel,{});assert.equal(rig.body.position.y,0);assert.equal(rig.head.rotation.x,0);assert.equal(rig.jaw.position.y,-.14);
  const shapes=['swift_dates','spring_carrot','stamina_oats','happy_apple'].map(id=>createCamelFoodView(deps,id));
  assert.equal(new Set(shapes.map(s=>s.name)).size,4);assert.ok(shapes.every(s=>s.children.length>=3));
});
