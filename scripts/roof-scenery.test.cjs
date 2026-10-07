const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {test}=require('node:test');
const root=path.resolve(__dirname,'..');
const url=file=>'data:text/javascript;base64,'+fs.readFileSync(path.join(root,file)).toString('base64');

test('bakery, city, desert and cabin roof caps are level and rest on their wall tops',async()=>{
  const [THREE,{buildFlatRoof}]=await Promise.all([import(url('web/vendor/three.core.js')),import(url('web/roof-scenery.js'))]);
  const box=(w,h,d,color,x,y,z,parent)=>{const mesh=new THREE.Mesh(new THREE.BoxGeometry(w,h,d),new THREE.MeshBasicMaterial({color}));mesh.position.set(x,y,z);parent.add(mesh);return mesh;};
  for(const [width,depth,wallTop] of [[20,16,5.8],[12,10,6],[8,7,4],[5.6,4.8,3.6]]){
    const parent=new THREE.Group(),roof=buildFlatRoof({box,parent},{width,depth,wallTop});parent.updateMatrixWorld(true);
    const bounds=new THREE.Box3().setFromObject(roof),epsilon=1e-6;
    assert.ok(Math.abs(bounds.min.y-wallTop)<epsilon,'roof must touch, not float above or sink into the walls');
    assert.ok(Math.abs(bounds.max.y-wallTop-.36)<epsilon);
    assert.ok(Math.abs(bounds.min.x+width/2+.18)<epsilon&&Math.abs(bounds.max.x-width/2-.18)<epsilon);
    assert.ok(Math.abs(bounds.min.z+depth/2+.18)<epsilon&&Math.abs(bounds.max.z-depth/2-.18)<epsilon);
    assert.deepEqual([roof.rotation.x,roof.rotation.y,roof.rotation.z],[0,0,0]);
    const p=roof.geometry.attributes.position,heights=new Set();for(let i=0;i<p.count;i++)heights.add(p.getY(i).toFixed(5));
    assert.equal(heights.size,2,'roof cap must have horizontal upper and lower planes');
  }
});
