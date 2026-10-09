export function createNutIslandScene({THREE,scene,flatLabel}){
  const group=new THREE.Group();group.name='nut-garden-island';group.visible=false;scene.add(group);
  const materials=new Map(),cages=new Map();
  const mat=color=>{if(!materials.has(color))materials.set(color,new THREE.MeshStandardMaterial({color,roughness:.95,flatShading:true}));return materials.get(color);};
  const mesh=(parent,geometry,color,x,y,z,name)=>{const object=new THREE.Mesh(geometry,mat(color));object.position.set(x,y,z);object.name=name||'';object.receiveShadow=true;object.castShadow=true;parent.add(object);return object;};
  const box=(parent,w,h,d,color,x,y,z,name)=>mesh(parent,new THREE.BoxGeometry(w,h,d),color,x,y,z,name);
  const cylinder=(parent,r1,r2,h,color,x,y,z,name)=>mesh(parent,new THREE.CylinderGeometry(r1,r2,h,12),color,x,y,z,name);
  function label(parent,text,x,y,z,w=5){if(!flatLabel)return;const sign=flatLabel(text,'#f8efd0','#41614c',768,128);sign.position.set(x,y,z);sign.scale.set(w,w/6,1);parent.add(sign);}
  cylinder(group,130,130,.12,'#92c6c4',0,-.83,0,'nut-island-water');
  cylinder(group,42,43,.9,'#e5d3a5',0,-.64,0,'nut-island-shore');
  cylinder(group,36.5,37.2,.36,'#9ebd84',0,-.21,0,'nut-island-ground');
  box(group,4.2,.025,40,'#d8c798',0,.001,7,'nut-island-main-path');
  box(group,34,.026,2.2,'#d8c798',0,.002,18,'nut-island-cage-path');
  for(const z of [-18,-6,6])box(group,34,.026,1.6,'#c8bc8d',0,.002,z,'nut-island-cage-path');
  function tree(x,z,index){
    const tree=new THREE.Group();tree.position.set(x,0,z);tree.name='nut-island-grove-tree';group.add(tree);
    cylinder(tree,.26,.39,3.3,'#88745b',0,1.65,0);
    const crown=mesh(tree,new THREE.IcosahedronGeometry(1,1),index%2?'#668b57':'#7ca166',0,4,0);crown.scale.set(2.4,2.1,2.3);
    for(const side of [-1,1]){const leaf=mesh(tree,new THREE.IcosahedronGeometry(1,1),index%2?'#8caa71':'#537f56',side*1.3,3.4,.35);leaf.scale.set(1.65,1.35,1.65);}
    for(let i=0;i<3;i++)mesh(tree,new THREE.IcosahedronGeometry(.13,0),'#bd9157',Math.sin(index+i*2)*1.3,2.7+Math.sin(i),Math.cos(index+i*2));
  }
  [[-27,-11],[-28,10],[27,-11],[28,10],[-7,-25],[0,-25],[7,-25],[-25,23],[25,23]].forEach(([x,z],i)=>tree(x,z,i));
  const decor=new THREE.Group();decor.name='nut-island-meadow';group.add(decor);
  const pose=new THREE.Object3D(),flowerCount=110,flowers=new THREE.InstancedMesh(new THREE.IcosahedronGeometry(.12,0),mat('#f1d699'),flowerCount);
  flowers.name='nut-island-wildflowers';for(let i=0;i<flowerCount;i++){const angle=i*2.399963,radius=29+(i%6)*.8;pose.position.set(Math.cos(angle)*radius,.12,Math.sin(angle)*radius);pose.scale.set(1,.65,1);pose.updateMatrix();flowers.setMatrixAt(i,pose.matrix);}flowers.computeBoundingSphere();decor.add(flowers);
  const shrubs=new THREE.InstancedMesh(new THREE.IcosahedronGeometry(1,0),mat('#7d9b69'),26);shrubs.name='nut-island-berry-shrubs';for(let i=0;i<26;i++){const angle=i*2.399963,radius=33;pose.position.set(Math.cos(angle)*radius,.35,Math.sin(angle)*radius);pose.scale.set(.6,.43,.58);pose.updateMatrix();shrubs.setMatrixAt(i,pose.matrix);}shrubs.computeBoundingSphere();decor.add(shrubs);
  const portal=new THREE.Group();portal.name='nut-island-return';portal.position.set(0,0,28);group.add(portal);
  cylinder(portal,1.65,1.75,.16,'#d4c39b',0,.08,0);cylinder(portal,1.27,1.27,.025,'#7dad89',0,.17,0);
  const ring=mesh(portal,new THREE.TorusGeometry(1.16,.075,6,32),'#d5f2b5',0,.2,0,'nut-return-ring');ring.rotation.x=Math.PI/2;
  for(const x of [-1.7,1.7])cylinder(portal,.1,.12,2.5,'#826a4e',x,1.25,-.2);
  label(portal,'RETURN TO THE CLAY GROTTO',0,2.35,-.18,5.2);
  const workshop=new THREE.Group();workshop.name='nut-cage-workshop';workshop.position.set(4,0,22);group.add(workshop);
  box(workshop,2.6,.18,1.3,'#ab8157',0,.95,0);for(const x of [-1.08,1.08])for(const z of [-.48,.48])box(workshop,.13,.92,.13,'#836746',x,.46,z);
  for(const x of [-1.3,1.3])box(workshop,.12,2.65,.12,'#836746',x,1.325,-.55);
  box(workshop,3,.14,2,'#92ab78',0,2.7,-.15);label(workshop,'CAGE WORKSHOP / 150 COINS',0,2.1,-.45,3.2);
  cylinder(workshop,.17,.2,.2,'#ddb987',-.65,1.1,0);box(workshop,.12,.06,.6,'#d2bd8e',.4,1.08,0);
  function cageModel(cage,index){
    const root=new THREE.Group();root.name='nut-cage-'+cage.id;root.userData.cageId=cage.id;root.userData.width=7;root.userData.depth=7;root.position.set(cage.x,0,cage.z);group.add(root);
    box(root,7,.13,7,'#dbc991',0,.015,0,'cage-floor');
    const wood=[],wire=[];
    for(const x of [-3.45,3.45])for(const z of [-3.45,3.45])wood.push([x,1.3,z,.16,2.6,.16]);
    for(const y of [.22,1.2,2.35]){for(const x of [-3.45,3.45])wood.push([x,y,0,.1,.12,7]);for(const z of [-3.45,3.45])wood.push([0,y,z,7,.12,.1]);}
    for(let i=-6;i<=6;i++){const at=i*.52;for(const x of [-3.45,3.45])wire.push([x,1.27,at,.022,2.15,.022]);for(const z of [-3.45,3.45])wire.push([at,1.27,z,.022,2.15,.022]);wire.push([at,2.4,0,.025,.025,7],[0,2.4,at,7,.025,.025]);}
    function batch(name,list,color){const m=new THREE.InstancedMesh(new THREE.BoxGeometry(1,1,1),mat(color),list.length);m.name=name;for(const [i,v] of list.entries()){pose.position.set(v[0],v[1],v[2]);pose.rotation.set(0,0,0);pose.scale.set(v[3],v[4],v[5]);pose.updateMatrix();m.setMatrixAt(i,pose.matrix);}m.computeBoundingSphere();root.add(m);}
    batch('cage-timber',wood,'#8e7958');batch('cage-mesh',wire,'#718976');
    box(root,1.3,.1,.1,'#a48a5c',0,1.95,3.5,'cage-gate');box(root,.1,1.8,.1,'#a48a5c',-.65,1.05,3.5);box(root,.1,1.8,.1,'#a48a5c',.65,1.05,3.5);
    box(root,.18,.1,.06,'#e8c57c',.4,1.1,3.56,'cage-latch');
    cylinder(root,.3,.35,.12,'#81adb3',2.4,.15,2.3,'cage-water-bowl');cylinder(root,.32,.4,.19,'#b99765',-2.3,.15,2.3,'cage-food-bowl');
    box(root,1.5,.55,1.2,'#ba9870',0,.35,-2.4,'cage-shelter');box(root,1.7,.12,1.4,'#739674',0,.67,-2.4);label(root,'COMPANION CAGE '+(index+1)+' / ROOM FOR FOUR',0,2.7,3.5,6);
    return root;
  }
  function clearCage(root){group.remove(root);root.traverse(o=>{o.geometry?.dispose();if(o.material?.map){o.material.map.dispose();o.material.dispose();}});}
  function sync(snapshot){group.visible=!!snapshot.nutIsland&&snapshot.me?.island==='nut:'+snapshot.nutIsland.owner;if(!group.visible)return;const seen=new Set();for(const [index,cage] of (snapshot.nutIsland.cages||[]).entries()){seen.add(cage.id);if(!cages.has(cage.id))cages.set(cage.id,cageModel(cage,index));else cages.get(cage.id).position.set(cage.x,0,cage.z);}for(const [id,model] of cages)if(!seen.has(id)){clearCage(model);cages.delete(id);}}
  function update(time){ring.material.emissive?.set('#b2e7b3');ring.material.emissiveIntensity=.08+Math.sin(time*.7)*.025;}
  function dispose(){scene.remove(group);const geometries=new Set(),ownedLabels=new Set();group.traverse(o=>{if(o.geometry)geometries.add(o.geometry);if(o.material?.map)ownedLabels.add(o.material);});geometries.forEach(g=>g.dispose());ownedLabels.forEach(m=>{m.map.dispose();m.dispose();});materials.forEach(m=>m.dispose());cages.clear();}
  return {group,sync,update,dispose};
}
