const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {test}=require('node:test');
const root=path.resolve(__dirname,'..');
const url=file=>'data:text/javascript;base64,'+fs.readFileSync(path.join(root,file)).toString('base64');
const roomSource=fs.readFileSync(path.join(root,'web/bakery-room.js'),'utf8').replace('./roof-scenery.js',url('web/roof-scenery.js'));
const modules=Promise.all([import(url('web/vendor/three.core.js')),import('data:text/javascript;base64,'+Buffer.from(roomSource).toString('base64'))]);

async function fixture(){
  const [THREE,{buildBakeryRoom,BAKERY_ROOM}]=await modules,parent=new THREE.Group();
  const mat=(color,extra={})=>new THREE.MeshStandardMaterial({color,...extra});
  const add=(geometry,color,x,y,z,target)=>{const m=new THREE.Mesh(geometry,mat(color));m.position.set(x,y,z);target.add(m);return m;};
  const box=(w,h,d,color,x,y,z,target)=>add(new THREE.BoxGeometry(w,h,d),color,x,y,z,target);
  const cylinder=(top,bottom,h,color,x,y,z,target,segments)=>add(new THREE.CylinderGeometry(top,bottom,h,segments),color,x,y,z,target);
  buildBakeryRoom({THREE,parent,box,cylinder,mat,flatLabel:()=>new THREE.Group()},{id:'plot',label:'My bakery'});
  parent.updateMatrixWorld(true);return {THREE,parent,BAKERY_ROOM};
}

test('bakery has a twenty-by-sixteen shell and independently recognizable kitchen and cafe floors',async()=>{
  const {THREE,parent,BAKERY_ROOM}=await fixture();
  assert.deepEqual(BAKERY_ROOM,{width:20,depth:16,height:5.8,partitionZ:-1,doorWidth:3.2,doorHeight:3.5});
  for(const [name,minZ,maxZ] of [['kitchen-floor',-8,-1],['cafe-floor',-1,8]]){
    const floor=parent.getObjectByName(name),bounds=new THREE.Box3().setFromObject(floor);
    assert.equal(bounds.min.x,-10);assert.equal(bounds.max.x,10);assert.equal(bounds.min.z,minZ);assert.equal(bounds.max.z,maxZ);
    assert.equal(floor.geometry.attributes.color.count,floor.geometry.attributes.position.count);
  }
  assert.ok(parent.userData.sign);assert.ok(parent.getObjectByName('bakery-window--1').material.transparent);
  const bounds=new THREE.Box3().setFromObject(parent.getObjectByName('bakery-ceiling'));
  assert.ok(Math.abs(bounds.min.y-5.8)<1e-6);
  assert.equal(parent.children.filter(o=>/oven|mixer|display|chair|table|pendant/.test(o.name)).length,0,'all furniture must come from editable owner layout');
});

test('visual entrance and partition match the authoritative clear doorway and solid walls',async()=>{
  const {THREE,parent}=await fixture(),ray=new THREE.Raycaster();
  const hit=(x,y,z,dx,dy,dz,far)=>{ray.set(new THREE.Vector3(x,y,z),new THREE.Vector3(dx,dy,dz));ray.far=far;return ray.intersectObject(parent,true).length>0;};
  for(const x of [-1.59,0,1.59]){
    assert.equal(hit(x,1.7,9,0,0,-1,15),false,'entrance and partition share a clear corridor');
    assert.equal(hit(x,3.4,9,0,0,-1,15),false,'doorway retains headroom');
  }
  for(const x of [-5,5])assert.equal(hit(x,1.7,1,0,0,-1,3),true,'partition sides stay solid');
  assert.equal(hit(0,3.7,1,0,0,-1,3),true,'partition lintel visible');
  assert.equal(hit(0,3.7,9,0,0,-1,2),true,'front lintel visible');
  assert.equal(hit(0,1.7,-7,0,0,-1,2),true,'rear wall visible');
  assert.equal(hit(9,1.7,3,1,0,0,2),true,'side wall visible');
  assert.equal(hit(0,1.7,3,0,1,0,5),true,'roof has a continuous ceiling');
});
