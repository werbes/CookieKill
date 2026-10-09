const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),{test}=require('node:test');
const root=path.resolve(__dirname,'..'),url=source=>'data:text/javascript;base64,'+Buffer.from(source).toString('base64'),read=file=>fs.readFileSync(path.join(root,file),'utf8');
const motion=url(read('web/motion.js')),panelsURL=url(read('web/rodents-panels.js'));
const modules=Promise.all([import(url(read('web/vendor/three.core.js'))),import(url(read('web/rodents-scene.js').replace('./motion.js',motion))),import(panelsURL),import(url(read('web/nut-island-scene.js'))),import(url(read('web/nut-island-panels.js').replace('./rodents-panels.js',panelsURL)))]);
const species=['wood_mouse','house_mouse','rat','squirrel','cactus_mouse','blind_albino_rat','flying_squirrel','beaver','hamster','otter','chipmunk','mole_rat'];
const button=(label,action,data={},disabled=false)=>`<button data-action="${action}" data-payload='${JSON.stringify(data)}'${disabled?' disabled':''}>${label}</button>`;
const buttons=(html,action)=>Array.from(html.matchAll(/<button data-action="([^"]+)" data-payload='([^']+)'([^>]*)>(.*?)<\/button>/g)).filter(m=>m[1]===action).map(m=>({data:JSON.parse(m[2]),disabled:m[3].includes('disabled'),label:m[4]}));

test('Wildlife guide orders shuffled catalog by rarity without changing order within each tier or the snapshot',async()=>{
  const [,,{createRodentsPanels}]=await modules;
  const catalog=Object.freeze([
    {id:'otter',name:'Otter',rarity:'mythic'},
    {id:'beaver',name:'Beaver',rarity:'legendary'},
    {id:'rat',name:'Rat',rarity:'common'},
    {id:'flying_squirrel',name:'Flying squirrel',rarity:'rare'},
    {id:'squirrel',name:'Squirrel',rarity:'uncommon'},
    {id:'wood_mouse',name:'Wood mouse',rarity:'common'},
    {id:'chipmunk',name:'Chipmunk',rarity:'uncommon'},
    {id:'mole_rat',name:'Mole rat',rarity:'uncommon'}
  ]);
  const api=createRodentsPanels({state:{snapshot:{rodentCatalog:catalog}},escapeHTML:String});
  const headings=Array.from(api.render().matchAll(/<h3>[^ ]+ ([^<]+)<\/h3>/g),match=>match[1]);
  assert.deepEqual(headings,['Rat','Wood mouse','Squirrel','Chipmunk','Mole rat','Flying squirrel','Beaver','Otter']);
  assert.equal(catalog[0].id,'otter','rendering does not reorder the shared catalog');
});

test('all twelve small animal models have distinct body geometry/colors and meaningful species traits',async()=>{
  const [THREE,{createRodentModel,animateRodentModel}]=await modules,models=species.map(s=>createRodentModel(THREE,s)),signatures=[];
  for(const model of models){const mesh=model.userData.rodentRig.body.children.find(c=>c.isMesh);assert.ok(mesh.geometry.attributes.position.count>100);signatures.push(JSON.stringify([Array.from(mesh.geometry.attributes.position.array),Array.from(mesh.geometry.attributes.color.array)]));assert.equal(model.userData.rodentRig.limbs.length,4);}
  assert.equal(new Set(signatures).size,12,'each species has its own proportions and markings');
  const width=model=>new THREE.Box3().setFromObject(model).getSize(new THREE.Vector3()).x;
  assert.ok(width(models[2])>width(models[0])*1.3,'rat is visibly larger than a mouse');
  assert.ok(models[7].userData.parts.includes('front-tooth'));assert.ok(models[10].userData.parts.includes('cream-stripe'));assert.ok(models[11].userData.parts.includes('skin-fold'));
  const glider=models[6],wing=glider.userData.rodentRig.wings[0];assert.equal(glider.userData.rodentRig.wings.length,2);assert.notEqual(wing.rotation.z,0);
  animateRodentModel(glider,1,{state:'gliding',moving:true});assert.equal(wing.rotation.z,0);assert.notEqual(glider.userData.rodentRig.limbs[0].rotation.x,0);
  animateRodentModel(models[9],1,{state:'rainbow'});assert.equal(models[9].userData.rodentRig.halo.visible,true);
  animateRodentModel(models[8],1,{state:'burrowing'});assert.ok(models[8].userData.rodentRig.body.scale.y<1);
});

