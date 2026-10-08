// Shared low-poly primitives. Static scenery is instanced by shape and color.
export function activityArt(THREE,parent){
  const geometries={box:new THREE.BoxGeometry(1,1,1),ball:new THREE.IcosahedronGeometry(1,1),post:new THREE.CylinderGeometry(1,1,1,10),cone:new THREE.ConeGeometry(1,1,10)},materials=new Map(),staticMeshes=[];
  function shape(kind,color,x,y,z,sx,sy,sz,host=parent,dynamic=false){
    if(!materials.has(color))materials.set(color,new THREE.MeshStandardMaterial({color,roughness:.84,flatShading:true}));
    const object=new THREE.Mesh(geometries[kind],materials.get(color));object.position.set(x,y,z);object.scale.set(sx,sy,sz);object.receiveShadow=true;host.add(object);if(!dynamic)staticMeshes.push(object);return object;
  }
  const box=(w,h,d,c,x,y,z,host=parent,dynamic=false)=>shape('box',c,x,y,z,w,h,d,host,dynamic);
  const ball=(r,c,x,y,z,host=parent,dynamic=false)=>shape('ball',c,x,y,z,r,r,r,host,dynamic);
  const post=(r,h,c,x,y,z,host=parent,dynamic=false)=>shape('post',c,x,y,z,r,h,r,host,dynamic);
  function beam(a,b,r,color,host=parent){const start=new THREE.Vector3(...a),end=new THREE.Vector3(...b),d=end.clone().sub(start),middle=start.add(end).multiplyScalar(.5),o=post(r,d.length(),color,middle.x,middle.y,middle.z,host);o.quaternion.setFromUnitVectors(new THREE.Vector3(0,1,0),d.normalize());return o;}
  function batch(){
    parent.updateMatrixWorld(true);const inverse=parent.matrixWorld.clone().invert(),groups=new Map();
    for(const object of staticMeshes){const key=object.geometry.uuid+'|'+object.material.uuid;if(!groups.has(key))groups.set(key,[]);groups.get(key).push(object);}
    for(const objects of groups.values()){
      const first=objects[0],instance=new THREE.InstancedMesh(first.geometry,first.material,objects.length);instance.name='activity-static-'+first.geometry.type;instance.receiveShadow=true;
      objects.forEach((o,i)=>{instance.setMatrixAt(i,inverse.clone().multiply(o.matrixWorld));o.removeFromParent();});instance.computeBoundingSphere();parent.add(instance);
    }
    staticMeshes.length=0;
  }
  return {box,ball,post,beam,shape,batch};
}

export function activityLabel(group,flatLabel,text,x,y,z,width=4,height=.65,rotation=0,color='#fff0cb',background='#586747'){
  const label=flatLabel(text,color,background,768,128);label.position.set(x,y,z);label.rotation.y=rotation;label.scale.set(width,height,1);group.add(label);return label;
}

export function activityCookie(THREE,art,parent,name,color,x,y,z){
  const root=new THREE.Group();root.name=name;root.position.set(x,y,z);parent.add(root);
  const cookie=art.post(.66,.18,color,0,0,0,root,true);cookie.rotation.x=Math.PI/2;
  for(let i=0;i<7;i++){const angle=i*2.399,rad=.13+(i%3)*.14;const chip=art.ball(.085,i%2?'#725840':'#997750',Math.sin(angle)*rad,Math.cos(angle)*rad,.13,root,true);chip.scale.z*=.35;}
  const halo=new THREE.Mesh(new THREE.TorusGeometry(.87,.028,6,36),new THREE.MeshBasicMaterial({color,transparent:true,opacity:.65}));halo.name='cookie-portal-halo';root.add(halo);return root;
}

export function disposeActivity(group){
  const geometries=new Set(),materials=new Set(),textures=new Set();group.traverse(o=>{if(o.geometry)geometries.add(o.geometry);for(const m of Array.isArray(o.material)?o.material:o.material?[o.material]:[]){materials.add(m);if(m.map)textures.add(m.map);}});for(const t of textures)t.dispose();for(const g of geometries)g.dispose();for(const m of materials)m.dispose();group.removeFromParent();
}
