import {createMotionTrack} from './motion.js';

// Track and car poses come from the same server curve. Render only between
// confirmed car positions, on the clock also used by the rider's avatar.
export function createRollerCoasterScene({THREE,scene,flatLabel}){
  const group=new THREE.Group();group.name='desert-roller-coaster';scene.add(group);
  const trackGroup=new THREE.Group();trackGroup.name='coaster-track';group.add(trackGroup);
  const stationGroup=new THREE.Group();stationGroup.name='coaster-station';group.add(stationGroup);
  const carsGroup=new THREE.Group();carsGroup.name='coaster-cars';group.add(carsGroup);
  const materials=new Map(),cars=new Map(),pose=new THREE.Object3D(),up=new THREE.Vector3(0,1,0);
  let trackSignature='',selfID='',lastTime=0;
  const material=color=>{if(!materials.has(color))materials.set(color,new THREE.MeshStandardMaterial({color,roughness:.68,metalness:.12}));return materials.get(color);};
  function box(parent,name,w,h,d,color,x,y,z){const mesh=new THREE.Mesh(new THREE.BoxGeometry(w,h,d),material(color));mesh.name=name;mesh.position.set(x,y,z);mesh.castShadow=true;mesh.receiveShadow=true;parent.add(mesh);return mesh;}
  function label(parent,text,x,y,z,w,h){const mesh=flatLabel(text,'#fff1c9','#345c59',1024,128);mesh.position.set(x,y,z);mesh.scale.set(w,h,1);parent.add(mesh);return mesh;}
  function clearGeometry(parent){for(const child of [...parent.children]){parent.remove(child);child.traverse(o=>o.geometry?.dispose());}}
  function station(point){
    stationGroup.position.set(point.x,point.y||0,point.z);
    if(stationGroup.children.length)return;
    box(stationGroup,'coaster-platform',16,.12,12,'#d9bd85',0,-.05,-1);
    box(stationGroup,'coaster-boarding-path',3.4,.025,8,'#f8e6bc',0,.022,2);
    for(const x of [-5.4,5.4]){
      for(const z of [-.5,6.5])box(stationGroup,'station-post',.22,5.7,.22,'#538179',x,2.85,z);
      box(stationGroup,'station-awning',3.8,.18,8,'#e8b668',x,5.6,3);
      for(const z of [0,2.5,5]){
        box(stationGroup,'station-bench',2.4,.15,.62,'#956d4d',x,.66,z);
        for(const dx of [-.85,.85])box(stationGroup,'bench-leg',.14,.6,.35,'#5d7161',x+dx,.3,z);
      }
    }
    for(const x of [-3.1,3.1]){
      for(const z of [1,3,5])box(stationGroup,'queue-post',.12,1.05,.12,'#4f8078',x,.525,z);
      for(const y of [.5,.95])box(stationGroup,'queue-rail',.08,.08,4,'#dca35f',x,y,3);
    }
    box(stationGroup,'station-sign-beam',11,.88,.28,'#345c59',0,5.5,3.5);
    label(stationGroup,'DUNE DIPPER / DESERT COASTER',0,5.5,3.67,10,.76);
    label(stationGroup,'BOARD HERE / CAMELS WELCOME',0,2.75,1.9,4.5,.56);
    for(const x of [-6.8,6.8]){
      box(stationGroup,'station-lantern-post',.12,3,.12,'#886b49',x,1.5,4.5);
      box(stationGroup,'station-lantern',.34,.5,.34,'#ffe9a3',x,3.02,4.5);
    }
  }
  station({x:165,y:0,z:-161});

  function instancedSegments(name,segments,color,radius){
    const mesh=new THREE.InstancedMesh(new THREE.CylinderGeometry(1,1,1,7),material(color),segments.length);
    mesh.name=name;mesh.castShadow=true;mesh.receiveShadow=true;
    const delta=new THREE.Vector3();
    segments.forEach(([a,b],i)=>{delta.subVectors(b,a);pose.position.copy(a).add(b).multiplyScalar(.5);pose.quaternion.setFromUnitVectors(up,delta.clone().normalize());pose.scale.set(radius,delta.length(),radius);pose.updateMatrix();mesh.setMatrixAt(i,pose.matrix);});
    mesh.computeBoundingSphere();trackGroup.add(mesh);return mesh;
  }
  function buildTrack(samples){
    const signature=JSON.stringify(samples);if(signature===trackSignature)return;trackSignature=signature;clearGeometry(trackGroup);
    if(samples.length<2)return;
    const rails=[[],[]],supports=[],braces=[],ties=[],q=new THREE.Quaternion(),rotation=new THREE.Euler(0,0,0,'YXZ');
    for(let i=0;i<samples.length;i++){
      const p=samples[i];q.setFromEuler(rotation.set(p.pitch||0,p.yaw||0,p.bank||0));
      const center=new THREE.Vector3(p.x,p.y-.25,p.z),right=new THREE.Vector3(1,0,0).applyQuaternion(q);
      for(let side=0;side<2;side++)rails[side].push(center.clone().addScaledVector(right,side?1.35:-1.35));
      if(i<samples.length-1)ties.push({center,q:q.clone()});
      if(i%4===0&&i<samples.length-1){
        const tops=[center.clone().addScaledVector(right,-1.55),center.clone().addScaledVector(right,1.55)];
        for(const top of tops){const foot=top.clone();foot.y=.06;supports.push([foot,top]);}
        if(center.y>3){const foot=tops[0].clone();foot.y=.12;braces.push([foot,tops[1]]);}
      }
    }
    for(let side=0;side<2;side++)instancedSegments('coaster-rail-'+side,rails[side].slice(1).map((p,i)=>[rails[side][i],p]),'#d4844f',.115);
    instancedSegments('coaster-supports',supports,'#63928b',.16);
    instancedSegments('coaster-support-braces',braces,'#7aa59a',.085);
    const sleepers=new THREE.InstancedMesh(new THREE.BoxGeometry(3.65,.14,.3),material('#f2d59c'),ties.length);sleepers.name='coaster-track-ties';sleepers.castShadow=true;sleepers.receiveShadow=true;
    ties.forEach((tie,i)=>{pose.position.copy(tie.center);pose.quaternion.copy(tie.q);pose.scale.set(1,1,1);pose.updateMatrix();sleepers.setMatrixAt(i,pose.matrix);});sleepers.computeBoundingSphere();trackGroup.add(sleepers);
    group.userData.trackSamples=samples.length;
  }
  function createCar(id){
    const root=new THREE.Group();root.name='coaster-car-'+id;carsGroup.add(root);
    // The open deck holds either a standing rider or their full-sized camel.
    box(root,'coaster-car-floor',3.8,.22,5.6,'#435f5b',0,-.11,0);
    box(root,'coaster-car-floor-inlay',3.2,.02,4.8,'#dbb579',0,.012,0);
    for(const x of [-1.85,1.85]){
      box(root,'car-side-panel',.16,.65,5.5,'#d9895c',x,.32,0);
      box(root,'car-side-rail',.16,.12,5.5,'#ffe2a8',x,.86,0);
      for(const z of [-2.55,0,2.55])box(root,'car-rail-post',.12,.8,.12,'#ffe2a8',x,.42,z);
      for(const z of [-1.9,1.9]){
        const wheel=new THREE.Mesh(new THREE.CylinderGeometry(.29,.29,.18,12),material('#344d50'));wheel.name='coaster-car-wheel';wheel.rotation.z=Math.PI/2;wheel.position.set(x,-.31,z);root.add(wheel);
      }
    }
    box(root,'coaster-car-nose',3.8,.74,.32,'#dd9660',0,.32,-2.66);
    box(root,'coaster-car-front-rail',3.8,.12,.16,'#ffe2a8',0,.86,-2.72);
    box(root,'coaster-car-tail',3.8,.36,.16,'#d9895c',0,.15,2.72);
    const badge=box(root,'coaster-car-badge',1.15,.32,.05,'#3e7471',0,.36,-2.84);badge.rotation.z=-.06;
    const track=createMotionTrack();
    const tilt=createMotionTrack({teleportDistance:10});
    const car={root,track,tilt,pose:{},angles:{},playerId:''};cars.set(id,car);return car;
  }
  function sync(snapshot){
    group.visible=!snapshot.me?.island;if(!group.visible)return;
    const view=snapshot.rollerCoaster,time=(snapshot.time||0)*1000,reset=time<lastTime;selfID=snapshot.me?.id||'';lastTime=time;
    if(view?.station)station(view.station);if(view?.track)buildTrack(view.track);
    const active=new Set();
    for(const row of view?.cars||[]){
      if(!row.id||![row.x,row.y,row.z].every(Number.isFinite))continue;
      active.add(row.id);const car=cars.get(row.id)||createCar(row.id);car.playerId=row.playerId||'';
      car.root.userData.riderId=car.playerId;car.root.userData.camelId=row.camelId||'';car.root.userData.progress=row.progress||0;
      const key=row.id+'|'+row.startedAt;car.track.push(lastTime,row,{key,reset});car.tilt.push(lastTime,{x:row.pitch||0,y:row.bank||0,z:0},{key,reset});
      // Show a new car at its first confirmed location before the next frame.
      if(car.track.size===1)renderCar(car,lastTime);
    }
    for(const [id,car]of cars)if(!active.has(id)){clearGeometry(car.root);car.root.removeFromParent();cars.delete(id);}
  }
  function renderCar(car,time){
    const p=car.track.sample(time,car.pose),a=car.tilt.sample(time,car.angles);if(!p)return;
    p.pitch=a?.x||0;p.bank=a?.y||0;
    car.root.position.set(p.x,p.y,p.z);car.root.rotation.set(a?.x||0,p.yaw,a?.y||0,'YXZ');
  }
  function update(remoteTime=lastTime,localTime=remoteTime){if(group.visible)for(const car of cars.values())renderCar(car,car.playerId===selfID?localTime:remoteTime);}
  function poseForPlayer(id){if(!group.visible)return null;for(const car of cars.values())if(car.playerId===id)return car.pose;return null;}
  function dispose(){group.removeFromParent();group.traverse(o=>{o.geometry?.dispose();if(o.material?.map){o.material.map.dispose();o.material.dispose();}});for(const m of materials.values())m.dispose();cars.clear();}
  return {group,sync,update,poseForPlayer,dispose};
}