test('small wildlife interpolates authoritative movement, hides burrows, changes world and includes released pets',async()=>{
  const [THREE,{createRodentsScene}]=await modules,scene=new THREE.Scene(),api=createRodentsScene({THREE,scene});
  const state={time:1,me:{id:'one',island:''},rodents:[{id:'mouse',species:'wood_mouse',x:0,y:0,z:0,island:'',state:'wandering'}]};
  api.sync(state);api.update(1000);const first=api.group.getObjectByName('small-animal-mouse');assert.equal(first.position.x,0);
  state.time=1.1;state.rodents[0].x=1;api.sync(state);api.update(1050);assert.equal(first.position.x,.5);assert.notEqual(first.userData.rodentRig.limbs[0].rotation.x,0);
  state.rodents[0].state='hidden';api.sync(state);assert.equal(first.visible,false);
  state.me.island='nut:one';state.nutIsland={owner:'one',pets:[{id:'pet1',species:'beaver',x:16,y:0,z:18,yaw:1,cageId:'c1'}]};api.sync(state);api.update(1100);
  assert.equal(api.group.getObjectByName('small-animal-mouse'),undefined);const pet=api.group.getObjectByName('small-animal-pet1');assert.deepEqual(pet.position.toArray(),[16,0,18]);
  let disposed=0;pet.traverse(o=>{if(o.geometry)o.geometry.addEventListener('dispose',()=>disposed++);});state.nutIsland.pets=[];api.sync(state);assert.ok(disposed>0);assert.equal(api.group.children.length,0);
  api.dispose();assert.equal(scene.children.length,0);
});

test('H catches nearest reachable animal in the same world, honoring species radius and current states',async()=>{
  const [,,{nearestCatchable,rodentCaptureHint,createRodentCaptureHUD}]=await modules;
  const snapshot={me:{id:'one',x:0,y:0,z:0},rodents:[{id:'far',species:'rat',name:'Rat',x:4,y:0,z:0,captureRadius:5},{id:'cactus',species:'cactus_mouse',x:3.2,y:0,z:0,captureRadius:3},{id:'high',species:'flying_squirrel',x:0,y:7,z:0,state:'gliding',captureRadius:5},{id:'hidden',species:'beaver',x:.2,y:0,z:0,state:'diving',captureRadius:4},{id:'other',x:.1,z:0,island:'friend',captureRadius:5}]};
  assert.equal(nearestCatchable(snapshot).id,'far');assert.match(rodentCaptureHint(snapshot),/H.*Rat/);
  snapshot.rodents[2].y=3;assert.equal(nearestCatchable(snapshot).id,'high','low glider can be captured within genuine 3D reach');
  snapshot.me.fallen=true;assert.equal(nearestCatchable(snapshot),null);snapshot.me.fallen=false;snapshot.rollerCoaster={ride:{}};assert.equal(nearestCatchable(snapshot),null);delete snapshot.rollerCoaster;
  const elements=[],document={body:{appendChild:e=>elements.push(e)},createElement:()=>({style:{},addEventListener(){},remove(){this.removed=true;}})},actions=[],state={snapshot,connected:true,locked:true,hasPlayed:true};
  const hud=createRodentCaptureHUD({document,state,action:(...args)=>actions.push(args)});hud.update();assert.equal(hud.root.hidden,false);assert.equal(hud.catchNearest(),true);assert.deepEqual(actions[0],['catch_rodent',{target:'high'}]);
  snapshot.rodents=[{id:'otter',species:'otter',name:'Otter',rarity:'mythic',x:1,y:0,z:0,state:'wandering',captureRadius:5}];
  assert.equal(nearestCatchable(snapshot),null);assert.match(rodentCaptureHint(snapshot),/Nut crunch first/);hud.update();assert.equal(hud.root.disabled,true);assert.equal(hud.catchNearest(),false);
  snapshot.rodents[0].nutFed=true;assert.equal(nearestCatchable(snapshot),null);assert.match(rodentCaptureHint(snapshot),/Cosmic cake next/);
  snapshot.rodents[0].state='confused';assert.equal(nearestCatchable(snapshot).id,'otter');assert.match(rodentCaptureHint(snapshot),/H.*Catch Otter/);hud.update();assert.equal(hud.root.disabled,false);assert.equal(hud.catchNearest(),true);assert.deepEqual(actions[1],['catch_rodent',{target:'otter'}]);
  snapshot.rodents[0].nutFed=false;assert.equal(nearestCatchable(snapshot),null,'confused state alone does not skip the Nut crunch requirement');
  state.locked=false;hud.update();assert.equal(hud.root.hidden,true);assert.equal(hud.catchNearest(),false);hud.dispose();assert.equal(hud.root.removed,true);
});

