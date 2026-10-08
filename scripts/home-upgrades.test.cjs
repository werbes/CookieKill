const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),{test}=require('node:test');
const root=path.resolve(__dirname,'..'),url=source=>'data:text/javascript;base64,'+Buffer.from(source).toString('base64'),read=file=>fs.readFileSync(path.join(root,file),'utf8');
test('home expansion and pond panels show exact costs, limits, reach, and housing capacity',async()=>{
 const {createIslandBuildingPanels}=await import(url(read('web/island-building-panels.js')));
 const home={owner:'me',upgradeLevel:0,buildRadius:32,walkRadius:40,nextUpgradeCost:150,maxUpgrades:5,objects:[]},me={id:'me',island:'me',x:-5,z:24,coins:1000,camels:[]},state={connected:true,snapshot:{me,island:home,buildCatalog:[]}};
 const button=(name,action,data={},disabled)=>`<button data-action="${action}"${disabled?' disabled':''}>${name}</button>`;
 const panel=createIslandBuildingPanels({state,button,escapeHTML:String,pretty:String,shopRow:(_icon,name,description,buttons)=>name+description+buttons});
 assert.match(panel.upgrade(),/150 coins/);assert.match(panel.upgrade(),/32 → 37/);assert.doesNotMatch(panel.upgrade().match(/<button data-action="upgrade_island"[^>]*>/)[0],/disabled/);
 me.x=20;assert.match(panel.upgrade().match(/<button data-action="upgrade_island"[^>]*>/)[0],/disabled/);me.x=-5;home.upgradeLevel=5;assert.match(panel.upgrade(),/Fully expanded/);assert.doesNotMatch(panel.upgrade(),/data-action="upgrade_island"/);
 const pond={id:'pond',kind:'pond',x:0,z:0,pondCapacity:3,pondUpgradeCost:200,adoptedAnimals:['fish','turtle']};home.objects=[pond];state.context=pond;me.x=0;me.z=4.5;
 assert.match(panel.buildObject(),/2 \/ 3 rescued animals/);assert.match(panel.buildObject(),/7 × 6 to 8 × 7/);assert.match(panel.buildObject(),/200 coins/);
 pond.pondCapacity=4;assert.match(panel.buildObject(),/full four-animal capacity/);assert.doesNotMatch(panel.buildObject(),/data-action="upgrade_pond"/);
 const pen={id:'pen',kind:'camel_pen',x:0,z:0};home.objects=[pen];state.context=pen;me.camels=Array.from({length:4},(_,i)=>({id:'camel'+i,island:'me',x:0,z:0,homePenId:'pen'}));me.camels.push({id:'fifth',island:'me',x:0,z:0});
 const html=panel.buildObject();assert.match(html,/Home for 4 \/ 4 camels/);assert.match(html,/<button data-action="camel_house" disabled>/);assert.equal((html.match(/data-action="camel_unhouse"/g)||[]).length,4);
});
test('expanded island terrain, physical upgrade sign, four pond residents and hollow pen render',async()=>{
 const THREE=await import(url(read('web/vendor/three.core.js'))),camel=url(read('web/camel-view.js')),{createIslandScene}=await import(url(read('web/island-scene.js').replace('./camel-view.js',camel)));
 const scene=new THREE.Scene(),mat=color=>new THREE.MeshBasicMaterial({color}),mesh=(geometry,color,x,y,z,parent)=>{const m=new THREE.Mesh(geometry,typeof color==='string'?mat(color):color);m.position.set(x,y,z);parent.add(m);return m;},box=(w,h,d,c,x,y,z,p)=>mesh(new THREE.BoxGeometry(w,h,d),c,x,y,z,p),sphere=(r,c,x,y,z,p)=>mesh(new THREE.SphereGeometry(r,8,6),c,x,y,z,p),cylinder=(t,b,h,c,x,y,z,p,n=8)=>mesh(new THREE.CylinderGeometry(t,b,h,n),c,x,y,z,p);
 const api=createIslandScene({THREE,scene,mat,mesh,box,sphere,cylinder,textSprite:text=>{const g=new THREE.Group();g.userData.text=text;return g;},disposeObject:()=>{},makeAnimal:animal=>{const g=new THREE.Group();g.userData.species=animal.species;return g;}});
 const island={owner:'me',upgradeLevel:5,buildRadius:57,walkRadius:65,objects:[{id:'pond',kind:'pond',x:0,z:0,pondCapacity:4,adoptedAnimals:['fish','turtle','fish','turtle']},{id:'pen',kind:'camel_pen',x:10,z:10}]};
 api.sync({me:{id:'me',island:'me'},island,buildCatalog:[{id:'pond',width:5,depth:4,height:.25},{id:'camel_pen',width:8,depth:8,height:1.5}]});
 const terrain=scene.getObjectByName('home-island-terrain'),buildings=scene.getObjectByName('home-island-buildings');assert.equal(terrain.userData.walkRadius,65);assert.equal(terrain.userData.buildRadius,57);assert.ok(scene.getObjectByName('island-upgrade-sign'));
 const pond=buildings.children.find(o=>o.userData.buildObjectID==='pond');assert.equal(pond.userData.pondCapacity,4);assert.equal(pond.children.filter(o=>o.name.startsWith('pond-rescue-')).length,4);assert.equal(buildings.children.find(o=>o.userData.buildObjectID==='pen').userData.camelPen,true);
 api.sync({me:{id:'me',island:''},island:null});assert.equal(terrain.visible,false);assert.equal(buildings.visible,false);
});
