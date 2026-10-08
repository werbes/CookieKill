const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),{test}=require('node:test');
const root=path.resolve(__dirname,'..'),load=file=>import('data:text/javascript;base64,'+Buffer.from(fs.readFileSync(path.join(root,file),'utf8')).toString('base64'));
const modules=Promise.all([load('web/vendor/three.core.js'),load('web/bakery-scene.js')]);
function fixture(THREE,createBakeryScene){
 const parent=new THREE.Group(),building=new THREE.Group(),scenery=new Map([['plot',building]]);parent.add(building);
 const box=(w,h,d,color,x,y,z,group)=>{const m=new THREE.Mesh(new THREE.BoxGeometry(w,h,d),new THREE.MeshBasicMaterial({color}));m.position.set(x,y,z);group.add(m);return m;};
 const makePerson=(_name,_color,_peace,avatar)=>{const body=new THREE.Group();body.userData.avatar=avatar;for(const side of [-1,1]){box(.18,.65,.2,'#aaa',side*.16,.38,0,body);box(.2,.1,.3,'#aaa',side*.16,.1,0,body);box(.12,.6,.15,'#aaa',side*.35,1.1,0,body);}const sample=new THREE.Group();sample.name='sample-cookie';sample.position.set(.37,.97,-.2);body.add(sample);return body;};
 const scene=createBakeryScene({THREE,parent,box,makePerson,makeCookie:item=>{const g=new THREE.Group();g.userData.item=item;return g;},flatLabel:()=>new THREE.Group(),disposeObject:()=>{}});
 return {scene,building,scenery};
}

test('the visible display represents real item types and clears when stock or ownership disappears',async()=>{
 const [THREE,{createBakeryScene,bakeryDisplayItems}]=await modules;
 assert.deepEqual(bakeryDisplayItems({sugar:3,cake:1}),['cake','sugar','sugar','sugar']);assert.equal(bakeryDisplayItems({sugar:100}).length,12);assert.deepEqual(bakeryDisplayItems({sugar:0}),[]);
 const {scene,building,scenery}=fixture(THREE,createBakeryScene),node={id:'plot_display',kind:'bakery_display',plotId:'plot',owner:'owner',available:true,stock:{sugar:2,cake:1}},furniture={id:'counter',kind:'display',x:5,z:3,rotation:0},snapshot={me:{},nodes:[node,{id:'plot',kind:'bakery_plot',bakeryFurniture:[furniture]}]};
 scene.sync(snapshot,scenery);let display=building.getObjectByName('bakery-stock-display');assert.deepEqual(display.children.filter(o=>o.name.startsWith('stocked-')).map(o=>o.userData.item),['cake','sugar','sugar']);
 node.stock={};scene.sync(snapshot,scenery);display=building.getObjectByName('bakery-stock-display');assert.equal(display.children.filter(o=>o.name.startsWith('stocked-')).length,0);
 snapshot.nodes.push({id:'cashier',kind:'bakery_cashier',plotId:'plot',x:2.4,z:.95});scene.sync(snapshot,scenery);assert.ok(building.getObjectByName('bakery-till'));
 furniture.x=6;furniture.rotation=90;scene.sync(snapshot,scenery);display=building.getObjectByName('bakery-stock-display');assert.equal(display.position.x,6);assert.equal(display.position.z,3);assert.equal(display.rotation.y,Math.PI/2);
 snapshot.me.editingBakeryFurnitureId='counter';scene.sync(snapshot,scenery);assert.equal(display.visible,false);snapshot.me.editingBakeryFurnitureId='';scene.sync(snapshot,scenery);assert.equal(display.visible,true);
 node.owner='';scene.sync(snapshot,scenery);assert.equal(building.getObjectByName('bakery-stock-display'),undefined);scene.dispose();
});