test('rainbow effect is gentle, obeys reduced effects and ends without altering canvas filters',async()=>{
  const [,{createRodentRainbowEffect}]=await modules,document={body:{appendChild(){}},createElement:()=>({style:{},children:[],appendChild(child){this.children.push(child);},setAttribute(){},remove(){this.removed=true;}})},effect=createRodentRainbowEffect({document});
  effect.update({until:80,time:20});assert.equal(effect.layer.style.opacity,'0','event fades in from transparent');
  effect.update({until:80,time:21});assert.equal(Number(effect.layer.style.opacity),.5);
  effect.update({until:80,time:30});assert.equal(effect.layer.style.display,'block');assert.equal(effect.layer.children.length,2);assert.ok(Number(effect.tint.style.opacity)<.25);assert.match(effect.tint.style.filter,/hue-rotate/);
  assert.equal(effect.layer.style.pointerEvents,'none');assert.equal(effect.ribbons.style.pointerEvents,'none');assert.match(effect.ribbons.style.backgroundImage,/data:image\/svg\+xml/);assert.equal((decodeURIComponent(effect.ribbons.style.backgroundImage).match(/<path /g)||[]).length,14,'two visible seven-color ribbon arcs');
  const drift=effect.ribbons.style.transform;effect.update({until:80,time:31});assert.notEqual(effect.ribbons.style.transform,drift,'ribbons drift gently');
  effect.update({until:80,time:31,reduced:true});assert.equal(effect.tint.style.filter,'none');assert.equal(effect.ribbons.style.transform,'none');assert.ok(Number(effect.tint.style.opacity)<.15);assert.ok(Number(effect.ribbons.style.opacity)<.4);
  effect.update({until:80,time:78.5});assert.equal(Number(effect.layer.style.opacity),.5,'event fades out smoothly');
  effect.update({until:80,time:81});assert.equal(effect.layer.style.display,'none');effect.update({until:90,time:32,enabled:false});assert.equal(effect.layer.style.display,'none');effect.dispose();assert.equal(effect.layer.removed,true);
});

test('nut island scene keeps the arrival path open, uses full-sized cages and hides off-world',async()=>{
  const [THREE,,,,]=await modules,{createNutIslandScene}=await modules.then(m=>m[3]),scene=new THREE.Scene(),api=createNutIslandScene({THREE,scene,flatLabel:()=>new THREE.Group()});
  const snapshot={me:{id:'one',island:'nut:one'},nutIsland:{owner:'one',cages:[{id:'one',x:16,z:18},{id:'two',x:-16,z:18}]}};api.sync(snapshot);assert.equal(api.group.visible,true);assert.ok(api.group.getObjectByName('nut-island-return'));assert.ok(api.group.getObjectByName('nut-cage-workshop'));
  const cage=api.group.getObjectByName('nut-cage-one');assert.equal(cage.userData.width,7);assert.deepEqual(cage.position.toArray(),[16,0,18]);assert.ok(cage.getObjectByName('cage-mesh').isInstancedMesh);
  api.group.updateMatrixWorld(true);const ray=new THREE.Raycaster(new THREE.Vector3(0,1.7,29),new THREE.Vector3(0,0,-1),0,15);assert.equal(ray.intersectObject(api.group,true).length,0,'return path and arrival center remain visually clear');
  snapshot.nutIsland.cages.pop();api.sync(snapshot);assert.equal(api.group.getObjectByName('nut-cage-two'),undefined);snapshot.me.island='';api.sync(snapshot);assert.equal(api.group.visible,false);api.dispose();assert.equal(scene.children.length,0);
});

test('nut controls enforce paid unlock, workshop reach, held pets and four-animal cage capacity',async()=>{
  const [,,,,{createNutIslandPanels}]=await modules,state={connected:true,snapshot:{me:{id:'one',x:120,y:-24,z:-126,inCave:true,grounded:true,coins:999,inventory:{pet_rat:1}},rodentCatalog:[{id:'rat',name:'Rat'}]}};
  const api=createNutIslandPanels({state,button,escapeHTML:String,pretty:String}),me=state.snapshot.me;
  assert.equal(buttons(api.render(),'nut_island_unlock')[0].disabled,true);me.coins=1000;assert.equal(buttons(api.render(),'nut_island_unlock')[0].disabled,false);me.y=0;assert.equal(buttons(api.render(),'nut_island_unlock')[0].disabled,true);
  Object.assign(me,{island:'nut:one',inCave:false,nutIsland:{owner:'one'},x:4,y:0,z:22,coins:150});state.snapshot.nutIsland={owner:'one',cagePrice:150,maxCages:8,maxPets:100,cages:[{id:'c1',x:16,z:18,capacity:4}],pets:[]};
  assert.equal(buttons(api.render(),'nut_cage_buy')[0].disabled,false);me.x=12;assert.equal(buttons(api.render(),'nut_cage_buy')[0].disabled,true);
  let release=buttons(api.inventoryControls('pet_rat'),'rodent_release');assert.equal(release.length,2);assert.equal(release[0].disabled,false);assert.equal(release[1].disabled,true);
  me.x=12;me.z=18;release=buttons(api.inventoryControls('pet_rat'),'rodent_release');assert.equal(release[1].disabled,false);
  state.snapshot.nutIsland.pets=Array.from({length:4},(_,i)=>({id:'pet'+i,species:'rat',x:16,y:0,z:18,cageId:'c1'}));assert.equal(buttons(api.inventoryControls('pet_rat'),'rodent_release')[1].disabled,true);
  assert.equal(buttons(api.render(),'nut_pet_pack')[0].disabled,false,'caged pets can be reached from outside the solid cage');
  me.inventory.pet_rat=0;assert.equal(buttons(api.inventoryControls('pet_rat'),'rodent_release')[0].disabled,true);
  state.snapshot.rollerCoaster={ride:{}};assert.equal(buttons(api.travelControls(),'nut_island_leave')[0].disabled,true);
});
