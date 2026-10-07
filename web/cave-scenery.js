export const CAVE_DEPTH=24, CAVE_STEPS=96, CAVE_RUN=37/CAVE_STEPS;
export const CAVE_MOUTH_Z=-75, CAVE_STAIR_TOP_Z=-78;
export const CAVE_SURFACE_COLORS=Object.freeze(['#b3865f','#ba8e66','#bf956e','#b78a63','#c09a76']);
export const caveHeight=(x,z)=>x>=117&&x<=123&&z<=CAVE_MOUTH_Z&&z>=-115?(-Math.min(CAVE_STEPS,Math.ceil(Math.max(0,(CAVE_STAIR_TOP_Z-z)/CAVE_RUN-1e-8)))*.25||0):x>=107&&x<=133&&z<=-113&&z>=-129?-CAVE_DEPTH:0;

// The opening lies entirely inside the sandstone outcrop. The visible entrance
// is horizontal; the first stairs start three metres behind the rock face.
export function cutCaveOpening(geometry,THREE){
  const position=geometry.attributes.position,color=geometry.attributes.color,index=geometry.index.array,vertices=[],colors=[];
  const planes=[v=>117-v[0],v=>v[0]-123,v=>v[2]+78,v=>-88-v[2]];
  const clip=(polygon,plane,positive)=>{
    const out=[];
    for(let i=0;i<polygon.length;i++){
      const a=polygon[i],b=polygon[(i+1)%polygon.length],da=plane(a),db=plane(b),ain=positive?da>=0:da<=0,bin=positive?db>=0:db<=0;
      if(ain)out.push(a);
      if(ain!==bin){const t=da/(da-db);out.push(a.map((n,j)=>n+(b[j]-n)*t));}
    }
    return out;
  };
  const emit=polygon=>{for(let j=1;j<polygon.length-1;j++)for(const v of [polygon[0],polygon[j],polygon[j+1]]){vertices.push(...v.slice(0,3));colors.push(...v.slice(3));}};
  for(let i=0;i<index.length;i+=3){
    let polygon=[index[i],index[i+1],index[i+2]].map(n=>[position.getX(n),position.getY(n),position.getZ(n),color.getX(n),color.getY(n),color.getZ(n)]);
    if(polygon.every(v=>v[0]<=117)||polygon.every(v=>v[0]>=123)||polygon.every(v=>v[2]>=-78)||polygon.every(v=>v[2]<=-88)){emit(polygon);continue;}
    for(const plane of planes){emit(clip(polygon,plane,true));polygon=clip(polygon,plane,false);if(!polygon.length)break;}
  }
  geometry.setIndex(null);geometry.setAttribute('position',new THREE.Float32BufferAttribute(vertices,3));geometry.setAttribute('color',new THREE.Float32BufferAttribute(colors,3));geometry.deleteAttribute('normal');geometry.computeVertexNormals();
}

