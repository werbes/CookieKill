const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),{test}=require('node:test');
const root=path.resolve(__dirname,'..'),load=file=>import('data:text/javascript;base64,'+Buffer.from(fs.readFileSync(path.join(root,file),'utf8')).toString('base64'));
const modules=Promise.all([load('web/vendor/three.core.js'),load('web/bakery-furnishings.js')]);
const catalog=[{id:'oven',width:2.2,depth:1.4,height:1.8,room:'kitchen',core:true},{id:'table',width:1.8,depth:1.4,height:.85,room:'cafe'},{id:'chair',width:.9,depth:.9,height:1.4,room:'cafe'},{id:'rug',width:3,depth:2,height:.04,room:'any',solid:false}];
function fixture(THREE,create){
 const material=color=>new THREE.MeshBasicMaterial({color}),box=(w,h,d,c,x,y,z,p)=>{const o=new THREE.Mesh(new THREE.BoxGeometry(w,h,d),material(c));o.position.set(x,y,z);p.add(o);return o;},cylinder=(a,b,h,c,x,y,z,p)=>{const o=new THREE.Mesh(new THREE.CylinderGeometry(a,b,h),material(c));o.position.set(x,y,z);p.add(o);return o;};
 const renderer=create({THREE,box,cylinder,sphere:(r,c,x,y,z,p)=>{const o=new THREE.Mesh(new THREE.SphereGeometry(r),material(c));o.position.set(x,y,z);p.add(o);return o;},flatLabel:()=>new THREE.Group(),disposeObject:()=>{}}),building=new THREE.Group();building.position.set(58,0,72);
 return {renderer,building,scenery:new Map([['plot',building]]),box};
}
test('bakery build scope and preview enforce room, door, overlap and rotated bounds',async()=>{
 const [,{insideOwnBakery,bakeryPlacementValid,bakeryFootprint}]=await modules,snapshot={me:{id:'me',bakeryPlot:'plot',x:58,z:72},nodes:[{id:'plot',kind:'bakery_plot',owner:'me',x:58,z:72}]};
 assert.equal(insideOwnBakery(snapshot).id,'plot');snapshot.me.island='me';assert.equal(insideOwnBakery(snapshot),null);snapshot.me.island='';snapshot.me.x=80;assert.equal(insideOwnBakery(snapshot),null);
 const table={id:'table1',kind:'table',x:-5,z:3,rotation:0};assert.equal(bakeryPlacementValid(table,catalog),true);assert.equal(bakeryPlacementValid({...table,z:-3},catalog),false);assert.equal(bakeryPlacementValid({...table,x:9},catalog),false);assert.equal(bakeryPlacementValid({...table,x:0,z:7},catalog),false);
 assert.equal(bakeryPlacementValid({...table,kind:'oven'},catalog),false);assert.equal(bakeryPlacementValid({...table,kind:'oven',z:-5},catalog),true);assert.equal(bakeryPlacementValid({...table,id:'table2'},catalog,[table]),false);assert.equal(bakeryPlacementValid(table,catalog,[table]),true);
 assert.equal(bakeryPlacementValid({kind:'rug',x:0,z:0},catalog),true);assert.ok(Math.abs(bakeryFootprint({...table,rotation:90},catalog).width-1.4)<.001);
});
test('persisted furniture moves between plots without stale meshes and picks stop at walls',async()=>{
 const [THREE,{createBakeryFurnishings}]=await modules,{renderer,building,scenery,box}=fixture(THREE,createBakeryFurnishings),furniture={id:'one',kind:'table',x:-5,z:3,rotation:90,color:'teal'},plot={id:'plot',kind:'bakery_plot',bakeryFurniture:[furniture]},snapshot={nodes:[plot],bakeryFurnitureCatalog:catalog};
 renderer.sync(snapshot,scenery);let view=building.getObjectByName('bakery-furniture-one');assert.equal(view.position.x,-5);assert.equal(view.rotation.y,Math.PI/2);assert.equal(view.children[0].material.color.getHexString(),'478b82');
 furniture.x=-4;renderer.sync(snapshot,scenery);assert.equal(view.parent,null);view=building.getObjectByName('bakery-furniture-one');assert.equal(view.position.x,-4);
 const camera=new THREE.PerspectiveCamera(65,1,.1,100);camera.position.set(54,.8,78);camera.lookAt(54,.8,75);assert.equal(renderer.pickObject(camera,'plot').id,'one');assert.equal(renderer.pickObject(camera,'other'),null);
 box(2,3,.2,'#aaa',-4,1.5,5,building);assert.equal(renderer.pickObject(camera,'plot'),null);
 renderer.setEditing('one');assert.equal(view.visible,false);renderer.setEditing('');assert.equal(view.visible,true);
 plot.bakeryFurniture=[];renderer.sync(snapshot,scenery);assert.equal(building.getObjectByName('bakery-furniture-one'),undefined);renderer.dispose();
});

test('display preview allows the second case, rejects a third, and permits moving either existing case',async()=>{
 const [,{bakeryPlacementValid}]=await modules,displayCatalog=[{id:'display',width:4.2,depth:1.1,room:'cafe',maxCount:2,core:false}],first={id:'first',kind:'display',x:-5,z:3},second={id:'second',kind:'display',x:5,z:3},third={id:'third',kind:'display',x:5,z:6};
 assert.equal(bakeryPlacementValid(second,displayCatalog,[first]),true);assert.equal(bakeryPlacementValid(third,displayCatalog,[first,second]),false);assert.equal(bakeryPlacementValid({...second,z:6},displayCatalog,[first,second]),true);
});
