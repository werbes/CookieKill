const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const {test} = require('node:test');

const root = path.resolve(__dirname, '..');
const importSource = file => import(`data:text/javascript;base64,${Buffer.from(fs.readFileSync(path.join(root, file), 'utf8')).toString('base64')}`);
// The bundled core contains the real Three.js geometry, meshes and raycaster;
// importing it directly keeps this test independent of a DOM and npm packages.
const modules = Promise.all([importSource('web/vendor/three.core.js'), importSource('web/cave-scenery.js')]);
const cross = (a, b, c) => (b[0]-a[0])*(c[1]-a[1])-(b[1]-a[1])*(c[0]-a[0]);

function overlapsOpening(triangle) {
  const opening = [[117,-88],[123,-88],[123,-78],[117,-78]];
  const axes = [[1,0],[0,1]];
  for (let i=0;i<3;i++) {
    const a=triangle[i], b=triangle[(i+1)%3];
    axes.push([-(b[1]-a[1]),b[0]-a[0]]);
  }
  // A separating axis detects positive-area overlap, including triangles with
  // every vertex outside the hole that nevertheless cut across its interior.
  return axes.every(axis => {
    if (Math.hypot(...axis)<1e-9) return true;
    const project = polygon => polygon.map(point => point[0]*axis[0]+point[1]*axis[1]);
    const a=project(triangle), b=project(opening);
    return Math.min(Math.max(...a),Math.max(...b))-Math.max(Math.min(...a),Math.min(...b)) > 1e-6;
  });
}

test('the hidden entrance aperture preserves continuous desert around the outcrop', async () => {
  const [THREE,{cutCaveOpening}] = await modules;
  for (const [xSegments,zSegments] of [[120,115],[37,31]]) {
    const geometry = new THREE.PlaneGeometry(480,460,xSegments,zSegments);
    geometry.rotateX(-Math.PI/2);
    geometry.translate(0,0,-30);
    const positions=geometry.attributes.position, colors=[];
    for (let i=0;i<positions.count;i++) colors.push((positions.getX(i)+240)/480,(positions.getZ(i)+260)/460,.5);
    geometry.setAttribute('color',new THREE.Float32BufferAttribute(colors,3));
    cutCaveOpening(geometry,THREE);
    assert.equal(geometry.index,null);
    const result=geometry.attributes.position;
    assert.equal(geometry.attributes.color.count,result.count);
    assert.equal(geometry.attributes.normal.count,result.count);
    let area=0;
    for (let i=0;i<result.count;i+=3) {
      const triangle=[0,1,2].map(j=>[result.getX(i+j),result.getZ(i+j)]);
      const triangleArea=Math.abs(cross(...triangle))/2;
      area+=triangleArea;
      if (triangleArea>1e-8) assert.equal(overlapsOpening(triangle),false,`terrain remains over the stair mouth: ${JSON.stringify(triangle)}`);
      for (let j=0;j<3;j++) {
        const x=result.getX(i+j), z=result.getZ(i+j);
        assert.ok(x>=-240.00001&&x<=240.00001&&z>=-260.00001&&z<=200.00001,'clipping moved terrain outside its original bounds');
        assert.ok(Math.abs(geometry.attributes.color.getX(i+j)-(x+240)/480)<2e-6,'clipped vertex lost interpolated terrain color');
        assert.ok(Math.abs(geometry.attributes.color.getY(i+j)-(z+260)/460)<2e-6,'clipped vertex lost interpolated terrain color');
      }
    }
    assert.ok(Math.abs(area-(480*460-60))<.001,`removed ${480*460-area} square metres instead of exactly 60`);
    const terrain=new THREE.Mesh(geometry,new THREE.MeshBasicMaterial({side:THREE.DoubleSide}));
    const ray=new THREE.Raycaster();
    const covered=(x,z)=>{
      ray.set(new THREE.Vector3(x,20,z),new THREE.Vector3(0,-1,0));
      return ray.intersectObject(terrain,false).length>0;
    };
    for (let i=0;i<40;i++) {
      const along=.075+i*.15;
      for (const [x,z] of [[117-.001,-88+along],[123+.001,-88+along],[117+along,-88-.001],[117+along,-78+.001]]) {
        assert.equal(covered(x,z),true,`terrain has a gap outside the exact stairwell at (${x},${z})`);
      }
    }
    for (const [x,z] of [[117.001,-83],[122.999,-83],[120,-87.999],[120,-78.001],[120,-83]]) {
      assert.equal(covered(x,z),false,`terrain covers the opening at (${x},${z})`);
    }
    terrain.material.dispose();
    geometry.dispose();
  }
});

