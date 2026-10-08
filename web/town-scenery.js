const FLOWERS={flower_daisy:'#f9eac2',flower_tulip:'#e89eb4',flower_lavender:'#aa91d2'};
const BOARDS=[['camel_coins','CAMEL COINS'],['play_time','TIME PLAYED'],['coins_earned','COINS EARNED'],['kills','MOST KILLS'],['cookie_ring_kills','RING KILLS']];

export function createTownScenery({THREE,parent,flatLabel}){
  const group=new THREE.Group();group.name='crumb-city-town';parent.add(group);
  const batches=new Map(),geometry={box:new THREE.BoxGeometry(1,1,1),ball:new THREE.SphereGeometry(1,8,6),post:new THREE.CylinderGeometry(1,1,1,8)};
  const material=new THREE.MeshStandardMaterial({roughness:.9,flatShading:true}),pose=new THREE.Object3D(),color=new THREE.Color();
  const flowers=new Map(),labels=new Map();
  function part(shape,shade,x,y,z,sx,sy,sz,ry=0){if(!batches.has(shape))batches.set(shape,[]);batches.get(shape).push({shade,x,y,z,sx,sy,sz,ry});}
  const box=(w,h,d,shade,x,y,z)=>part('box',shade,x,y,z,w,h,d);
  const ball=(r,shade,x,y,z,sx=1,sy=1,sz=1)=>part('ball',shade,x,y,z,r*sx,r*sy,r*sz);
  const post=(r,h,shade,x,y,z)=>part('post',shade,x,y,z,r,h,r);
  function label(text,x,y,z,w,h,shade='#514940',background='#f4e7cb',name=''){
    const object=flatLabel(text,shade,background,768,100);object.position.set(x,y,z);object.scale.set(w,h,1);object.name=name;group.add(object);return object;
  }
  function building(x,z,w,d,wall,roof,title,signColor){
    box(w,.1,d,'#dfd4ba',x,.025,z);box(w,4.8,.24,wall,x,2.4,z-d/2);
    for(const side of [-1,1]){
      box(.24,4.8,d,wall,x+side*w/2,2.4,z);
      const wing=(w-3.2)/2;box(wing,4.8,.24,wall,x+side*(1.6+wing/2),2.4,z+d/2);
      box(.18,3.3,.38,'#eee5d4',x+side*1.65,1.65,z+d/2+.05);
      box(2.8,1.6,.12,'#9fbeb9',x+side*(w/2-2),2,z+d/2+.15);
      box(2.95,.12,.2,'#fff1dc',x+side*(w/2-2),1.2,z+d/2+.22);
      box(.1,1.6,.2,'#fff1dc',x+side*(w/2-2),2,z+d/2+.22);
    }
    box(w+.6,.3,d+.6,roof,x,4.95,z);box(3.6,.2,.4,'#eee5d4',x,3.3,z+d/2+.05);
    label(title,x,4.05,z+d/2+.18,w-1.4,.7,signColor,'#fff0d8');
  }
  building(130,153,18,16,'#d5dccb','#647f77','CRUMB CITY RECORDS','#3c635a');
  building(168,153,14,12,'#ead4c9','#ad7f88','PETAL & POSY','#875566');
  building(198,153,16,12,'#d9dec6','#8b9673','PAWS & PALMS','#52643f');

  // Five real boards hang inside the records room, with clear walking aisles.
  BOARDS.forEach(([id,title],i)=>{
    const x=123.6+i*3.2;
    box(2.95,3.35,.18,'#8b7658',x,2.3,145.35);box(2.7,3.1,.2,'#314b49',x,2.3,145.46);
    label(title,x,3.5,145.59,2.48,.36,'#f5dfa5','#314b49','town-board-'+id);
    label(id==='coins_earned'?'Gambling & refunds excluded':'All saved players',x,.91,145.59,2.5,.23,'#c5d5c5','#314b49');
  });
  label('Five ways to leave your mark',130,4.28,145.28,9,.48,'#4b6759','#d5dccb');
  for(const x of [125,135]){box(2.5,.22,1,'#ab8a66',x,.68,157);for(const side of [-1,1])box(.15,.6,.8,'#715e48',x+side,.3,157);}

  // Striped canvas, overflowing pots, and a counter make Betty's shop easy to spot.
  for(let i=0;i<12;i++)box(1.1,.15,1.9,i%2?'#f8e4da':'#cf91a5',161.95+i*1.1,3.3,159.8);
  box(4,1,1,'#bc9278',168,.5,152);box(4.15,.14,1.12,'#ead9bc',168,1.07,152);
  label('FLOWERS FOR EVERY KIND OF DAY',168,3.65,147.18,10,.5,'#85596a','#f6e6ce');
  label('PICK A STEM / BETTY BUYS FLOWERS',149,1.25,157.8,8,.42,'#f4e7cb','#6a7f54');
  for(const x of [146,152])post(.045,1.3,'#8d7958',x,.65,157.8);
  function bloom(x,y,z,index=0,scale=1){
    const shade=Object.values(FLOWERS)[index%3],h=.65*scale;post(.024*scale,h,'#5c8051',x,y+h/2,z);
    ball(.12*scale,'#739258',x+.1*scale,y+h*.45,z,1.4,.25,.6);
    for(let p=0;p<6;p++){const a=p*Math.PI/3;ball(.12*scale,shade,x+Math.cos(a)*.13*scale,y+h,z+Math.sin(a)*.13*scale,1,.4,1);}
    ball(.07*scale,'#d8ac46',x,y+h+.025*scale,z);
  }
  for(const [i,x] of [162.5,173.5].entries()){
    post(.55,.7,'#c59477',x,.35,160);for(let j=0;j<5;j++)bloom(x+Math.sin(j*2.4)*.3,.7,160+Math.cos(j*2.4)*.3,i+j,1.1);
  }
  for(let i=0;i<5;i++)bloom(166.7+i*.65,1.14,152,i,.75);
  for(const x of [145,149,153])for(const z of [145,149,153])post(.72,.08,'#8f795b',x,.035,z);

  // Betty: silver bun, round spectacles, knitted cardigan and a flowered apron.
  const bx=171.8,bz=154.5;
  post(.36,.93,'#9c739d',bx,.57,bz);box(.48,.65,.43,'#e2b5b1',bx,1.24,bz);
  box(.34,.67,.08,'#f5dfbd',bx,1.04,bz+.26);
  ball(.26,'#e4bd9f',bx,1.77,bz,1,.99,.92);ball(.28,'#d3d0c4',bx,1.87,bz-.055,1,.73,1);
  ball(.16,'#d8d5cd',bx,2.06,bz-.11);ball(.25,'#e4bd9f',bx,1.75,bz+.045,1,.75,.88);
  for(const side of [-1,1]){
    box(.13,.15,.07,'#716154',bx+side*.115,1.8,bz+.25);box(.09,.1,.075,'#c9ddd5',bx+side*.115,1.8,bz+.29);
    ball(.055,'#dca199',bx+side*.18,1.65,bz+.2);box(.15,.56,.22,'#e2b5b1',bx+side*.34,1.18,bz);
    ball(.095,'#e4bd9f',bx+side*.34,.88,bz);box(.2,.15,.36,'#6d584d',bx+side*.17,.08,bz+.06);
  }
  box(.11,.023,.06,'#716154',bx,1.8,bz+.29);box(.09,.026,.045,'#ad7367',bx,1.65,bz+.29);
  for(let i=0;i<3;i++)ball(.034,'#d08cab',bx+(i-1)*.09,1.08+(i%2)*.16,bz+.315);
  label('BETTY / flower friend',bx,2.55,bz+.1,3.4,.37,'#805b72','#f9e7d2','town-betty-label');

  box(4,1,1,'#9baa7b',198,.5,152);box(4.15,.14,1.12,'#eee0bd',198,1.07,152);
  label('A LITTLE CARE GOES A LONG WAY',198,3.55,147.2,11,.45,'#5e7051','#edf0d5');
  ['DATES','CARROTS','OATS','APPLES'].forEach((name,i)=>{
    const x=193.4+i*3;box(2,.18,.8,'#9e8967',x,1.3,148.2);post(.32,.65,['#ad784d','#d4975f','#d0bd81','#adbf76'][i],x,1.7,148.2);
    post(.36,.1,'#eeddb9',x,2.07,148.2);label(name,x,2.6,148.3,2.6,.36,'#4d6146','#edf0d5');
  });
  // A little plush camel guards the shop window.
  box(1.3,.62,.5,'#caa16b',203,1.65,158.55);ball(.32,'#ddbb82',203,2,158.55);post(.18,.85,'#caa16b',202.36,2,158.55);
  ball(.24,'#dfbc84',202.3,2.43,158.55,1.3,.85,.8);ball(.035,'#534638',202.2,2.48,158.76);
  for(const x of [202.6,203.4])post(.09,.55,'#ad865c',x,1.12,158.55);

  for(const [shape,parts] of batches){
    const batch=new THREE.InstancedMesh(geometry[shape],material,parts.length);batch.name='town-static-'+shape;
    parts.forEach((p,i)=>{pose.position.set(p.x,p.y,p.z);pose.scale.set(p.sx,p.sy,p.sz);pose.rotation.set(0,p.ry,0);pose.updateMatrix();batch.setMatrixAt(i,pose.matrix);batch.setColorAt(i,color.set(p.shade));});
    batch.instanceMatrix.needsUpdate=true;batch.instanceColor.needsUpdate=true;batch.castShadow=true;batch.receiveShadow=true;group.add(batch);
  }
  function disposeObject(object){const geometries=new Set(),materials=new Set();object.traverse(o=>{if(o.geometry)geometries.add(o.geometry);for(const m of Array.isArray(o.material)?o.material:o.material?[o.material]:[])materials.add(m);});for(const g of geometries)g.dispose();for(const m of materials){m.map?.dispose();m.dispose();}}
  function rankText(board,entry){if(!entry)return '—';const value=board.unit==='seconds'?Math.floor(entry.value/3600)+'h '+Math.floor(entry.value%3600/60)+'m':Number(entry.value).toLocaleString('en-US');return '#'+entry.rank+' '+String(entry.name).slice(0,14)+' · '+value;}
  function update(snapshot){
    group.visible=!snapshot?.me?.island&&!snapshot?.me?.inCave;
    const present=new Set();
    for(const node of snapshot?.nodes||[]){
      if(!FLOWERS[node.kind])continue;present.add(node.id);let flower=flowers.get(node.id);
      if(!flower){
        flower=new THREE.Group();flower.name='harvest-flower-'+node.id;group.add(flower);flowers.set(node.id,flower);
        const stem=new THREE.Mesh(new THREE.CylinderGeometry(.022,.025,.75,5),new THREE.MeshStandardMaterial({color:'#557c49'}));stem.position.y=.38;flower.add(stem);
        const petals=new THREE.InstancedMesh(new THREE.SphereGeometry(.14,7,5),new THREE.MeshStandardMaterial({color:FLOWERS[node.kind]}),6);
        for(let i=0;i<6;i++){const a=i*Math.PI/3;pose.position.set(Math.cos(a)*.14,.78,Math.sin(a)*.14);pose.scale.set(1,.45,1);pose.rotation.set(0,a,0);pose.updateMatrix();petals.setMatrixAt(i,pose.matrix);}petals.instanceMatrix.needsUpdate=true;flower.add(petals);
        const center=new THREE.Mesh(new THREE.SphereGeometry(.08,7,5),new THREE.MeshStandardMaterial({color:'#d6a63f'}));center.position.y=.81;flower.add(center);
      }
      flower.position.set(node.x,node.y||0,node.z);flower.visible=node.available!==false;
    }
    for(const [id,flower] of flowers)if(!present.has(id))flower.visible=false;
    for(const [index,[id]] of BOARDS.entries()){
      const board=snapshot?.town?.leaderboards?.find(board=>board.id===id);if(!board)continue;
      for(let row=0;row<3;row++){
        const key=id+'-'+row,text=board.entries.length?rankText(board,board.entries[row]):row===0?'Your story starts here':'—';
        if(labels.get(key)?.userData.text===text)continue;
        const old=labels.get(key);if(old){group.remove(old);disposeObject(old);}
        const line=label(text,123.6+index*3.2,2.8-row*.58,145.6,2.5,.36,'#e6ddbc','#314b49','town-rank-'+key);line.userData.text=text;labels.set(key,line);
      }
    }
  }
  return {group,update,dispose(){parent.remove(group);disposeObject(group);flowers.clear();labels.clear();}};
}
