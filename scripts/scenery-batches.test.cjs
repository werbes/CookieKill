const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {test}=require('node:test');
const source=file=>import(`data:text/javascript;base64,${Buffer.from(fs.readFileSync(path.join(__dirname,'..',file),'utf8')).toString('base64')}`);
const modules=Promise.all([source('web/vendor/three.core.js'),source('web/scenery-batches.js')]);

test('stationary plants share draw calls while preserving transformed vertices, colors and shadows',async()=>{
  const [THREE,{createSceneryBatches}]=await modules;
  const parent=new THREE.Group();parent.position.set(12,3,-20);parent.rotation.y=.2;
  const roots=[],expected=[];
  for(let i=0;i<8;i++){
    const root=new THREE.Group();root.position.set(2+i,0,5);root.rotation.y=i*.17;root.scale.set(1,.8+i*.1,1.3);parent.add(root);roots.push(root);
    const material=new THREE.MeshStandardMaterial({color:i%2?'#589654':'#846034',roughness:.9,flatShading:true});
    const child=new THREE.Mesh(new THREE.BoxGeometry(.5,2,.5),material);child.position.y=1;child.castShadow=true;child.receiveShadow=true;root.add(child);
  }
  parent.updateWorldMatrix(true,true);
  const inverse=parent.matrixWorld.clone().invert();
  for(const root of roots)for(const child of root.children){
    const transform=new THREE.Matrix4().multiplyMatrices(inverse,child.matrixWorld);
    const attribute=child.geometry.attributes.position;
    for(let i=0;i<attribute.count;i++)expected.push({point:new THREE.Vector3().fromBufferAttribute(attribute,i).applyMatrix4(transform),color:child.material.color});
  }
  const manager=createSceneryBatches(THREE,parent,roots);
  assert.equal(manager.batches.length,1,'different plant colors should use one vertex-colored material');
  const batch=manager.batches[0];assert.equal(batch.castShadow,true);assert.equal(batch.receiveShadow,true);
  const positions=batch.geometry.attributes.position,colors=batch.geometry.attributes.color;
  assert.equal(positions.count,expected.length);
  for(let i=0;i<expected.length;i++){
    assert.ok(new THREE.Vector3().fromBufferAttribute(positions,i).distanceTo(expected[i].point)<1e-5);
    assert.ok(Math.abs(colors.getX(i)-expected[i].color.r)<1e-6);
    assert.ok(Math.abs(colors.getY(i)-expected[i].color.g)<1e-6);
    assert.ok(Math.abs(colors.getZ(i)-expected[i].color.b)<1e-6);
  }
  manager.dispose();assert.equal(parent.children.filter(o=>o.isMesh).length,0);
});

test('chopping hides only one resource and regrowth restores its exact triangles',async()=>{
  const [THREE,{createSceneryBatches}]=await modules;
  const parent=new THREE.Group(),roots=[];
  for(let i=0;i<3;i++){
    const root=new THREE.Group();root.position.x=i*2;root.add(new THREE.Mesh(new THREE.BoxGeometry(),new THREE.MeshStandardMaterial()));parent.add(root);roots.push(root);
  }
  const manager=createSceneryBatches(THREE,parent,roots),index=manager.batches[0].geometry.index;
  const original=Array.from(index.array),perTree=original.length/3;
  manager.setVisible(roots[1],false);
  assert.deepEqual(Array.from(index.array.slice(0,perTree)),original.slice(0,perTree));
  assert.ok(index.array.slice(perTree,2*perTree).every(value=>value===0));
  assert.deepEqual(Array.from(index.array.slice(2*perTree)),original.slice(2*perTree));
  const version=index.version;manager.setVisible(roots[1],false);assert.equal(index.version,version,'unchanged snapshots must not upload geometry');
  manager.setVisible(roots[1],true);assert.deepEqual(Array.from(index.array),original);
});

test('spatial chunks remain independently cullable and transparent meshes remain separate',async()=>{
  const [THREE,{createSceneryBatches}]=await modules;
  const parent=new THREE.Group(),roots=[];
  for(const x of [2,90]){const root=new THREE.Group();root.position.x=x;root.add(new THREE.Mesh(new THREE.BoxGeometry(),new THREE.MeshStandardMaterial()));parent.add(root);roots.push(root);}
  const transparent=new THREE.Mesh(new THREE.BoxGeometry(),new THREE.MeshStandardMaterial({transparent:true,opacity:.5}));roots[0].add(transparent);
  const manager=createSceneryBatches(THREE,parent,roots);assert.equal(manager.batches.length,2);assert.equal(transparent.parent,roots[0]);
  for(const batch of manager.batches)assert.ok(batch.geometry.boundingSphere.radius<2);
});