test('customers walk toward authoritative positions and carry a bag only after an actual sale',async()=>{
 const [THREE,{createBakeryScene}]=await modules,{scene,scenery}=fixture(THREE,createBakeryScene),customer={id:'customer',kind:'bakery_customer',x:0,z:8,yaw:0,phase:'arriving',amount:3};
 const snapshot={me:{},nodes:[customer]};scene.sync(snapshot,scenery);const actor=scene.group.children[0];assert.equal(actor.getObjectByName('sample-cookie'),undefined);assert.equal(actor.userData.purchase.visible,false);
 customer.z=3;scene.sync(snapshot,scenery);scene.update(1,.1);assert.ok(actor.position.z>3&&actor.position.z<8);assert.notEqual(actor.userData.legs[0].rotation.x,0);
 customer.phase='waiting';scene.sync(snapshot,scenery);for(let i=0;i<20;i++)scene.update(2+i*.1,.1);assert.equal(actor.userData.legs[0].rotation.x,0);
 customer.phase='leaving';scene.sync(snapshot,scenery);assert.equal(actor.userData.purchase.visible,true);customer.amount=0;scene.sync(snapshot,scenery);assert.equal(actor.userData.purchase.visible,false);
 snapshot.nodes.push({id:'chef',kind:'bakery_chef',x:-1.6,z:-2.2,phase:'Baking Sugar cookie'});scene.sync(snapshot,scenery);scene.update(3,.1);const chef=scene.group.children.find(o=>o.userData.node.id==='chef');assert.equal(chef.userData.body.userData.avatar.hat,'chef');assert.notEqual(chef.userData.arms[0].rotation.x,0);
 snapshot.nodes[1].x=3;snapshot.nodes[1].phase='Walking to the mixer';scene.sync(snapshot,scenery);scene.update(3.2,.1);assert.notEqual(chef.userData.legs[0].rotation.x,0);assert.ok(chef.position.x>-1.6&&chef.position.x<3);
 snapshot.me.inCave=true;scene.sync(snapshot,scenery);assert.equal(scene.group.visible,false);snapshot.nodes=[];scene.sync(snapshot,scenery);assert.equal(scene.group.children.length,0);scene.dispose();
});
test('customer clicks raycast the visible person within reach and cannot pass through a bakery wall',async()=>{
 const [THREE,{createBakeryScene}]=await modules,{scene,scenery,building}=fixture(THREE,createBakeryScene),customer={id:'customer',kind:'bakery_customer',plotId:'plot',owner:'me',x:0,z:0,yaw:0,phase:'waiting',amount:3},snapshot={me:{id:'me',x:.16,z:3},nodes:[customer]},camera=new THREE.PerspectiveCamera(70,1,.1,100);camera.position.set(.16,.4,3);camera.lookAt(.16,.4,0);scene.sync(snapshot,scenery);
 assert.equal(scene.pickCustomer(camera,snapshot)?.id,'customer');camera.lookAt(3,.4,0);assert.equal(scene.pickCustomer(camera,snapshot),null);camera.lookAt(.16,.4,0);
 const wall=new THREE.Mesh(new THREE.BoxGeometry(3,3,.2),new THREE.MeshBasicMaterial());wall.position.set(0,1,1);building.add(wall);assert.equal(scene.pickCustomer(camera,snapshot),null);wall.visible=false;assert.equal(scene.pickCustomer(camera,snapshot)?.id,'customer');snapshot.me.z=4.1;assert.equal(scene.pickCustomer(camera,snapshot),null);snapshot.me.z=3;snapshot.me.inCave=true;assert.equal(scene.pickCustomer(camera,snapshot),null);scene.dispose();
});

test('server walks with actual baked goods on a tray and clears it after delivery',async()=>{
 const [THREE,{createBakeryScene}]=await modules,{scene,scenery}=fixture(THREE,createBakeryScene),server={id:'server',kind:'bakery_server',x:0,z:-5,yaw:0,phase:'Carrying fresh bakes to the display',item:'berry_cookie',amount:20},snapshot={me:{},nodes:[server]};
 scene.sync(snapshot,scenery);const actor=scene.group.children[0],tray=actor.getObjectByName('server-bake-tray');assert.equal(tray.visible,true);assert.equal(tray.children.filter(c=>c.name==='carried-berry_cookie').length,6);assert.equal(actor.userData.body.userData.avatar.shirtColor,'blue');
 server.z=3;scene.sync(snapshot,scenery);scene.update(1,.1);assert.ok(actor.position.z>-5&&actor.position.z<3);assert.equal(actor.userData.arms[0].rotation.x,.85);
 server.item='';server.amount=0;server.phase='Display stocked';scene.sync(snapshot,scenery);assert.equal(tray.visible,false);assert.equal(tray.children.length,1);scene.dispose();
});