test('rendered first and last stairs agree with the authoritative Go dimensions', async () => {
  const [THREE,{buildCave,caveHeight,CAVE_DEPTH,CAVE_STEPS,CAVE_RUN}] = await modules;
  const backend=fs.readFileSync(path.join(root,'internal/game/worldmap.go'),'utf8');
  const floor=Number(backend.match(/CaveFloorY\s*=\s*(-?[\d.]+)/)?.[1]);
  const count=Number(backend.match(/CaveStairCount\s*=\s*(\d+)/)?.[1]);
  const length=Number(backend.match(/CaveStairRun\s*=\s*([\d.]+)\s*\/\s*CaveStairCount/)?.[1]);
  assert.match(backend,/CaveStairRise\s*=\s*-CaveFloorY\s*\/\s*CaveStairCount/);
  assert.equal(CAVE_DEPTH,-floor);
  assert.ok(CAVE_DEPTH>=24,'the chamber must be substantially underground');
  assert.equal(CAVE_STEPS,count);
  assert.equal(CAVE_RUN,length/count);
  const parent=new THREE.Group();
  const mat=(color,options={})=>new THREE.MeshStandardMaterial({color,...options});
  const mesh=(geometry,color,x=0,y=0,z=0,group=parent)=>{
    const object=new THREE.Mesh(geometry,color?.isMaterial?color:mat(color));
    object.position.set(x,y,z);group.add(object);return object;
  };
  const box=(width,height,depth,color,x,y,z,group)=>mesh(new THREE.BoxGeometry(width,height,depth),color,x,y,z,group);
  const sphere=(radius,color,x,y,z,group)=>mesh(new THREE.IcosahedronGeometry(radius,0),color,x,y,z,group);
  const cylinder=(top,bottom,height,color,x,y,z,group,sides=7)=>mesh(new THREE.CylinderGeometry(top,bottom,height,sides),color,x,y,z,group);
  const groundRock={id:'cave_surface_rock_fixture',kind:'cave_rock',x:109.9,z:-74.5,width:4.6,depth:4.6,height:2.8,variant:2};
  const scene=buildCave({THREE,parent,layout:{props:[groundRock]},box,sphere,cylinder,mesh,mat,flatLabel:()=>new THREE.Group()});
  const treads=[];
  scene.traverse(object=>{
    const dimensions=object.geometry?.parameters;
    if (dimensions?.width===6&&dimensions.height===.25&&Math.abs(dimensions.depth-(CAVE_RUN+.006))<1e-9) treads.push(object);
  });
  assert.equal(treads.length,count);
  treads.sort((a,b)=>b.position.z-a.position.z);
  const top=tread=>tread.position.y+tread.geometry.parameters.height/2;
  assert.equal(top(treads[0]),floor/count,'first step must descend exactly one authoritative riser');
  assert.equal(top(treads.at(-1)),floor,'last step must meet the authoritative chamber floor');
  for (let i=0;i<count;i++) {
    assert.equal(top(treads[i]),(i+1)*floor/count);
    assert.equal(caveHeight(120,treads[i].position.z),top(treads[i]),'client height function disagrees with the visible tread');
  }
  assert.equal(caveHeight(120,-78),0);
  assert.equal(caveHeight(120,-115),floor);
  scene.updateMatrixWorld(true);
  const ray=new THREE.Raycaster();
  const firstHit=(x,y,z,dx,dy,dz)=>{
    ray.set(new THREE.Vector3(x,y,z),new THREE.Vector3(dx,dy,dz));
    return ray.intersectObject(scene,true)[0]?.distance;
  };
  assert.ok(firstHit(120,1.7,-70,0,0,-1)>9,'entrance must be an open horizontal passage, not a rock face');
  assert.ok(firstHit(112,1.7,-70,0,0,-1)<6,'left outcrop cheek must be solid');
  assert.ok(firstHit(128,1.7,-70,0,0,-1)<6,'right outcrop cheek must be solid');
  assert.ok(firstHit(120,6,-70,0,0,-1)<6,'rock lintel must cover the entrance');
  for(const direction of [-1,1]){
    const clearance=firstHit(120+direction*3.3,1.7,-76,direction,0,0);
    assert.ok(clearance>=.42&&clearance<.9,'the flared-mouth collision stop must leave the player capsule inside a nearby visible rock cheek');
  }
  for(const z of [-81,-89,-98,-106,-114])assert.ok(firstHit(120,caveHeight(120,z)+3.1,z,0,1,0)>1,'arched stairway must leave headroom above a camel rider');
  assert.ok(firstHit(120,floor+3.1,-117,0,1,0)>4,'chamber needs a tall, closed rock vault');
  const crownRocks=scene.children.filter(object=>object.name==='cave-crown-rock');
  assert.ok(crownRocks.length>=8,'substantial rock clusters must dress the crown');
  const crownShell=scene.children.filter(object=>object.name==='cave-outcrop');
  let silhouetteRocks=0;
  for(const rock of crownRocks){
    const bounds=new THREE.Box3().setFromObject(rock);
    ray.set(new THREE.Vector3(rock.position.x,20,rock.position.z),new THREE.Vector3(0,-1,0));
    const shellHit=ray.intersectObjects(crownShell,false)[0];
    assert.ok(shellHit,'crown rock must be attached to the actual outcrop');
    assert.ok(bounds.max.y>shellHit.point.y+.3,'roof decoration is buried beneath the visible crown');
    if(bounds.max.y>8.8)silhouetteRocks++;
  }
  assert.ok(silhouetteRocks>=3,'rock clusters must visibly break the upper silhouette');
  const boulder=scene.getObjectByName(groundRock.id),point=new THREE.Vector3(),boulderPositions=boulder.geometry.attributes.position;
  for(let i=0;i<boulderPositions.count;i++){
    point.fromBufferAttribute(boulderPositions,i).applyMatrix4(boulder.matrixWorld);
    assert.ok(Math.hypot(point.x-groundRock.x,point.z-groundRock.z)<=groundRock.width/2+1e-6,'ground boulder protrudes outside its authoritative collision footprint');
    assert.ok(point.y>=-1e-6&&point.y<=groundRock.height+1e-6,'ground boulder disagrees with its authoritative height');
  }
  const surfaceStones=scene.children.filter(object=>object.name==='cave-surface-stones');
  assert.ok(surfaceStones.length>0,'the roof needs visible stone across its surface');
  const regions=new Set(),matrix=new THREE.Matrix4(),vertex=new THREE.Vector3();
  for(const field of surfaceStones)for(let i=0;i<field.count;i++){
    field.getMatrixAt(i,matrix);
    const center=new THREE.Vector3().setFromMatrixPosition(matrix);
    regions.add(`${center.x<120?'west':'east'}-${center.z<-116?'back':'front'}`);
    const bounds=new THREE.Box3().makeEmpty(),positions=field.geometry.attributes.position;
    for(let j=0;j<positions.count;j++)bounds.expandByPoint(vertex.fromBufferAttribute(positions,j).applyMatrix4(matrix));
    assert.ok(bounds.max.y<.2,'decorative roof stones must remain shallow enough to walk over');
    assert.ok(bounds.max.x<=117||bounds.min.x>=123||bounds.max.z<=-88||bounds.min.z>=-78,'roof stones obstruct the open stairwell');
  }
  assert.ok(regions.size>=2,'scree must frame both sides of the outcrop');
  scene.traverse(object=>{object.geometry?.dispose();object.material?.dispose();});
});
