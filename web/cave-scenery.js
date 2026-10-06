export const CAVE_DEPTH = 10;
export const caveHeight = (x,z) => x>=117&&x<=123&&z<=-98&&z>=-115 ? -CAVE_DEPTH*(-98-z)/17 : x>=107&&x<=133&&z<=-113&&z>=-129 ? -CAVE_DEPTH : 0;

// A clay grotto with a walkable ramp and a rock vault below the desert surface.
export function buildCave({THREE,parent,layout,box,sphere,cylinder,mesh,mat,flatLabel}) {
  const cave=new THREE.Group();parent.add(cave);
  const stone=['#a1765d','#946b57','#b38d70','#876856'],clay='#b98264';
  const stoneMat=color=>mat(color,{emissive:'#6f4030',emissiveIntensity:.13});
  const rock=(x,y,z,s=1,index=0)=>{
    const r=mesh(new THREE.IcosahedronGeometry(s,1),stoneMat(stone[index%stone.length]),x,y,z,cave);
    r.rotation.set(index*.21,index*.73,index*.11);return r;
  };
  function floor(w,d,x,y,z,color) {
    const tile=mesh(new THREE.PlaneGeometry(w,d),stoneMat(color),x,y,z,cave);tile.rotation.x=-Math.PI/2;return tile;
  }
  function rockWall(p) {
    const geometry=new THREE.BoxGeometry(p.width,p.height,p.depth,Math.ceil(p.width/1.1),5,Math.ceil(p.depth/1.1));
    const positions=geometry.attributes.position;
    // Shared corner noise keeps the facets joined, including across box faces.
    for(let i=0;i<positions.count;i++) {
      const x=positions.getX(i),y=positions.getY(i),z=positions.getZ(i);
      const noise=Math.sin(x*7.3+y*11.9+z*4.7+p.x)*.12;
      const fork=p.width===3&&p.depth===8;
      const bevelX=fork?x*(1-.13*Math.pow(Math.abs(z)/4,6))*(1+.035*Math.sin(y*2)):x;
      const bevelZ=fork?z*(1-.075*Math.pow(Math.abs(x)/1.5,4)):z;
      positions.setXYZ(i,bevelX+noise,y+noise*.45,bevelZ+noise);
    }
    const facets=geometry.toNonIndexed(),colors=[];
    for(let i=0;i<facets.attributes.position.count;i+=3) {
      const color=new THREE.Color('#a77d62').multiplyScalar(.96+Math.sin(i*7.123+p.x)*.045);
      for(let j=0;j<3;j++)colors.push(color.r,color.g,color.b);
    }
    facets.setAttribute('color',new THREE.Float32BufferAttribute(colors,3));facets.computeVertexNormals();geometry.dispose();
    return mesh(facets,mat('#ffffff',{vertexColors:true,flatShading:true,emissive:'#6f4030',emissiveIntensity:.13}),p.x,p.y+p.height/2,p.z,cave);
  }
  floor(8,4,120,.01,-96,'#d4ae77');
  const ramp=box(6,.18,Math.hypot(17,CAVE_DEPTH),clay,120,-CAVE_DEPTH/2-.09,-106.5,cave);
  ramp.rotation.x=-Math.atan2(CAVE_DEPTH,17);ramp.material=stoneMat(clay);
  floor(26,16,120,-CAVE_DEPTH,-121,clay);
  // Fine, irregular clay patches avoid a flat rectangular floor without changing its height.
  for(let i=0;i<52;i++) {
    const x=108+(i*7.71)%24,z=-114-(i*3.13)%14;
    const patch=mesh(new THREE.CircleGeometry(.2+(i%5)*.16,5),stoneMat(i%2?'#bd8c6c':'#ad795d'),x,-CAVE_DEPTH+.012,z,cave);
    patch.rotation.x=-Math.PI/2;patch.rotation.z=i;patch.scale.y=.65;
  }
  for(const p of layout?.props||[])if(p.kind==='cave_wall') {
    // The continuous vault supplies the ramp walls; its collision supports stay hidden.
    if(p.z>-113)continue;
    rockWall(p);
  }
  // Connected, sloping facets form the tunnel vault rather than an open trench.
  const profile=[[-3.05,0],[-3.05,2.9],[-2.3,4.1],[-1.15,4.65],[0,4.85],[1.15,4.65],[2.3,4.1],[3.05,2.9],[3.05,0]];
  for(let segment=0;segment<8;segment++)for(let side=0;side<profile.length-1;side++) {
    const t0=segment/8,t1=(segment+1)/8,a=profile[side],b=profile[side+1];
    const vertices=[120+a[0],a[1]-CAVE_DEPTH*t0,-98-17*t0,120+b[0],b[1]-CAVE_DEPTH*t0,-98-17*t0,120+a[0],a[1]-CAVE_DEPTH*t1,-98-17*t1,120+b[0],b[1]-CAVE_DEPTH*t1,-98-17*t1];
    const geom=new THREE.BufferGeometry();geom.setAttribute('position',new THREE.Float32BufferAttribute(vertices,3));geom.setIndex([0,2,1,1,2,3]);geom.computeVertexNormals();
    mesh(geom,mat(stone[(side+segment)%stone.length],{side:THREE.DoubleSide,emissive:'#6f4030',emissiveIntensity:.13}),0,0,0,cave);
  }
  const roof=box(27,.8,16.3,stone[1],120,-5,-121,cave);roof.material=stoneMat(stone[1]);
  // Layered stone along the chamber edges and the fork keeps the clear walking lanes intact.
  for(let i=0;i<11;i++)for(const sign of [-1,1]) {
    const r=rock(120+sign*12.85,-9.65,-114.4-i*1.27,.6+(i%3)*.13,i);r.rotation.set(0,0,0);r.scale.set(.65,.75,1.45);
  }
  for(let i=0;i<15;i++) {
    const r=rock(108+i*1.68,-9.65,-128.7,.7,i+1);r.rotation.set(0,0,0);r.scale.set(1.25,.75,.5);
  }
  for(const x of [110,113,127,130])for(let i=0;i<3;i++) {
    const stalactite=mesh(new THREE.ConeGeometry(.18+(i%2)*.12,.55+i*.18,6),stoneMat(stone[i]),x,-5.65,-116-i*4,cave);stalactite.rotation.z=Math.PI;
  }
  function crystal(x,z,color) {
    for(let i=0;i<5;i++) {
      const h=.45+(i%3)*.24,crystal=mesh(new THREE.ConeGeometry(.15,h,5),mat(color,{emissive:color,emissiveIntensity:.35,roughness:.3}),x+Math.sin(i*2.4)*.33,-10+h/2,z+Math.cos(i*2.4)*.33,cave);
      crystal.rotation.z=(i-2)*.13;
    }
    const light=new THREE.PointLight(color,3,7,2);light.position.set(x,-9,z);cave.add(light);
  }
  crystal(108.1,-125,'#e8a0be');crystal(131.9,-125,'#8fc8e2');
  function lantern(x,y,z) {
    cylinder(.05,.06,.6,'#77543d',x,y+.3,z,cave);
    const lamp=box(.25,.34,.25,'#dcb173',x,y+.75,z,cave);lamp.material=mat('#f4cd8b',{emissive:'#ffbe70',emissiveIntensity:1.1});
    cylinder(.2,.2,.05,'#755842',x,y+.95,z,cave,6);cylinder(.16,.16,.05,'#755842',x,y+.55,z,cave,6);
    const light=new THREE.PointLight('#ffd8a0',9,11,2);light.position.set(x,y+.8,z);cave.add(light);
  }
  for(const [x,z] of [[117.7,-100],[122.3,-107],[117.7,-113],[112.5,-126.5],[127.5,-126.5]])lantern(x,caveHeight(x,z),z);
  for(const [x,color,text] of [[111,'#eab0c5','A HOME OF YOUR OWN'],[129,'#a2d8e6','FRIENDS ACROSS THE WATER']]) {
    cylinder(.7,.85,.2,'#b59174',x,-9.9,-124,cave,10);
    const plaque=flatLabel(text,color,'#624c40',768,100);plaque.position.set(x,-7.75,-128.7);plaque.scale.set(5.5,.7,1);cave.add(plaque);
  }
  // The mouth is a low boulder mound with an open arch, grasses and desert flowers.
  for(const sign of [-1,1])for(let i=0;i<6;i++) {
    const r=rock(120+sign*(3.5+(i%2)*.8),.65+Math.floor(i/2)*.65,-98-i*.82,1.35,i);r.scale.set(1,.78,1.25);
  }
  for(let i=0;i<5;i++){const r=rock(117.2+i*1.4,3.5+Math.sin(i*Math.PI/4)*.8,-99,.95,i);r.scale.set(1.15,.8,1.3);}
  for(const [x,z] of [[115,-95],[125,-96],[113.9,-99],[126.8,-101],[115,-104],[125,-105]]) {
    for(let i=0;i<5;i++){const leaf=mesh(new THREE.ConeGeometry(.16,.85+(i%2)*.3,4),'#7f9669',x+(i-2)*.13,.35,z,cave);leaf.rotation.z=(i-2)*-.15;}
    for(let i=0;i<3;i++)sphere(.085,i%2?'#ddb56c':'#d08f7e',x-.3+i*.24,.65+(i%2)*.1,z+.1,cave);
  }
  return cave;
}