export function buildCave({THREE,parent,layout,box,sphere,cylinder,mesh,mat,flatLabel}){
  const cave=new THREE.Group();cave.name='clay-cavern';parent.add(cave);
  const floor=-CAVE_DEPTH,lights=[];
  const stone=['#896247','#957052','#a77e58','#805c43'];
  const stoneMat=color=>mat(color,{roughness:1,side:THREE.DoubleSide});
  const rock=(x,y,z,s=1,index=0)=>{const r=mesh(new THREE.IcosahedronGeometry(s,1),stoneMat(stone[index%4]),x,y,z,cave);r.rotation.set(index*.21,index*1.27,index*.13);return r;};
  const slab=(w,h,d,color,x,y,z)=>{const r=box(w,h,d,color,x,y,z,cave);r.material=stoneMat(color);return r;};
  function surface(vertices,color,name='cave-stone'){
    const geometry=new THREE.BufferGeometry();geometry.setAttribute('position',new THREE.Float32BufferAttribute(vertices,3));geometry.computeVertexNormals();
    const object=mesh(geometry,stoneMat(color),0,0,0,cave);object.name=name;return object;
  }
  const quad=(a,b,c,d)=>[...a,...b,...c,...a,...c,...d];
  const noise=(x,y,z)=>Math.sin(x*2.31+y*1.73+z*3.17)*Math.cos(x*.91-z*1.61);

  // A watertight layered rock shell: broad irregular crown, solid cheeks and
  // back, and an open arch at eye level. No painted doorway or exposed pit.
  const sides=24,rings=16;
  const backingHeight=z=>5.05+2.75*Math.min(1,Math.max(0,(-74.5-z)/6));
  const outer=(i,z)=>{
    const a=i/sides*Math.PI,edge=Math.sin(a),bulge=1+.045*Math.sin((z+75)*.43);
    return [120-14*Math.cos(a)*bulge,Math.pow(edge,.65)*(backingHeight(z)+Math.sin(i*.8+z*.27)*.3),z];
  };
  const inner=i=>{const a=i/sides*Math.PI;return [120-4.4*Math.cos(a),3.8*Math.pow(Math.sin(a),.55),-74.65+Math.sin(i*1.9)*.12];};
  for(let r=0;r<rings;r++)for(let i=0;i<sides;i++){
    const z0=-74.5-r*18/rings,z1=-74.5-(r+1)*18/rings;
    surface(quad(outer(i,z0),outer(i,z1),outer(i+1,z1),outer(i+1,z0)),['#b88e68','#bc936e','#bf9873','#b78c66'][Math.floor(r/4)%4],'cave-outcrop');
  }
  for(let band=0;band<5;band++)for(let i=0;i<sides;i++){
    const at=(j,t)=>{const a=inner(j),b=outer(j,-74.5),ridge=Math.sin(j*1.2+t*7)*.15;return [a[0]+(b[0]-a[0])*t,a[1]+(b[1]-a[1])*t,a[2]+(b[2]-a[2])*t+Math.sin(t*Math.PI)*ridge];};
    surface(quad(at(i,band/5),at(i+1,band/5),at(i+1,(band+1)/5),at(i,(band+1)/5)),CAVE_SURFACE_COLORS[band],'cave-outcrop-face');
  }
  for(let i=0;i<sides;i++)surface([...outer(i,-92.5),...outer(i+1,-92.5),120,0,-92.5],'#ab7952','cave-outcrop-back');
  // Flat-bottomed, weathered chunks share a restrained sandstone palette. The
  // ground pieces fit their authoritative cylindrical footprints and heights.
  function sandstone(x,y,z,radius,height,index,name,depth=1){
    const geometry=new THREE.IcosahedronGeometry(1,1),positions=geometry.attributes.position;
    let bottom=Infinity,top=-Infinity;
    for(let i=0;i<positions.count;i++){
      const px=positions.getX(i),py=positions.getY(i),pz=positions.getZ(i),n=noise(px+index*.73,py,pz-index*.31);
      let dx=px*(.94+n*.055),dz=pz*(.94+n*.055);
      const reach=Math.hypot(dx,dz);if(reach>1){dx/=reach;dz/=reach;}
      const dy=Math.max(-.62,py*(.92+n*.06));bottom=Math.min(bottom,dy);top=Math.max(top,dy);
      positions.setXYZ(i,dx,dy,dz);
    }
    for(let i=0;i<positions.count;i++)positions.setY(i,(positions.getY(i)-bottom)/(top-bottom));
    const colors=[],warm=new THREE.Color(CAVE_SURFACE_COLORS[index%CAVE_SURFACE_COLORS.length]);
    for(let i=0;i<positions.count;i+=3){
      const elevation=(positions.getY(i)+positions.getY(i+1)+positions.getY(i+2))/3;
      const color=warm.clone().offsetHSL(0,0,(elevation-.45)*.035+Math.sin(i*.83+index)*.012);
      for(let j=0;j<3;j++)colors.push(color.r,color.g,color.b);
    }
    geometry.setAttribute('color',new THREE.Float32BufferAttribute(colors,3));geometry.computeVertexNormals();
    const b=mesh(geometry,mat('#ffffff',{vertexColors:true,roughness:1}),x,y,z,cave);b.name=name;
    b.scale.set(radius,height,radius*depth);b.rotation.y=index*1.19;return b;
  }
  const crownHeight=(x,z)=>{
    const bulge=1+.045*Math.sin((z+75)*.43),angle=Math.acos(Math.max(-1,Math.min(1,(120-x)/(14*bulge))));
    return Math.pow(Math.sin(angle),.65)*(backingHeight(z)+Math.sin(angle/Math.PI*sides*.8+z*.27)*.3);
  };
  // Overlapping boulders form the visible crown. The old continuous shell is
  // only a watertight backing beneath this pile, never an exposed smooth roof.
  // All chunks stay above the inaccessible outcrop and inside its footprint.
  const crownRows=[[-76.6,108.3,3.9,7],[-79.8,109.1,4.3,6],[-83.4,108.2,3.9,7],[-86.8,109.4,4.1,6],[-90.25,108.7,3.8,7]];
  let crownIndex=0;
  crownRows.forEach(([z,start,spacing,count],row)=>{
    for(let column=0;column<count;column++){
      const index=crownIndex++,x=start+column*spacing;
      const radius=Math.min(3.25+Math.sin(index*1.8)*.2,x-106,134-x,-74.5-z,z+92.5);
      const height=(row===0?3.9:2.5)+Math.sin(index*2.3)*.35,y=crownHeight(x,z)-height*.44;
      sandstone(x,y,z,radius,height,index,'cave-crown-rock',.93+(index%2)*.04).userData.pileLayer=0;
      if((row===1||row===3)&&column%2===0){
        // Upper chunks overlap the broad midsection of a lower supporting rock.
        sandstone(x+.35,y+height*.53,z-.25,radius*.66,1.65+(column%3)*.16,index+2,'cave-crown-rock',.88).userData.pileLayer=1;
      }
    }
  });
  // A few interlocking fracture shelves grow out of the cheeks. They are high
  // above the clear mouth and buried into the existing solid outcrop faces.
  for(const [i,p] of [[109.5,3.25,-74.8,2.2,1.4],[112.2,5.35,-74.9,1.85,1.2],[128.1,4.5,-74.8,2.35,1.1],[131,2.7,-74.7,2,1.65]].entries()){
    sandstone(p[0],p[1],p[2],p[3],p[4],i+1,'cave-fracture-ledge',.48);
    sandstone(p[0]+(i%2?.85:-.9),p[1]+p[4]*.55,p[2]-.12,p[3]*.58,p[4]*.5,i+3,'cave-fracture-ledge',.55);
  }
  for(const p of layout?.props||[])if(p.kind==='cave_rock'){
    sandstone(p.x,0,p.z,p.width/2,p.height,p.variant||0,p.id);
  }
  // Flat stone flakes and scree settle around the outcrop without closing its mouth.
  const stones=[];for(let i=0;i<120;i++){
    const x=102+(i*7.713)%36,z=-69-(i*3.137)%30;
    if(x>115.3&&x<124.7&&z>-94||x>106&&x<134&&z<-74.5&&z>-92.5)continue;
    stones.push({x,z,scale:.12+(i%5)*.08});
  }
  const pebbleGeometry=new THREE.IcosahedronGeometry(1,0),field=new THREE.InstancedMesh(pebbleGeometry,stoneMat('#bc956f'),stones.length),pose=new THREE.Object3D();
  field.name='cave-surface-stones';stones.forEach((p,i)=>{pose.position.set(p.x,.04,p.z);pose.rotation.set(0,i*2.4,0);pose.scale.set(p.scale,.05+(i%3)*.02,p.scale*.7);pose.updateMatrix();field.setMatrixAt(i,pose.matrix);});field.computeBoundingSphere();field.receiveShadow=true;cave.add(field);

  // A two-centimetre wearing surface avoids coplanar flicker with the sand.
  slab(8.8,.16,3.5,'#a5815d',120,-.06,-76.25);
  for(let i=0;i<CAVE_STEPS;i++){
    const z=CAVE_STAIR_TOP_Z-(i+.5)*CAVE_RUN,y=-(i+1)*.25;
    const step=slab(6,.25,CAVE_RUN+.006,'#a18059',120,y-.125,z);step.name='cave-tread';
    slab(6,.25,.018,'#94724e',120,y+.125,CAVE_STAIR_TOP_Z-i*CAVE_RUN-.005);
    slab(5.9,.014,.035,'#ad8a61',120,y+.007,CAVE_STAIR_TOP_Z-i*CAVE_RUN-.04);
  }
  // Continuous arched tunnel, with the same elevation profile as server stairs.
  const tunnelPoint=(i,z)=>{const a=i/sides*Math.PI,t=Math.min(1,Math.max(0,(-74.65-z)/3.35)),f=z>=CAVE_STAIR_TOP_Z?0:-Math.min(CAVE_DEPTH,(CAVE_STAIR_TOP_Z-z)/37*CAVE_DEPTH);return [120-(4.4-1.2*t)*Math.cos(a),f+(3.8+t)*Math.pow(Math.sin(a),.55),z+Math.sin(i*1.9)*.12*(1-t)];};
  for(let i=0;i<80;i++){
    const z0=-74.65-i*40.35/80,z1=-74.65-(i+1)*40.35/80;
    for(let j=0;j<sides;j++){
      const a=tunnelPoint(j,z0),b=tunnelPoint(j+1,z0),c=tunnelPoint(j+1,z1),d=tunnelPoint(j,z1);
      surface(quad(a,b,c,d),'#997b5c','cave-tunnel-shell');
    }
  }
  function roughWall(p){
    const geo=new THREE.BoxGeometry(p.width,p.height,p.depth,Math.ceil(p.width/1.1),9,Math.ceil(p.depth/1.1)),positions=geo.attributes.position;
    for(let i=0;i<positions.count;i++){
      const x=positions.getX(i),y=positions.getY(i),z=positions.getZ(i),n=noise(x+p.x,y+p.y,z+p.z);
      positions.setXYZ(i,x+n*.13,y+n*.15,z+n*.13);
    }
    const facets=geo.toNonIndexed();facets.computeVertexNormals();geo.dispose();
    const wall=mesh(facets,stoneMat('#947050'),p.x,p.y+p.height/2,p.z,cave);wall.name=p.id;
  }
  for(const p of layout?.props||[])if(p.kind==='cave_wall'&&p.id.startsWith('cave_wall_'))roughWall(p);
  slab(26.2,.5,16.2,'#856344',120,floor-.25,-121);
  for(let i=0;i<85;i++){
    const patch=mesh(new THREE.CircleGeometry(.3+(i%5)*.27,7),stoneMat(i%2?'#a36b3b':'#83502e'),107.6+(i*7.71)%24.8,floor+.012,-113.6-(i*3.13)%14.8,cave);patch.rotation.x=-Math.PI/2;patch.rotation.z=i;patch.scale.y=.55;
  }
  // A closed irregular vaulted ceiling rises above headroom and the camel rider.
  const vault=(x,z)=>floor+8.8+1.45*Math.sin((x-107)/26*Math.PI)*Math.sin((-113-z)/16*Math.PI)+noise(x,0,z)*.22;
  for(let x=107;x<133;x+=1.3)for(let z=-113;z>-129;z-=1){
    const x1=Math.min(133,x+1.3),z1=Math.max(-129,z-1);
    surface(quad([x,vault(x,z),z],[x1,vault(x1,z),z],[x1,vault(x1,z1),z1],[x,vault(x,z1),z1]),'#89694e','cave-vault');
  }
  // Continuous flowing deposits and hanging folds break every straight wall.
  for(let i=0;i<74;i++){
    const side=i%3,x=side===0?107.03:side===1?132.97:108+(i*3.73)%24,z=side===2?-129:-114-(i*1.57)%14.5;
    const b=rock(x,floor+1+(i*1.13)%6.8,z,1,i);b.scale.set(side===2?1.1:.34,.7+(i%4)*.18,side===2?.35:1.05);
  }
  function dripstone(x,y,z,radius,height,down,index){
    const profile=[];for(let ring=0;ring<=8;ring++){const t=ring/8,bulge=down?Math.pow(1-t,.78):Math.pow(1-t,.64);profile.push(new THREE.Vector2(radius*(.05+.95*bulge)*(1+.07*Math.sin(ring*2.6+index)),t*height));}
    const geo=new THREE.LatheGeometry(profile,7);geo.computeVertexNormals();
    const s=mesh(geo,stoneMat(stone[index%4]),x,y,z,cave);s.name=down?'cave-stalactite':'cave-stalagmite';if(down)s.rotation.z=Math.PI;return s;
  }
  for(const p of layout?.props||[])if(p.kind==='cave_stalagmite'){
    dripstone(p.x,p.y,p.z,p.width/2,p.height,false,p.variant);if(p.height>5)dripstone(p.x+.2,vault(p.x,p.z),p.z-.1,1.1,5.4,true,p.variant+1);
  }
  for(let i=0;i<62;i++){
    const x=107.5+(i*5.713)%25,z=-114-(i*3.137)%14.4,y=vault(x,z)-.05,h=.85+(i%7)*.36;
    dripstone(x,y,z,.18+(i%5)*.09,h,true,i);
    if(i%3===0)dripstone(x+.25,y-.12,z+.2,.12,h*.65,true,i+2);
  }
  const light=(color,intensity,distance,x,y,z)=>{const l=new THREE.PointLight(color,intensity,distance,2);l.position.set(x,y,z);cave.add(l);lights.push(l);return l;};
  function lamp(x,y,z,index){
    const pedestal=rock(x,y+.12,z,.23,index);pedestal.scale.y=.55;
    const ember=sphere(.09,'#ffd28e',x,y+.24,z,cave);ember.material=mat('#ffc27a',{emissive:'#ff8c28',emissiveIntensity:3});
  }
  for(const [i,z] of [-80,-87,-95,-103,-110].entries()){
    const x=120+(i%2?2.45:-2.45),y=caveHeight(120,z);lamp(x,y,z,i);light('#ffae59',22,11,x,y+.85,z);
  }
  for(const [i,p] of [[109,-119],[115,-126],[125,-119],[131,-126]].entries()){
    lamp(p[0],floor,p[1],i);light('#ffbd70',54,16,p[0],floor+1.25,p[1]);
  }
  light('#d98a53',10,18,120,floor+5,-122);
  for(const [x,color,text] of [[111,'#ed94b7','A HOME OF YOUR OWN'],[129,'#85dce8','FRIENDS ACROSS THE WATER']]){
    cylinder(.7,.85,.2,'#98623c',x,floor+.1,-124,cave,10);
    const plaque=flatLabel(text,color,'#3c251b',768,100);plaque.position.set(x,floor+2.5,-128.5);plaque.scale.set(5.1,.58,1);cave.add(plaque);
    for(let i=0;i<5;i++){const h=.3+(i%3)*.2,c=mesh(new THREE.ConeGeometry(.1,h,5),mat(color,{emissive:color,emissiveIntensity:.6}),x+(i-2)*.19,floor+h/2,-125.5,cave);c.rotation.z=(i-2)*.12;}
  }
  const entrance=flatLabel('CLAY GROTTO','#f4dfbb','#78523a',512,90);entrance.position.set(126.5,1,-71.8);entrance.scale.set(2.2,.39,1);entrance.rotation.y=-.2;cave.add(entrance);cylinder(.045,.055,.8,'#80664d',126.5,.4,-71.8,cave);
  cave.userData.lights=lights;
  return cave;
}
