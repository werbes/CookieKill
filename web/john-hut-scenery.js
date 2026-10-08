// Dimensions and furniture positions match internal/game/john_hut.go.
export const JOHN_HUT_X=222,JOHN_HUT_Z=-239;
export function buildJohnHut({THREE,parent,flatLabel}){
  const group=new THREE.Group();group.name='john-pork-hut';group.position.set(JOHN_HUT_X,0,JOHN_HUT_Z);parent.add(group);
  const geometry=new THREE.BoxGeometry(1,1,1),materials=new Map();
  function box(name,color,x,y,z,w,h,d){
    if(!materials.has(color))materials.set(color,new THREE.MeshStandardMaterial({color,roughness:.87}));
    const object=new THREE.Mesh(geometry,materials.get(color));object.name=name;object.position.set(x,y,z);object.scale.set(w,h,d);object.castShadow=true;object.receiveShadow=true;group.add(object);return object;
  }
  box('hut-floor','#a99671',0,-.055,0,8,.1,8);
  for(const [id,x,z,w,d]of [['west',-4,0,.3,8.3],['east',4,0,.3,8.3],['north',0,-4,8.3,.3],['south_left',-2.6,4,2.8,.3],['south_right',2.6,4,2.8,.3]])box('john-hut-'+id,'#b8a888',x,1.5,z,w,3,d);
  box('john-hut-door-lintel','#b8a888',0,2.75,4,2.4,.5,.3);
  box('john-hut-ceiling','#8b7858',0,3.1,0,8.3,.2,8.3);
  for(const x of [-4,4])for(const z of [-4,4])box('hut-corner-post','#725b43',x,1.54,z,.26,3.08,.35);
  for(const x of [-1.23,1.23])box('hut-door-frame','#725b43',x,1.25,4.17,.13,2.5,.12);
  box('hut-door-frame','#725b43',0,2.5,4.17,2.6,.14,.12);
  const pitch=Math.atan2(1.18,4.5),roofWidth=Math.hypot(4.5,1.18);
  for(const side of [-1,1]){
    const roof=box('hut-roof','#766951',side*2.25,3.71,0,roofWidth,.22,9);roof.rotation.z=-side*pitch;
    for(let i=0;i<10;i++){const batten=box('hut-roof-batten','#92815d',side*2.25,3.85,-4.32+i*.96,roofWidth,.06,.07);batten.rotation.z=-side*pitch;}
  }
  box('hut-roof-ridge','#a28b64',0,4.39,0,.2,.15,9.1);
  // A shaded front window and a level opening welcome curious daytime visitors.
  box('hut-window-frame','#675943',-2.55,1.7,4.18,1.16,1.12,.11);
  box('hut-window','#769596',-2.55,1.7,4.245,.96,.92,.015);
  box('hut-window-cross','#d0bd92',-2.55,1.7,4.27,.055,.92,.035);
  box('hut-window-cross','#d0bd92',-2.55,1.7,4.27,.96,.055,.035);
  box('hut-welcome-mat','#725c52',0,.017,4.85,2.1,.025,1.05);
  box('john-hut-bed','#665043',-1.8,.25,-.9,1.55,.5,2.7);
  box('hut-mattress','#e5ddbf',-1.8,.6,-.9,1.45,.2,2.6);
  box('hut-pillow','#fbf0d5',-1.8,.765,-1.92,1.23,.13,.56);
  box('hut-headboard','#806249',-1.8,.73,-2.25,1.65,1.2,.12);
  box('hut-folded-blanket','#987664',-1.8,.73,.1,1.46,.06,.45);
  box('john-hut-nightstand','#826347',-.25,.34,-1.8,.7,.68,.7);
  box('hut-phone','#242d2c',-.25,.71,-1.8,.16,.05,.28);
  box('hut-phone-screen','#a3c4af',-.25,.739,-1.8,.12,.009,.2);
  box('hut-rug','#718780',1.15,.015,-.2,2.2,.02,3.7);
  for(let i=0;i<6;i++)box('hut-rug-stripe','#c8b99a',1.15,.027,-1.75+i*.62,2.15,.006,.065);
  if(flatLabel){
    const sign=flatLabel('JOHN PORK','#f0dec0','#534a3b',512,100);sign.name='john-hut-sign';sign.position.set(0,2.98,4.21);sign.scale.set(2.45,.4,1);group.add(sign);
    const hours=flatLabel('ASLEEP UNTIL NIGHTFALL','#655744','#dac9a1',512,100);hours.name='john-hut-hours';hours.position.set(2.6,1.7,4.19);hours.scale.set(2,.39,1);group.add(hours);
  }
  return group;
}
